package handler

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/coolleng2525/hubterm/internal/center/model"
	"github.com/coolleng2525/hubterm/internal/pkg/log"
	"github.com/coolleng2525/hubterm/internal/pkg/script"
)

// ScriptHandler handles script management API endpoints.
type ScriptHandler struct {
	DB     *gorm.DB
	Engine *script.Engine
}

var scriptLog = log.New("script_handler")

// NewScriptHandler creates a new ScriptHandler with the given database and script engine.
func NewScriptHandler(db *gorm.DB, engine *script.Engine) *ScriptHandler {
	return &ScriptHandler{
		DB:     db,
		Engine: engine,
	}
}

// Create handles POST /api/scripts — upload a new script.
// Request body: {"name": "...", "description": "...", "language": "python", "source": "...", "params": [...], "timeout": 30}
func (h *ScriptHandler) Create(c *gin.Context) {
	var req struct {
		Name        string         `json:"name" binding:"required"`
		Description string         `json:"description"`
		Language    string         `json:"language"`
		Source      string         `json:"source" binding:"required"`
		Params      []script.Param `json:"params"`
		Timeout     int            `json:"timeout"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Default language to python.
	if req.Language == "" {
		req.Language = "python"
	}

	// Validate script syntax for Python scripts.
	if req.Language == "python" {
		if err := h.Engine.Validate(req.Source); err != nil {
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

	scriptModel := model.Script{
		ScriptID:    uuid.New().String(),
		Name:        req.Name,
		Description: req.Description,
		Language:    req.Language,
		Source:      req.Source,
		Params:      paramsJSON,
		Timeout:     req.Timeout,
		CreatedBy:   username.(string),
	}

	if err := h.DB.Create(&scriptModel).Error; err != nil {
		scriptLog.Error("failed to create script", log.Err(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	scriptLog.Info("script created",
		log.String("script_id", scriptModel.ScriptID),
		log.String("name", scriptModel.Name),
	)

	c.JSON(http.StatusCreated, scriptModel)
}

// Execute handles POST /api/scripts/:id/execute — execute a script locally on the center.
func (h *ScriptHandler) Execute(c *gin.Context) {
	id := c.Param("id")

	var scriptModel model.Script
	if err := h.DB.Where("script_id = ? OR id = ?", id, id).First(&scriptModel).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "script not found"})
		return
	}

	var req struct {
		Params map[string]string `json:"params"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		req.Params = nil
	}

	// Parse stored params.
	scriptParams := parseScriptParams(scriptModel.Params)

	scriptDef := &script.Script{
		ID:       scriptModel.ScriptID,
		Name:     scriptModel.Name,
		Language: scriptModel.Language,
		Source:   scriptModel.Source,
		Params:   scriptParams,
		Timeout:  scriptModel.Timeout,
	}

	result, err := h.Engine.Execute(scriptDef, req.Params)
	if err != nil {
		scriptLog.Warn("script execution error",
			log.String("script_id", scriptModel.ScriptID),
			log.Err(err),
		)
	}

	// Store result in database.
	status := "completed"
	if result.ExitCode != 0 {
		status = "failed"
	}
	if err != nil && result.ExitCode == -1 {
		status = "failed"
	}

	startedAt := time.UnixMilli(result.StartedAt)
	completedAt := time.UnixMilli(result.CompletedAt)

	resultModel := model.ScriptResult{
		ScriptID:    scriptModel.ScriptID,
		NodeID:      "",
		Stdout:      result.Stdout,
		Stderr:      result.Stderr,
		ExitCode:    result.ExitCode,
		Duration:    result.Duration,
		Status:      status,
		StartedAt:   startedAt,
		CompletedAt: completedAt,
	}
	if err := h.DB.Create(&resultModel).Error; err != nil {
		scriptLog.Error("failed to store script result", log.Err(err))
	}

	scriptLog.Info("script executed",
		log.String("script_id", scriptModel.ScriptID),
		log.Int("exit_code", result.ExitCode),
		log.Int("duration_ms", int(result.Duration)),
	)

	c.JSON(http.StatusOK, gin.H{
		"result": result,
	})
}

