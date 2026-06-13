package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"github.com/coolleng2525/hubterm/internal/center/model"
	"github.com/coolleng2525/hubterm/internal/pkg/log"
)

type AuditLogHandler struct {
	DB *gorm.DB
}

var auditLog = log.New("audit_log_handler")

func (h *AuditLogHandler) List(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "50"))
	action := c.Query("action")

	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 200 {
		pageSize = 50
	}

	query := h.DB.Model(&model.AuditLog{})
	if action != "" {
		query = query.Where("action = ?", action)
	}

	var total int64
	// FIXED: check Count error
	if err := query.Count(&total).Error; err != nil {
		auditLog.Error("failed to count audit logs", log.Err(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	var logs []model.AuditLog
	// FIXED: check Find error
	if err := query.Order("created_at desc").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Find(&logs).Error; err != nil {
		auditLog.Error("failed to list audit logs", log.Err(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"logs":  logs,
		"total": total,
		"page":  page,
	})
}

// LogEntry represents a single log entry from an agent.
type LogEntry struct {
	Level   string `json:"level"`
	Message string `json:"message"`
	Module  string `json:"module"`
}

// LogUploadRequest is the request body for agent log upload.
type LogUploadRequest struct {
	NodeID string     `json:"node_id"`
	Logs   []LogEntry `json:"logs"`
}

// UploadLogs accepts log batches from agents.
func (h *AuditLogHandler) UploadLogs(c *gin.Context) {
	var req LogUploadRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	for _, entry := range req.Logs {
		if err := h.DB.Create(&model.AuditLog{
			User:   req.NodeID,
			Action: "agent_log",
			Detail: "[" + entry.Level + "] " + entry.Message,
		}).Error; err != nil {
			auditLog.Error("failed to store agent log", log.Err(err))
		}
	}

	auditLog.Info("agent logs uploaded",
		log.String("node_id", req.NodeID),
		log.Int("count", len(req.Logs)),
	)

	c.JSON(http.StatusOK, gin.H{"success": true})
}
