package handler

import (
	"net/http"
	"strings"

	"github.com/coolleng2525/hubterm/internal/center/model"
	"github.com/coolleng2525/hubterm/internal/pkg/log"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type SessionHandler struct {
	DB      *gorm.DB
	AgentWS SessionCommander
}

type SessionCommander interface {
	SendControlCommand(nodeID, commandType, sessionID string) (string, error)
}

var sessionLog = log.New("session_handler")

type SessionListItem struct {
	model.Session
	NodeIP   string `json:"node_ip"`
	NodeName string `json:"node_name"`
}

func (h *SessionHandler) List(c *gin.Context) {
	nodeID := c.Query("node_id")
	portName := c.Query("port_name")
	query := h.DB.Model(&model.Session{})
	if nodeID != "" {
		query = query.Where("node_id = ?", nodeID)
	}
	if portName != "" {
		query = query.Where("port_name = ?", portName)
	}
	var sessions []model.Session
	// FIXED: check DB error
	if err := query.Order("connected_at desc").Find(&sessions).Error; err != nil {
		sessionLog.Error("failed to list sessions", log.Err(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	nodeIDs := make([]string, 0, len(sessions))
	seenNodes := make(map[string]struct{})
	for _, session := range sessions {
		if session.NodeID == "" {
			continue
		}
		if _, ok := seenNodes[session.NodeID]; ok {
			continue
		}
		seenNodes[session.NodeID] = struct{}{}
		nodeIDs = append(nodeIDs, session.NodeID)
	}

	nodesByID := make(map[string]model.Node)
	if len(nodeIDs) > 0 {
		var nodes []model.Node
		if err := h.DB.Where("node_id IN ?", nodeIDs).Find(&nodes).Error; err != nil {
			sessionLog.Error("failed to load session nodes", log.Err(err))
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
			return
		}
		for _, node := range nodes {
			nodesByID[node.NodeID] = node
		}
	}

	items := make([]SessionListItem, 0, len(sessions))
	for _, session := range sessions {
		item := SessionListItem{Session: session}
		if node, ok := nodesByID[session.NodeID]; ok {
			item.NodeIP = node.IP
			if node.Name != "" {
				item.NodeName = node.Name
			} else {
				item.NodeName = node.Hostname
			}
		}
		items = append(items, item)
	}

	c.JSON(http.StatusOK, items)
}

func (h *SessionHandler) Kick(c *gin.Context) {
	id := c.Param("id")
	username, _ := c.Get("username")

	var session model.Session
	if err := h.DB.Where("session_id = ? OR id = ?", id, id).First(&session).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "session not found"})
		return
	}
	if h.AgentWS == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "agent command channel is unavailable"})
		return
	}
	cmdID, err := h.AgentWS.SendControlCommand(session.NodeID, "kick_session", session.SessionID)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": err.Error()})
		return
	}

	// FIXED: check Delete error
	if err := h.DB.Delete(&session).Error; err != nil {
		sessionLog.Error("failed to delete session", log.Err(err), log.String("session_id", session.SessionID))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	sessionLog.Info("session kicked",
		log.String("username", username.(string)),
		log.String("session_id", session.SessionID),
		log.String("port", session.PortName),
	)

	// FIXED: check Create error
	if err := model.GetDB().Create(&model.AuditLog{
		User:   username.(string),
		Action: "kick_session",
		Target: session.SessionID,
		Detail: "Kicked session on " + session.PortName,
	}).Error; err != nil {
		sessionLog.Error("failed to create audit log", log.Err(err))
	}

	c.JSON(http.StatusAccepted, gin.H{"success": true, "cmd_id": cmdID, "status": "pending"})
}

func (h *SessionHandler) Rename(c *gin.Context) {
	id := c.Param("id")
	username, _ := c.Get("username")

	var req struct {
		DisplayName string `json:"display_name"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	displayName := strings.TrimSpace(req.DisplayName)
	if len(displayName) > 128 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "display_name is too long"})
		return
	}

	var session model.Session
	if err := h.DB.Where("session_id = ? OR id = ?", id, id).First(&session).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "session not found"})
		return
	}

	if err := h.DB.Model(&session).Update("display_name", displayName).Error; err != nil {
		sessionLog.Error("failed to rename session", log.Err(err), log.String("session_id", session.SessionID))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	session.DisplayName = displayName
	if err := saveSessionDisplayNameOverride(h.DB, session, displayName); err != nil {
		sessionLog.Error("failed to save session display name override", log.Err(err), log.String("session_id", session.SessionID))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	user, _ := username.(string)
	sessionLog.Info("session renamed",
		log.String("username", user),
		log.String("session_id", session.SessionID),
		log.String("display_name", displayName),
	)

	if err := model.GetDB().Create(&model.AuditLog{
		User:   user,
		Action: "rename_session",
		Target: session.SessionID,
		Detail: "Renamed session on " + session.PortName + " to " + displayName,
	}).Error; err != nil {
		sessionLog.Error("failed to create audit log", log.Err(err))
	}

	c.JSON(http.StatusOK, session)
}

func (h *SessionHandler) AssignMaster(c *gin.Context) {
	id := c.Param("id")
	username, _ := c.Get("username")

	var session model.Session
	if err := h.DB.Where("session_id = ? OR id = ?", id, id).First(&session).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "session not found"})
		return
	}
	if h.AgentWS == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "agent command channel is unavailable"})
		return
	}
	cmdID, err := h.AgentWS.SendControlCommand(session.NodeID, "assign_master", session.SessionID)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": err.Error()})
		return
	}

	// demote all other sessions on same port to watcher
	// FIXED: check Update error
	if err := h.DB.Model(&model.Session{}).
		Where("node_id = ? AND port_name = ?", session.NodeID, session.PortName).
		Update("type", "watcher").Error; err != nil {
		sessionLog.Error("failed to demote sessions", log.Err(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	// promote this session to master
	// FIXED: check Update error
	if err := h.DB.Model(&session).Update("type", "master").Error; err != nil {
		sessionLog.Error("failed to promote session", log.Err(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	sessionLog.Info("master assigned",
		log.String("username", username.(string)),
		log.String("session_id", session.SessionID),
		log.String("port", session.PortName),
	)

	// FIXED: check Create error
	if err := model.GetDB().Create(&model.AuditLog{
		User:   username.(string),
		Action: "assign_master",
		Target: session.SessionID,
		Detail: "Assigned master on " + session.PortName,
	}).Error; err != nil {
		sessionLog.Error("failed to create audit log", log.Err(err))
	}

	c.JSON(http.StatusAccepted, gin.H{"success": true, "cmd_id": cmdID, "status": "pending"})
}
