package handler

import (
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/coolleng2525/hubterm/internal/center/middleware"
	"github.com/coolleng2525/hubterm/internal/center/model"
	"github.com/coolleng2525/hubterm/internal/pkg/log"
	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

type AuthHandler struct {
	DB *gorm.DB
}

var authLog = log.New("auth_handler")

const (
	loginRateLimitWindow      = time.Minute
	loginRateLimitMaxAttempts = 5
)

type loginAttemptBucket struct {
	count      int
	windowEnds time.Time
}

var (
	loginAttemptsMu sync.Mutex
	loginAttempts   = make(map[string]loginAttemptBucket)
)

type LoginReq struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

type RegisterReq struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
	Role     string `json:"role"`
}

func (h *AuthHandler) Login(c *gin.Context) {
	var req LoginReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	clientIP := c.ClientIP()
	if !allowLoginAttempt(clientIP, time.Now()) {
		authLog.Warn("login rate limited", log.String("ip", clientIP))
		c.JSON(http.StatusTooManyRequests, gin.H{"error": "too many login attempts"})
		return
	}

	var user model.User
	if err := h.DB.Where("username = ?", req.Username).First(&user).Error; err != nil {
		authLog.Warn("login failed: user not found",
			log.String("username", req.Username),
			log.String("ip", clientIP),
		)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid username or password"})
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		authLog.Warn("login failed: wrong password",
			log.String("username", req.Username),
			log.String("ip", clientIP),
		)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid username or password"})
		return
	}

	token, err := middleware.GenerateToken(user.ID, user.Username, user.Role)
	if err != nil {
		authLog.Error("failed to generate token", log.Err(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate token"})
		return
	}

	authLog.Info("login success",
		log.String("username", user.Username),
		log.String("role", user.Role),
		log.String("ip", clientIP),
	)
	resetLoginAttempts(clientIP)

	c.JSON(http.StatusOK, gin.H{
		"token": token,
		"user": gin.H{
			"id":       user.ID,
			"username": user.Username,
			"role":     user.Role,
		},
	})
}

func allowLoginAttempt(ip string, now time.Time) bool {
	loginAttemptsMu.Lock()
	defer loginAttemptsMu.Unlock()

	bucket := loginAttempts[ip]
	if bucket.windowEnds.IsZero() || now.After(bucket.windowEnds) {
		bucket = loginAttemptBucket{windowEnds: now.Add(loginRateLimitWindow)}
	}
	if bucket.count >= loginRateLimitMaxAttempts {
		loginAttempts[ip] = bucket
		return false
	}
	bucket.count++
	loginAttempts[ip] = bucket
	return true
}

func resetLoginAttempts(ip string) {
	loginAttemptsMu.Lock()
	defer loginAttemptsMu.Unlock()
	delete(loginAttempts, ip)
}

func (h *AuthHandler) Register(c *gin.Context) {
	var req RegisterReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.Role == "" {
		req.Role = "operator"
	}
	if req.Role != "admin" && req.Role != "operator" && req.Role != "readonly" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "role must be admin, operator, or readonly"})
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to hash password"})
		return
	}

	user := model.User{
		Username:     req.Username,
		PasswordHash: string(hash),
		Role:         req.Role,
	}

	if err := h.DB.Create(&user).Error; err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": "username already exists"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"id":       user.ID,
		"username": user.Username,
		"role":     user.Role,
	})
}

// RefreshToken generates a new token for an existing valid token.
func (h *AuthHandler) RefreshToken(c *gin.Context) {
	auth := c.GetHeader("Authorization")
	if auth == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing authorization header"})
		return
	}
	tokenStr := strings.TrimPrefix(auth, "Bearer ")
	newToken, err := middleware.RefreshToken(tokenStr)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"token": newToken})
}

