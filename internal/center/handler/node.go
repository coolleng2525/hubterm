package handler

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"github.com/coolleng2525/hubterm/internal/center/model"
	"github.com/coolleng2525/hubterm/internal/pkg/log"
	hubtermproto "github.com/coolleng2525/hubterm/internal/proto"
)

type NodeHandler struct {
	DB *gorm.DB
}

var nodeLog = log.New("node_handler")

// FIXED: NodeTokenRequired middleware validates node Bearer token
func NodeTokenRequired(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		auth := c.GetHeader("Authorization")
		if auth == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing authorization header"})
			return
		}
		tokenStr := strings.TrimPrefix(auth, "Bearer ")
		if tokenStr == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid authorization header"})
			return
		}
		var node model.Node
		if err := db.Where("token = ?", tokenStr).First(&node).Error; err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid node token"})
			return
		}
		c.Set("node_id", node.NodeID)
		c.Next()
	}
}

// GenerateNodeToken creates a random token for node authentication.
func GenerateNodeToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// List 节点列表
func (h *NodeHandler) List(c *gin.Context) {
	status := c.Query("status")
	query := h.DB.Model(&model.Node{})
	if status != "" {
		query = query.Where("status = ?", status)
	}
	var nodes []model.Node
	// FIXED: check DB error
	if err := query.Order("updated_at desc").Find(&nodes).Error; err != nil {
		nodeLog.Error("failed to list nodes", log.Err(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	c.JSON(http.StatusOK, nodes)
}

// Get 节点详情
func (h *NodeHandler) Get(c *gin.Context) {
	id := c.Param("id")
	var node model.Node
	if err := h.DB.Where("node_id = ? OR id = ?", id, id).First(&node).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "node not found"})
		return
	}

	var ports []model.SerialPort
	// FIXED: check DB error
	if err := h.DB.Where("node_id = ?", node.NodeID).Find(&ports).Error; err != nil {
		nodeLog.Error("failed to list ports", log.Err(err), log.String("node_id", node.NodeID))
	}

	var sessions []model.Session
	// FIXED: check DB error
	if err := h.DB.Where("node_id = ?", node.NodeID).Order("connected_at desc").Find(&sessions).Error; err != nil {
		nodeLog.Error("failed to list sessions", log.Err(err), log.String("node_id", node.NodeID))
	}

	c.JSON(http.StatusOK, gin.H{
		"node":     node,
		"ports":    ports,
		"sessions": sessions,
	})
}

