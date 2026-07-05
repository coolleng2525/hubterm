package handler

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/coolleng2525/hubterm/internal/center/model"
	"github.com/coolleng2525/hubterm/internal/pkg/securestore"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func (h *NodeHandler) StartLocalShell(c *gin.Context) {
	var req struct {
		Shell string `json:"shell"`
		Rows  int    `json:"rows"`
		Cols  int    `json:"cols"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Shell == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "shell is required"})
		return
	}
	sessionID := uuid.New().String()
	if err := h.AgentWS.StartLocalShell(c.Param("id"), req.Shell, sessionID, req.Rows, req.Cols); err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"session_id": sessionID})
}

func (h *NodeHandler) CloseLocalShell(c *gin.Context) {
	if err := h.AgentWS.CloseLocalShell(c.Param("id"), c.Param("session_id")); err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *NodeHandler) StartAgentSSH(c *gin.Context) {
	var req struct {
		ProfileID   uint   `json:"profile_id"`
		DisplayName string `json:"display_name"`
		Host        string `json:"host"`
		Port        int    `json:"port"`
		Username    string `json:"username"`
		Password    string `json:"password"`
		PrivateKey  string `json:"private_key"`
		Passphrase  string `json:"passphrase"`
		Rows        int    `json:"rows"`
		Cols        int    `json:"cols"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.ProfileID != 0 {
		userID := currentUserID(c)
		profile, err := loadSSHProfile(h.DB, req.ProfileID, userID)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "SSH profile not found"})
			return
		}
		req.Host, req.Port, req.Username = profile.Host, profile.Port, profile.Username
		req.Password, err = securestore.Decrypt(profile.EncryptedPassword)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to decrypt password"})
			return
		}
		req.PrivateKey, err = securestore.Decrypt(profile.EncryptedPrivateKey)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to decrypt private key"})
			return
		}
		req.Passphrase, err = securestore.Decrypt(profile.EncryptedPassphrase)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to decrypt passphrase"})
			return
		}
		if req.DisplayName == "" {
			req.DisplayName = profile.Name
		}
	}

	req.Host = strings.TrimSpace(req.Host)
	req.Username = strings.TrimSpace(req.Username)
	req.DisplayName = strings.TrimSpace(req.DisplayName)
	if req.Host == "" || req.Username == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "host and username are required"})
		return
	}
	if req.Port <= 0 {
		req.Port = 22
	}
	if req.Port > 65535 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid SSH port"})
		return
	}
	if req.DisplayName == "" {
		req.DisplayName = fmt.Sprintf("%s@%s:%d", req.Username, req.Host, req.Port)
	}
	if len(req.DisplayName) > 128 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "display_name is too long"})
		return
	}
	if h.AgentWS == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "agent command channel is unavailable"})
		return
	}

	nodeID := c.Param("id")
	sessionID := uuid.New().String()
	if err := h.AgentWS.StartSSHSession(nodeID, AgentSSHStartRequest{
		SessionID:   sessionID,
		DisplayName: req.DisplayName,
		Host:        req.Host,
		Port:        req.Port,
		Username:    req.Username,
		Password:    req.Password,
		PrivateKey:  req.PrivateKey,
		Passphrase:  req.Passphrase,
		Rows:        req.Rows,
		Cols:        req.Cols,
	}); err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": err.Error()})
		return
	}

	session := model.Session{
		SessionID:   sessionID,
		NodeID:      nodeID,
		DisplayName: req.DisplayName,
		PortName:    fmt.Sprintf("%s:%d", req.Host, req.Port),
		User:        req.Username,
		Type:        "master",
		ClientIP:    c.ClientIP(),
		ConnectedAt: time.Now(),
	}
	if err := h.DB.Create(&session).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create session"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"session_id": sessionID})
}
