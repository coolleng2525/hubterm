package handler

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/coolleng2525/hubterm/internal/center/model"
	"github.com/coolleng2525/hubterm/internal/pkg/log"
)

// BatchHandler 批量命令 API 处理器
type BatchHandler struct {
	DB      *gorm.DB
	AgentWS *AgentWSHandler
}

var batchLog = log.New("batch_handler")

// NewBatchHandler 创建批量命令处理器
func NewBatchHandler(db *gorm.DB, agentWS *AgentWSHandler) *BatchHandler {
	return &BatchHandler{
		DB:      db,
		AgentWS: agentWS,
	}
}

// Exec 批量执行命令
// POST /api/batch/exec
// Request: {"node_ids": ["id1","id2"], "command": "ls", "timeout": 30}
func (h *BatchHandler) Exec(c *gin.Context) {
	var req struct {
		NodeIDs []string `json:"node_ids" binding:"required,min=1"`
		Command string   `json:"command" binding:"required"`
		Timeout int      `json:"timeout"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.Timeout <= 0 {
		req.Timeout = 30
	}

	batchID := uuid.New().String()
	now := time.Now()

	type nodeResult struct {
		NodeID string `json:"node_id"`
		CmdID  string `json:"cmd_id,omitempty"`
		Status string `json:"status"`
		Error  string `json:"error,omitempty"`
	}

	results := make([]nodeResult, 0, len(req.NodeIDs))

	for _, nodeID := range req.NodeIDs {
		nr := nodeResult{NodeID: nodeID}

		// Save batch result record
		br := model.BatchResult{
			BatchID:   batchID,
			NodeID:    nodeID,
			Command:   req.Command,
			Status:    "pending",
			StartedAt: now,
		}
		if err := h.DB.Create(&br).Error; err != nil {
			batchLog.Error("failed to create batch result", log.Err(err))
			nr.Status = "failed"
			nr.Error = "internal error"
			results = append(results, nr)
			continue
		}

		if !h.AgentWS.IsNodeConnected(nodeID) {
			nr.Status = "failed"
			nr.Error = "node not connected"
			h.DB.Model(&br).Update("status", "failed")
			results = append(results, nr)
			continue
		}

		cmdID, err := h.AgentWS.SendExecCommand(nodeID, req.Command, req.Timeout)
		if err != nil {
			nr.Status = "failed"
			nr.Error = err.Error()
			h.DB.Model(&br).Update("status", "failed")
		} else {
			nr.CmdID = cmdID
			nr.Status = "pending"
			h.DB.Model(&br).Update("status", "running")
		}

		results = append(results, nr)
	}

	// Record audit log
	username, _ := c.Get("username")
	usernameStr, _ := username.(string)
	if err := h.DB.Create(&model.AuditLog{
		User:   usernameStr,
		Action: "batch_exec",
		Target: fmt.Sprintf("batch:%s", batchID),
		Detail: fmt.Sprintf("command: %s, nodes: %d", req.Command, len(req.NodeIDs)),
	}).Error; err != nil {
		batchLog.Warn("failed to create audit log", log.Err(err))
	}

	batchLog.Info("batch exec initiated",
		log.String("batch_id", batchID),
		log.Int("nodes", len(req.NodeIDs)),
		log.String("command", req.Command),
	)

	c.JSON(http.StatusOK, gin.H{
		"batch_id": batchID,
		"results":  results,
	})
}

// GetResult 查询批量执行结果
// GET /api/batch/exec/:batch_id
func (h *BatchHandler) GetResult(c *gin.Context) {
	batchID := c.Param("batch_id")
	var results []model.BatchResult
	if err := h.DB.Where("batch_id = ?", batchID).Find(&results).Error; err != nil {
		batchLog.Error("failed to query batch results", log.Err(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	if len(results) == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "batch not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"batch_id": batchID,
		"results":  results,
	})
}
