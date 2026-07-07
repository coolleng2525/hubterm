package middleware

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/coolleng2525/hubterm/internal/center/model"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

// FIXED: JWT secret read from JWT_SECRET env var, lazy-loaded
var jwtSecret []byte
var jwtOnce sync.Once

func getJWTSecret() []byte {
	jwtOnce.Do(func() {
		secret := os.Getenv("JWT_SECRET")
		if secret == "" {
			log.Fatal("JWT_SECRET environment variable is required")
		}
		jwtSecret = []byte(secret)
	})
	return jwtSecret
}

type Claims struct {
	UserID    uint   `json:"user_id"`
	Username  string `json:"username"`
	Role      string `json:"role"`
	TokenType string `json:"token_type,omitempty"`
	jwt.RegisteredClaims
}

// FIXED: Token expiry reduced to 1 hour, added RefreshToken
func GenerateToken(userID uint, username, role string) (string, error) {
	return GenerateTokenWithTTL(userID, username, role, 1*time.Hour)
}

func GenerateTokenWithTTL(userID uint, username, role string, ttl time.Duration) (string, error) {
	return generateTokenWithTTL(userID, username, role, "", ttl)
}

func GenerateMCPTokenWithTTL(userID uint, username, role string, ttl time.Duration) (string, error) {
	return generateTokenWithTTL(userID, username, role, "mcp", ttl)
}

func generateTokenWithTTL(userID uint, username, role, tokenType string, ttl time.Duration) (string, error) {
	if ttl <= 0 {
		ttl = 1 * time.Hour
	}
	claims := Claims{
		UserID:    userID,
		Username:  username,
		Role:      role,
		TokenType: tokenType,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(ttl)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(getJWTSecret())
}

func TokenHash(tokenStr string) string {
	sum := sha256.Sum256([]byte(tokenStr))
	return hex.EncodeToString(sum[:])
}

// RefreshToken generates a new token with extended expiry.
func RefreshToken(oldToken string) (string, error) {
	claims, err := ParseToken(oldToken)
	if err != nil {
		return "", err
	}
	return GenerateToken(claims.UserID, claims.Username, claims.Role)
}

func ParseToken(tokenStr string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &Claims{}, func(t *jwt.Token) (interface{}, error) {
		return getJWTSecret(), nil
	}, jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}))
	if err != nil {
		return nil, err
	}
	if claims, ok := token.Claims.(*Claims); ok && token.Valid {
		return claims, nil
	}
	return nil, jwt.ErrSignatureInvalid
}

func ValidateMCPToken(tokenStr string, claims *Claims) error {
	if claims == nil {
		return nil
	}
	db := model.GetDB()
	if db == nil {
		if claims.TokenType == "mcp" {
			return fmt.Errorf("mcp token store unavailable")
		}
		return nil
	}

	var token model.MCPToken
	err := db.Where("token_hash = ?", TokenHash(tokenStr)).First(&token).Error
	if err != nil {
		if claims.TokenType == "mcp" {
			return fmt.Errorf("mcp token not found")
		}
		return nil
	}
	if token.Revoked {
		return fmt.Errorf("mcp token revoked")
	}
	if time.Now().After(token.ExpiresAt) {
		return fmt.Errorf("mcp token expired")
	}
	now := time.Now().UTC()
	_ = db.Model(&token).Update("last_used_at", &now).Error
	return nil
}

func AuthRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		auth := c.GetHeader("Authorization")
		if auth == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing authorization header"})
			return
		}
		tokenStr := strings.TrimPrefix(auth, "Bearer ")
		claims, err := ParseToken(tokenStr)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
			return
		}
		if err := ValidateMCPToken(tokenStr, claims); err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
			return
		}
		c.Set("user_id", claims.UserID)
		c.Set("username", claims.Username)
		c.Set("role", claims.Role)
		c.Next()
	}
}

func AdminRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		role, _ := c.Get("role")
		if role != "admin" {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "admin required"})
			return
		}
		c.Next()
	}
}

// OperatorRequired allows roles that may change node state.
func OperatorRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		role, _ := c.Get("role")
		if role != "admin" && role != "operator" {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "operator required"})
			return
		}
		c.Next()
	}
}
