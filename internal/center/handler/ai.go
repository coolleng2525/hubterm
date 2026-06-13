// Package handler provides HTTP and WebSocket handlers for the HubTerm center service.
//
// ai.go — AI-friendly API handlers for device discovery, command execution, and script management.
// These endpoints are designed for AI agents (e.g., XiaoZhu) to discover devices,
// understand capabilities, issue commands, and retrieve results.
package handler

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/coolleng2525/hubterm/internal/center/model"
	"github.com/coolleng2525/hubterm/internal/center/service"
	"github.com/coolleng2525/hubterm/internal/pkg/log"
	"github.com/coolleng2525/hubterm/internal/pkg/script"
)

var aiLog = log.New("ai_handler")

// AIHandler provides AI-friendly REST API endpoints for device management and command execution.
//
// Endpoints:
//   GET    /api/v1/devices              — Discover all online devices
//   GET    /api/v1/devices/:id          — Get device details
//   GET    /api/v1/devices/:id/capabilities — Get device capabilities
//   POST   /api/v1/devices/:id/exec     — Execute a command on a device
//   GET    /api/v1/devices/:id/exec/:cmd_id — Query command execution result
//   POST   /api/v1/scripts              — Upload and execute a script on one or more targets
type AIHandler struct {
	DB          *gorm.DB
	DeviceSvc   *service.DeviceService
	AgentWS     *AgentWSHandler
	ScriptH     *ScriptHandler
	ScriptEngine *script.Engine
}

// NewAIHandler creates a new AIHandler with the given dependencies.
func NewAIHandler(db *gorm.DB, deviceSvc *service.DeviceService, agentWS *AgentWSHandler) *AIHandler {
	return &AIHandler{
		DB:           db,
		DeviceSvc:    deviceSvc,
		AgentWS:      agentWS,
		ScriptEngine: script.NewEngine(),
	}
}

// Discover handles GET /api/v1/devices
// Returns all online devices with their capabilities in an AI-friendly format.
//
// Response: {"devices": [{DeviceInfo}, ...]}
func (h *AIHandler) Discover(c *gin.Context) {
	devices := h.DeviceSvc.Discover()
	if devices == nil {
		devices = []service.DeviceInfo{}
	}
	c.JSON(http.StatusOK, gin.H{"devices": devices})
}

// GetDevice handles GET /api/v1/devices/:id
// Returns detailed information about a specific device.
func (h *AIHandler) GetDevice(c *gin.Context) {
	deviceID := c.Param("id")
	device, err := h.DeviceSvc.GetDevice(deviceID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, device)
}

// GetCapabilities handles GET /api/v1/devices/:id/capabilities
// Returns the capabilities of a specific device.
//
// Response: {"device_id": "...", "name": "...", "capabilities": ["console", "ping", ...], "protocols": ["serial", "ssh"]}
func (h *AIHandler) GetCapabilities(c *gin.Context) {
	deviceID := c.Param("id")
	device, err := h.DeviceSvc.GetCapabilities(deviceID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"device_id":    device.ID,
		"name":         device.Name,
		"type":         device.Type,
		"capabilities": device.Capabilities,
		"protocols":    device.Protocols,
	})
}