// ExecuteOnNode handles POST /api/scripts/:id/execute-on-node/:node_id — execute a script on a remote node.
func (h *ScriptHandler) ExecuteOnNode(c *gin.Context) {
	id := c.Param("id")
	nodeID := c.Param("node_id")

	var scriptModel model.Script
	if err := h.DB.Where("script_id = ? OR id = ?", id, id).First(&scriptModel).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "script not found"})
		return
	}

	var req struct {
		Params map[string]string `json:"params"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		req.Params = nil
	}

	scriptParams := parseScriptParams(scriptModel.Params)

	scriptDef := &script.Script{
		ID:       scriptModel.ScriptID,
		Name:     scriptModel.Name,
		Language: scriptModel.Language,
		Source:   scriptModel.Source,
		Params:   scriptParams,
		Timeout:  scriptModel.Timeout,
	}

	result, err := h.Engine.ExecuteOnNode(scriptDef, req.Params, nodeID)
	if err != nil {
		scriptLog.Error("remote execution not available",
			log.String("script_id", scriptModel.ScriptID),
			log.String("node_id", nodeID),
			log.Err(err),
		)
		c.JSON(http.StatusNotImplemented, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"result": result,
	})
}

// List handles GET /api/scripts — list all scripts.
func (h *ScriptHandler) List(c *gin.Context) {
	var scripts []model.Script
	if err := h.DB.Order("updated_at desc").Find(&scripts).Error; err != nil {
		scriptLog.Error("failed to list scripts", log.Err(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	c.JSON(http.StatusOK, scripts)
}

// Get handles GET /api/scripts/:id — get script details.
func (h *ScriptHandler) Get(c *gin.Context) {
	id := c.Param("id")
	var scriptModel model.Script
	if err := h.DB.Where("script_id = ? OR id = ?", id, id).First(&scriptModel).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "script not found"})
		return
	}
	c.JSON(http.StatusOK, scriptModel)
}

// Delete handles DELETE /api/scripts/:id — delete a script.
func (h *ScriptHandler) Delete(c *gin.Context) {
	id := c.Param("id")
	var scriptModel model.Script
	if err := h.DB.Where("script_id = ? OR id = ?", id, id).First(&scriptModel).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "script not found"})
		return
	}
	if err := h.DB.Delete(&scriptModel).Error; err != nil {
		scriptLog.Error("failed to delete script", log.Err(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	scriptLog.Info("script deleted",
		log.String("script_id", scriptModel.ScriptID),
		log.String("name", scriptModel.Name),
	)

	c.JSON(http.StatusOK, gin.H{"success": true})
}

// Update handles PUT /api/scripts/:id — update an existing script.
func (h *ScriptHandler) Update(c *gin.Context) {
	id := c.Param("id")
	var scriptModel model.Script
	if err := h.DB.Where("script_id = ? OR id = ?", id, id).First(&scriptModel).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "script not found"})
		return
	}

	var req struct {
		Name        string         `json:"name" binding:"required"`
		Description string         `json:"description"`
		Language    string         `json:"language"`
		Source      string         `json:"source" binding:"required"`
		Params      []script.Param `json:"params"`
		Timeout     int            `json:"timeout"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.Language == "" {
		req.Language = "python"
	}
	if req.Language == "python" {
		if err := h.Engine.Validate(req.Source); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "syntax error: " + err.Error()})
			return
		}
	}

	paramsJSON := "[]"
	if len(req.Params) > 0 {
		b, err := json.Marshal(req.Params)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid params"})
			return
		}
		paramsJSON = string(b)
	}

	scriptModel.Name = req.Name
	scriptModel.Description = req.Description
	scriptModel.Language = req.Language
	scriptModel.Source = req.Source
	scriptModel.Params = paramsJSON
	scriptModel.Timeout = req.Timeout

	if err := h.DB.Save(&scriptModel).Error; err != nil {
		scriptLog.Error("failed to update script", log.Err(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	scriptLog.Info("script updated", log.String("script_id", scriptModel.ScriptID))
	c.JSON(http.StatusOK, scriptModel)
}

// Results handles GET /api/scripts/:id/results — list execution history for a script.
func (h *ScriptHandler) Results(c *gin.Context) {
	id := c.Param("id")
	var results []model.ScriptResult
	if err := h.DB.Where("script_id = ?", id).Order("created_at desc").Find(&results).Error; err != nil {
		scriptLog.Error("failed to list script results", log.Err(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	c.JSON(http.StatusOK, results)
}

// parseScriptParams parses the JSON params string from the database into []script.Param.
func parseScriptParams(paramsJSON string) []script.Param {
	if paramsJSON == "" || paramsJSON == "[]" {
		return nil
	}
	var params []script.Param
	if err := json.Unmarshal([]byte(paramsJSON), &params); err != nil {
		scriptLog.Warn("failed to parse script params", log.Err(err))
		return nil
	}
	return params
}
