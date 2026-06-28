package handler

import (
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/coolleng2525/hubterm/internal/center/model"
	"github.com/coolleng2525/hubterm/internal/pkg/log"
)

// ProxySession 代理会话
type ProxySession struct {
	SessionID string    `json:"session_id"`
	Target    string    `json:"target"`
	NodeID    string    `json:"node_id"`
	Status    string    `json:"status"` // active/closed
	CreatedAt time.Time `json:"created_at"`
}

// ProxyHandler 会话代理 API 处理器
type ProxyHandler struct {
	DB            *gorm.DB
	mu            sync.RWMutex
	proxySessions map[string]*ProxySession
}

var proxyLog = log.New("proxy_handler")

const proxySessionTTL = 24 * time.Hour

// NewProxyHandler 创建代理处理器
func NewProxyHandler(db *gorm.DB) *ProxyHandler {
	return &ProxyHandler{
		DB:            db,
		proxySessions: make(map[string]*ProxySession),
	}
}

// Connect 建立代理连接
// POST /api/proxy/connect
// Request: {"target": "hubterm://ap-03"}
func (h *ProxyHandler) Connect(c *gin.Context) {
	var req struct {
		Target string `json:"target" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Resolve target
	var nodeID string

	// Check if it's a hubterm:// alias
	var alias model.DeviceAlias
	if err := h.DB.Where("alias = ?", req.Target).First(&alias).Error; err == nil {
		nodeID = alias.NodeID
	} else {
		// Check if it's a device ID
		var device model.Device
		if err := h.DB.Where("device_id = ?", req.Target).First(&device).Error; err == nil {
			nodeID = device.NodeID
		} else {
			// Check if it's a node ID directly
			var node model.Node
			if err := h.DB.Where("node_id = ?", req.Target).First(&node).Error; err == nil {
				nodeID = node.NodeID
			} else {
				c.JSON(http.StatusNotFound, gin.H{"error": fmt.Sprintf("target %s not found", req.Target)})
				return
			}
		}
	}

	if nodeID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "target has no managing node"})
		return
	}

	// Create proxy session
	sessionID := uuid.New().String()
	ps := &ProxySession{
		SessionID: sessionID,
		Target:    req.Target,
		NodeID:    nodeID,
		Status:    "active",
		CreatedAt: time.Now(),
	}

	h.mu.Lock()
	h.cleanupExpiredLocked(time.Now())
	h.proxySessions[sessionID] = ps
	h.mu.Unlock()

	// Record audit log
	username, _ := c.Get("username")
	usernameStr, _ := username.(string)
	if err := h.DB.Create(&model.AuditLog{
		User:   usernameStr,
		Action: "proxy_connect",
		Target: req.Target,
		Detail: fmt.Sprintf("session_id: %s, node_id: %s", sessionID, nodeID),
	}).Error; err != nil {
		proxyLog.Warn("failed to create audit log", log.Err(err))
	}

	proxyLog.Info("proxy session created",
		log.String("session_id", sessionID),
		log.String("target", req.Target),
		log.String("node_id", nodeID),
	)

	c.JSON(http.StatusOK, ps)
}

// Disconnect 断开代理连接
// POST /api/proxy/disconnect/:session_id
func (h *ProxyHandler) Disconnect(c *gin.Context) {
	sessionID := c.Param("session_id")

	h.mu.Lock()
	h.cleanupExpiredLocked(time.Now())
	ps, ok := h.proxySessions[sessionID]
	if ok {
		ps.Status = "closed"
		delete(h.proxySessions, sessionID)
	}
	h.mu.Unlock()

	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "session not found"})
		return
	}

	proxyLog.Info("proxy session closed", log.String("session_id", sessionID))
	c.JSON(http.StatusOK, gin.H{"success": true, "session_id": sessionID})
}

// ListSessions 活跃代理会话列表
// GET /api/proxy/sessions
func (h *ProxyHandler) ListSessions(c *gin.Context) {
	h.mu.Lock()
	h.cleanupExpiredLocked(time.Now())
	sessions := make([]*ProxySession, 0, len(h.proxySessions))
	for _, s := range h.proxySessions {
		sessions = append(sessions, s)
	}
	h.mu.Unlock()

	c.JSON(http.StatusOK, sessions)
}

func (h *ProxyHandler) cleanupExpiredLocked(now time.Time) {
	for id, session := range h.proxySessions {
		if now.Sub(session.CreatedAt) > proxySessionTTL {
			session.Status = "closed"
			delete(h.proxySessions, id)
		}
	}
}