// ChangePassword 修改当前用户密码
// PUT /api/auth/password
func (h *AuthHandler) ChangePassword(c *gin.Context) {
	var req struct {
		OldPassword string `json:"old_password" binding:"required"`
		NewPassword string `json:"new_password" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	userID, _ := c.Get("user_id")
	var user model.User
	if err := h.DB.First(&user, userID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.OldPassword)); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "旧密码错误"})
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to hash password"})
		return
	}

	if err := h.DB.Model(&user).Update("password_hash", string(hash)).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update password"})
		return
	}

	authLog.Info("password changed", log.String("username", user.Username))
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "密码修改成功"})
}

func (h *AuthHandler) GenerateMCPToken(c *gin.Context) {
	role, _ := c.Get("role")
	if role != "admin" && role != "operator" {
		c.JSON(http.StatusForbidden, gin.H{"error": "operator required"})
		return
	}

	userIDValue, _ := c.Get("user_id")
	userID, _ := userIDValue.(uint)
	usernameValue, _ := c.Get("username")
	username, _ := usernameValue.(string)
	roleStr, _ := role.(string)

	var req struct {
		Days int `json:"days"`
	}
	_ = c.ShouldBindJSON(&req)
	if req.Days <= 0 {
		req.Days = 365
	}
	if req.Days > 3650 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "days must be <= 3650"})
		return
	}

	ttl := time.Duration(req.Days) * 24 * time.Hour
	expiresAtTime := time.Now().Add(ttl).UTC()
	token, err := middleware.GenerateMCPTokenWithTTL(userID, username, roleStr, ttl)
	if err != nil {
		authLog.Error("failed to generate mcp token", log.Err(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate token"})
		return
	}

	tokenModel := model.MCPToken{
		TokenHash: middleware.TokenHash(token),
		UserID:    userID,
		Username:  username,
		Role:      roleStr,
		ExpiresAt: expiresAtTime,
	}
	if err := h.DB.Create(&tokenModel).Error; err != nil {
		authLog.Error("failed to save mcp token", log.Err(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save token"})
		return
	}
	expiresAt := expiresAtTime.Format(time.RFC3339)

	if err := h.DB.Create(&model.AuditLog{
		User:   username,
		Action: "generate_mcp_token",
		Target: username,
		Detail: "Generated MCP token valid for " + time.Duration(ttl).String(),
	}).Error; err != nil {
		authLog.Warn("failed to create audit log", log.Err(err))
	}

	c.JSON(http.StatusOK, gin.H{
		"token":      token,
		"token_type": "Bearer",
		"expires_at": expiresAt,
		"days":       req.Days,
		"token_id":   tokenModel.ID,
	})
}

func (h *AuthHandler) ListMCPTokens(c *gin.Context) {
	role, _ := c.Get("role")
	if role != "admin" && role != "operator" {
		c.JSON(http.StatusForbidden, gin.H{"error": "operator required"})
		return
	}
	userIDValue, _ := c.Get("user_id")
	userID, _ := userIDValue.(uint)

	var tokens []model.MCPToken
	if err := h.DB.Where("user_id = ?", userID).Order("created_at desc").Find(&tokens).Error; err != nil {
		authLog.Error("failed to list mcp tokens", log.Err(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list tokens"})
		return
	}

	now := time.Now()
	items := make([]gin.H, 0, len(tokens))
	for _, token := range tokens {
		status := "active"
		if token.Revoked {
			status = "revoked"
		} else if now.After(token.ExpiresAt) {
			status = "expired"
		}
		items = append(items, gin.H{
			"id":           token.ID,
			"username":     token.Username,
			"role":         token.Role,
			"status":       status,
			"revoked":      token.Revoked,
			"expires_at":   token.ExpiresAt.UTC().Format(time.RFC3339),
			"last_used_at": formatOptionalTime(token.LastUsedAt),
			"created_at":   token.CreatedAt.UTC().Format(time.RFC3339),
		})
	}
	c.JSON(http.StatusOK, gin.H{"tokens": items})
}

func (h *AuthHandler) RevokeMCPToken(c *gin.Context) {
	role, _ := c.Get("role")
	if role != "admin" && role != "operator" {
		c.JSON(http.StatusForbidden, gin.H{"error": "operator required"})
		return
	}
	userIDValue, _ := c.Get("user_id")
	userID, _ := userIDValue.(uint)
	usernameValue, _ := c.Get("username")
	username, _ := usernameValue.(string)

	var token model.MCPToken
	if err := h.DB.Where("id = ? AND user_id = ?", c.Param("id"), userID).First(&token).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "token not found"})
		return
	}
	if token.Revoked {
		c.JSON(http.StatusOK, gin.H{"success": true})
		return
	}
	if err := h.DB.Model(&token).Update("revoked", true).Error; err != nil {
		authLog.Error("failed to revoke mcp token", log.Err(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to revoke token"})
		return
	}
	if err := h.DB.Create(&model.AuditLog{User: username, Action: "revoke_mcp_token", Target: c.Param("id"), Detail: "Revoked MCP token"}).Error; err != nil {
		authLog.Warn("failed to create audit log", log.Err(err))
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func formatOptionalTime(t *time.Time) interface{} {
	if t == nil {
		return nil
	}
	return t.UTC().Format(time.RFC3339)
}

func (h *AuthHandler) Profile(c *gin.Context) {
	userID, _ := c.Get("user_id")
	var user model.User
	if err := h.DB.First(&user, userID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"id":       user.ID,
		"username": user.Username,
		"role":     user.Role,
	})
}