// Execute handles POST /api/v1/devices/:id/exec
// Executes a command on a device by routing through its managing node.
//
// Request:  {"command": "show log | tail -20", "timeout": 30}
// Response: {"cmd_id": "uuid", "status": "pending"}
func (h *AIHandler) Execute(c *gin.Context) {
	deviceID := c.Param("id")

	var req struct {
		Command string `json:"command" binding:"required"`
		Timeout int    `json:"timeout"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request: " + err.Error()})
		return
	}

	if req.Timeout <= 0 {
		req.Timeout = 30
	}

	// Use DeviceService.Execute with the AgentWS.SendExecCommand as the executor.
	cmdID, err := h.DeviceSvc.Execute(deviceID, req.Command, req.Timeout, func(nodeID, command string, timeout int) (string, error) {
		return h.AgentWS.SendExecCommand(nodeID, command, timeout)
	})
	if err != nil {
		aiLog.Warn("device exec failed",
			log.String("device_id", deviceID),
			log.Err(err),
		)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Record audit log.
	username, _ := c.Get("username")
	usernameStr, _ := username.(string)
	if err := h.DB.Create(&model.AuditLog{
		User:   usernameStr,
		Action: "ai_exec",
		Target: deviceID,
		Detail: fmt.Sprintf("command: %s, cmd_id: %s", req.Command, cmdID),
	}).Error; err != nil {
		aiLog.Warn("failed to create audit log", log.Err(err))
	}

	aiLog.Info("device exec initiated",
		log.String("device_id", deviceID),
		log.String("cmd_id", cmdID),
		log.String("command", req.Command),
	)

	c.JSON(http.StatusOK, gin.H{
		"cmd_id": cmdID,
		"status": "pending",
	})
}

// GetResult handles GET /api/v1/devices/:id/exec/:cmd_id
// Queries the execution result of a previously issued command.
//
// Response: {"status": "completed", "result": {"stdout": "...", "stderr": "", "exit_code": 0, "duration_ms": 1234}}
func (h *AIHandler) GetResult(c *gin.Context) {
	cmdID := c.Param("cmd_id")

	entry := GetExecResult(cmdID)
	if entry == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "command not found"})
		return
	}

	resp := gin.H{
		"status": entry.Status,
	}
	if entry.Result != nil {
		resp["result"] = entry.Result
	}

	c.JSON(http.StatusOK, resp)
}

// UploadAndExecute handles POST /api/v1/scripts
// Uploads a script and executes it on one or more target devices.
//
// Request: {
//   "name": "ping-check",
//   "description": "Ping a target IP",
//   "language": "python",
//   "source": "import subprocess\nsubprocess.run(['ping', '-c', '4', '${TARGET}'])",
//   "params": [{"name": "TARGET", "type": "string", "required": true, "description": "IP to ping"}],
//   "targets": ["ap-03", "server-db"],
//   "timeout": 30
// }
//
// Response: {"script_id": "...", "results": [{"target": "ap-03", "cmd_id": "...", "status": "pending"}, ...]}
func (h *AIHandler) UploadAndExecute(c *gin.Context) {
	var req struct {
		Name        string        `json:"name" binding:"required"`
		Description string        `json:"description"`
		Language    string        `json:"language"`
		Source      string        `json:"source" binding:"required"`
		Params      []script.Param `json:"params"`
		Targets     []string      `json:"targets" binding:"required,min=1"`
		Timeout     int           `json:"timeout"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request: " + err.Error()})
		return
	}

	if req.Language == "" {
		req.Language = "python"
	}
	if req.Timeout <= 0 {
		req.Timeout = 30
	}

	// Validate Python syntax.
	if req.Language == "python" {
		if err := h.ScriptEngine.Validate(req.Source); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "syntax error: " + err.Error()})
			return
		}
	}

	// Serialize params to JSON.
	paramsJSON := "[]"
	if len(req.Params) > 0 {
		b, err := json.Marshal(req.Params)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to serialize params"})
			return
		}
		paramsJSON = string(b)
	}

	username, _ := c.Get("username")
	usernameStr, _ := username.(string)

	// Save script to database.
	scriptModel := model.Script{
		ScriptID:    uuid.New().String(),
		Name:        req.Name,
		Description: req.Description,
		Language:    req.Language,
		Source:      req.Source,
		Params:      paramsJSON,
		Timeout:     req.Timeout,
		CreatedBy:   usernameStr,
	}
	if err := h.DB.Create(&scriptModel).Error; err != nil {
		aiLog.Error("failed to create script", log.Err(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	// Execute on each target.
	type targetResult struct {
		Target string `json:"target"`
		CmdID  string `json:"cmd_id,omitempty"`
		Status string `json:"status"`
		Error  string `json:"error,omitempty"`
	}

	results := make([]targetResult, 0, len(req.Targets))
	for _, target := range req.Targets {
		tr := targetResult{Target: target}

		// Check if target is a device or a node.
		var device model.Device
		if err := h.DB.Where("device_id = ?", target).First(&device).Error; err == nil {
			// Target is a device — execute via device service.
			cmdID, err := h.DeviceSvc.Execute(target, req.Source, req.Timeout, func(nodeID, command string, timeout int) (string, error) {
				return h.AgentWS.SendExecCommand(nodeID, command, timeout)
			})
			if err != nil {
				tr.Status = "failed"
				tr.Error = err.Error()
			} else {
				tr.CmdID = cmdID
				tr.Status = "pending"
			}
		} else {
			// Target might be a node ID — execute directly on the node.
			if h.AgentWS.IsNodeConnected(target) {
				cmdID, err := h.AgentWS.SendExecCommand(target, req.Source, req.Timeout)
				if err != nil {
					tr.Status = "failed"
					tr.Error = err.Error()
				} else {
					tr.CmdID = cmdID
					tr.Status = "pending"
				}
			} else {
				tr.Status = "failed"
				tr.Error = fmt.Sprintf("target %s not found as device or node", target)
			}
		}

		results = append(results, tr)
	}

	aiLog.Info("script uploaded and executed",
		log.String("script_id", scriptModel.ScriptID),
		log.String("name", req.Name),
		log.Int("targets", len(req.Targets)),
	)

	c.JSON(http.StatusCreated, gin.H{
		"script_id": scriptModel.ScriptID,
		"results":   results,
	})
}