// Report 节点上报
// FIXED: protected by NodeTokenRequired middleware
func (h *NodeHandler) Report(c *gin.Context) {
	var report hubtermproto.NodeReport
	if err := c.ShouldBindJSON(&report); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	now := time.Now()

	// upsert node
	var node model.Node
	result := h.DB.Where("node_id = ?", report.NodeID).First(&node)
	if result.Error != nil {
		if result.Error != gorm.ErrRecordNotFound {
			nodeLog.Error("failed to query node", log.Err(result.Error), log.String("node_id", report.NodeID))
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
			return
		}
		// new node: generate token
		token, err := GenerateNodeToken()
		if err != nil {
			nodeLog.Error("failed to generate node token", log.Err(err))
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
			return
		}
		node = model.Node{
			NodeID:    report.NodeID,
			Token:     token,
			CreatedAt: now,
		}
		nodeLog.Info("new node registered",
			log.String("node_id", report.NodeID),
			log.String("token", token),
		)
	}
	node.Name = report.Name
	node.IP = report.IP
	node.Hostname = report.Hostname
	node.OS = report.OS
	node.OSVersion = report.OSVersion
	node.Arch = report.Arch
	node.CPUPercent = report.CPUPercent
	node.MemoryTotal = report.MemoryTotal
	node.MemoryUsed = report.MemoryUsed
	node.MemoryPercent = report.MemoryPercent
	node.DiskTotal = report.DiskTotal
	node.DiskUsed = report.DiskUsed
	node.Status = "online"
	node.LastSeen = now
	node.UpdatedAt = now

	// FIXED: check Save error
	if err := h.DB.Save(&node).Error; err != nil {
		nodeLog.Error("failed to save node", log.Err(err), log.String("node_id", report.NodeID))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	nodeLog.Info("node reported",
		log.String("node_id", report.NodeID),
		log.String("ip", report.IP),
	)

	// upsert serial ports
	for _, sp := range report.SerialPorts {
		var port model.SerialPort
		res := h.DB.Where("node_id = ? AND port_name = ?", report.NodeID, sp.PortName).First(&port)
		if res.Error != nil {
			if res.Error != gorm.ErrRecordNotFound {
				nodeLog.Error("failed to query port", log.Err(res.Error))
				continue
			}
			port = model.SerialPort{
				NodeID:   report.NodeID,
				PortName: sp.PortName,
			}
		}
		port.Description = sp.Description
		port.Status = sp.Status
		port.BaudRate = sp.BaudRate
		port.UpdatedAt = now
		// FIXED: check Save error
		if err := h.DB.Save(&port).Error; err != nil {
			nodeLog.Error("failed to save port", log.Err(err), log.String("port", sp.PortName))
		}
	}

	// sync sessions: delete old, insert new
	// FIXED: check Delete error
	if err := h.DB.Where("node_id = ?", report.NodeID).Delete(&model.Session{}).Error; err != nil {
		nodeLog.Error("failed to delete old sessions", log.Err(err), log.String("node_id", report.NodeID))
	}
	for _, s := range report.Sessions {
		session := model.Session{
			SessionID:   s.SessionID,
			NodeID:      report.NodeID,
			PortName:    s.PortName,
			User:        s.User,
			Type:        s.Type,
			ClientIP:    s.ClientIP,
			ConnectedAt: time.Unix(s.ConnectedAt, 0),
		}
		// FIXED: check Create error
		if err := h.DB.Create(&session).Error; err != nil {
			nodeLog.Error("failed to create session", log.Err(err), log.String("session_id", s.SessionID))
		}
	}

	// broadcast node update via WS
	BroadcastNodeUpdate(node)

	c.JSON(http.StatusOK, gin.H{"success": true, "token": node.Token})
}

// Command 下发指令到节点
func (h *NodeHandler) Command(c *gin.Context) {
	id := c.Param("id")
	var req hubtermproto.CommandRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	username, _ := c.Get("username")
	// FIXED: check Create error
	if err := model.GetDB().Create(&model.AuditLog{
		User:   username.(string),
		Action: "command",
		Target: id,
		Detail: req.Command,
	}).Error; err != nil {
		nodeLog.Error("failed to create audit log", log.Err(err))
	}

	nodeLog.Info("command issued",
		log.String("username", username.(string)),
		log.String("node_id", id),
		log.String("command", req.Command),
	)

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "command queued"})
}

// RegenerateToken generates a new node token.
func (h *NodeHandler) RegenerateToken(c *gin.Context) {
	id := c.Param("id")
	var node model.Node
	if err := h.DB.Where("node_id = ? OR id = ?", id, id).First(&node).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "node not found"})
		return
	}

	token, err := GenerateNodeToken()
	if err != nil {
		nodeLog.Error("failed to generate node token", log.Err(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	if err := h.DB.Model(&node).Update("token", token).Error; err != nil {
		nodeLog.Error("failed to update node token", log.Err(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	nodeLog.Info("node token regenerated", log.String("node_id", node.NodeID))
	c.JSON(http.StatusOK, gin.H{"token": token})
}

// GetPendingCommands 获取待执行指令（节点轮询）
// FIXED: protected by NodeTokenRequired middleware
func (h *NodeHandler) GetPendingCommands(c *gin.Context) {
	nodeID := c.Query("node_id")
	if nodeID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "node_id required"})
		return
	}
	// simplified: return empty list
	c.JSON(http.StatusOK, gin.H{"commands": []hubtermproto.CommandRequest{}})
}
