// Package handler provides HTTP and WebSocket handlers for the HubTerm center service.
package handler

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/coolleng2525/hubterm/internal/center/model"
	"github.com/coolleng2525/hubterm/internal/center/service"
	"github.com/coolleng2525/hubterm/internal/pkg/log"
	"github.com/coolleng2525/hubterm/internal/pkg/script"
)

const mcpProtocolVersion = "2025-06-18"

var mcpLog = log.New("mcp_handler")

type MCPHandler struct {
	DB           *gorm.DB
	DeviceSvc    *service.DeviceService
	AgentWS      *AgentWSHandler
	ScriptEngine *script.Engine

	terminalExecMu sync.RWMutex
	terminalExecs  map[string]terminalExec
}

type terminalExec struct {
	CmdID     string
	DeviceID  string
	SessionID string
	NodeID    string
	Command   string
	CreatedAt time.Time
}

type mcpRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type mcpResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Result  interface{}     `json:"result,omitempty"`
	Error   *mcpError       `json:"error,omitempty"`
}

type mcpError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func NewMCPHandler(db *gorm.DB, deviceSvc *service.DeviceService, agentWS *AgentWSHandler) *MCPHandler {
	return &MCPHandler{DB: db, DeviceSvc: deviceSvc, AgentWS: agentWS, ScriptEngine: script.NewEngine(), terminalExecs: make(map[string]terminalExec)}
}

func (h *MCPHandler) Handle(c *gin.Context) {
	var req mcpRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, mcpResponse{JSONRPC: "2.0", Error: &mcpError{Code: -32700, Message: "parse error"}})
		return
	}
	if len(req.ID) == 0 && strings.HasPrefix(req.Method, "notifications/") {
		c.Status(http.StatusNoContent)
		return
	}
	if req.JSONRPC != "2.0" {
		c.JSON(http.StatusOK, h.mcpError(req.ID, -32600, "invalid JSON-RPC version"))
		return
	}

	result, err := h.dispatch(c, req.Method, req.Params)
	if err != nil {
		c.JSON(http.StatusOK, h.mcpError(req.ID, -32603, err.Error()))
		return
	}
	c.JSON(http.StatusOK, mcpResponse{JSONRPC: "2.0", ID: req.ID, Result: result})
}

func (h *MCPHandler) dispatch(c *gin.Context, method string, params json.RawMessage) (interface{}, error) {
	switch method {
	case "initialize":
		return gin.H{
			"protocolVersion": mcpProtocolVersion,
			"capabilities":    gin.H{"tools": gin.H{"listChanged": false}},
			"serverInfo":      gin.H{"name": "hubterm", "version": "1.14-mcp"},
		}, nil
	case "tools/list":
		return gin.H{"tools": mcpTools()}, nil
	case "tools/call":
		return h.callTool(c, params)
	case "ping":
		return gin.H{}, nil
	default:
		return nil, fmt.Errorf("method not found: %s", method)
	}
}

func (h *MCPHandler) callTool(c *gin.Context, params json.RawMessage) (interface{}, error) {
	var req struct {
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments"`
	}
	if err := json.Unmarshal(params, &req); err != nil {
		return nil, fmt.Errorf("invalid tools/call params")
	}
	if req.Name == "" {
		return nil, fmt.Errorf("tool name is required")
	}

	var result interface{}
	var err error
	switch req.Name {
	case "hubterm_discover_devices":
		result = gin.H{"devices": h.DeviceSvc.Discover()}
	case "hubterm_get_device":
		result, err = h.toolGetDevice(req.Arguments)
	case "hubterm_get_device_capabilities":
		result, err = h.toolGetDeviceCapabilities(req.Arguments)
	case "hubterm_execute_command":
		result, err = h.toolExecuteCommand(c, req.Arguments)
	case "hubterm_get_command_result":
		result, err = h.toolGetCommandResult(req.Arguments)
	case "hubterm_send_terminal_input":
		result, err = h.toolSendTerminalInput(c, req.Arguments)
	case "hubterm_list_quick_sends":
		result, err = h.toolListQuickSends(req.Arguments)
	case "hubterm_run_quick_send":
		result, err = h.toolRunQuickSend(c, req.Arguments)
	case "hubterm_get_terminal_output":
		result, err = h.toolGetTerminalOutput(req.Arguments)
	case "hubterm_upload_and_run_script":
		result, err = h.toolUploadAndRunScript(c, req.Arguments)
	default:
		return nil, fmt.Errorf("unknown tool: %s", req.Name)
	}
	if err != nil {
		return gin.H{"content": []gin.H{{"type": "text", "text": err.Error()}}, "isError": true}, nil
	}
	data, err := json.Marshal(result)
	if err != nil {
		return nil, fmt.Errorf("marshal tool result: %w", err)
	}
	return gin.H{"content": []gin.H{{"type": "text", "text": string(data)}}}, nil
}

func (h *MCPHandler) toolGetDevice(raw json.RawMessage) (interface{}, error) {
	var args struct {
		DeviceID string `json:"device_id"`
	}
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, fmt.Errorf("invalid arguments")
	}
	args.DeviceID = strings.TrimSpace(args.DeviceID)
	if args.DeviceID == "" {
		return nil, fmt.Errorf("device_id is required")
	}
	return h.DeviceSvc.GetDevice(args.DeviceID)
}

func (h *MCPHandler) toolGetDeviceCapabilities(raw json.RawMessage) (interface{}, error) {
	var args struct {
		DeviceID string `json:"device_id"`
	}
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, fmt.Errorf("invalid arguments")
	}
	args.DeviceID = strings.TrimSpace(args.DeviceID)
	if args.DeviceID == "" {
		return nil, fmt.Errorf("device_id is required")
	}
	device, err := h.DeviceSvc.GetCapabilities(args.DeviceID)
	if err != nil {
		return nil, err
	}
	return gin.H{"device_id": device.ID, "name": device.Name, "type": device.Type, "capabilities": device.Capabilities, "protocols": device.Protocols}, nil
}

func (h *MCPHandler) toolExecuteCommand(c *gin.Context, raw json.RawMessage) (interface{}, error) {
	var args struct {
		DeviceID string `json:"device_id"`
		Command  string `json:"command"`
		Timeout  int    `json:"timeout"`
	}
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, fmt.Errorf("invalid arguments")
	}
	args.DeviceID = strings.TrimSpace(args.DeviceID)
	args.Command = strings.TrimSpace(args.Command)
	if args.DeviceID == "" || args.Command == "" {
		return nil, fmt.Errorf("device_id and command are required")
	}
	if args.Timeout <= 0 {
		args.Timeout = 30
	}
	if h.AgentWS == nil {
		return nil, fmt.Errorf("agent command channel is unavailable")
	}
	cmdID, err := h.DeviceSvc.Execute(args.DeviceID, args.Command, args.Timeout, func(nodeID, command string, timeout int) (string, error) {
		return h.AgentWS.SendExecCommand(nodeID, command, timeout)
	})
	mode := "exec"
	if err != nil {
		if !strings.Contains(err.Error(), "device not found") {
			return nil, err
		}
		cmdID, err = h.sendTerminalCommand(args.DeviceID, "", args.Command)
		if err != nil {
			return nil, err
		}
		mode = "terminal"
	}

	username, _ := c.Get("username")
	usernameStr, _ := username.(string)
	if err := h.DB.Create(&model.AuditLog{User: usernameStr, Action: "mcp_exec", Target: args.DeviceID, Detail: fmt.Sprintf("mode: %s, command: %s, cmd_id: %s", mode, args.Command, cmdID)}).Error; err != nil {
		mcpLog.Warn("failed to create audit log", log.Err(err))
	}
	return gin.H{"cmd_id": cmdID, "status": "pending", "mode": mode}, nil
}

func (h *MCPHandler) toolListQuickSends(raw json.RawMessage) (interface{}, error) {
	var args struct {
		Search        string `json:"search"`
		IncludeSource bool   `json:"include_source"`
		Limit         int    `json:"limit"`
	}
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &args); err != nil {
			return nil, fmt.Errorf("invalid arguments")
		}
	}
	args.Search = strings.TrimSpace(args.Search)
	if args.Limit <= 0 || args.Limit > 200 {
		args.Limit = 50
	}

	query := h.DB.Model(&model.Script{}).Order("name asc").Limit(args.Limit)
	if args.Search != "" {
		like := "%" + strings.ToLower(args.Search) + "%"
		query = query.Where("lower(name) LIKE ? OR lower(description) LIKE ? OR lower(language) LIKE ?", like, like, like)
	}

	var scripts []model.Script
	if err := query.Find(&scripts).Error; err != nil {
		return nil, fmt.Errorf("query quick sends: %w", err)
	}

	items := make([]gin.H, 0, len(scripts))
	for _, item := range scripts {
		entry := gin.H{
			"id":          item.ID,
			"script_id":   item.ScriptID,
			"name":        item.Name,
			"description": item.Description,
			"language":    item.Language,
			"timeout":     item.Timeout,
		}
		if args.IncludeSource {
			entry["source"] = item.Source
		}
		items = append(items, entry)
	}
	return gin.H{"quick_sends": items}, nil
}

func (h *MCPHandler) toolRunQuickSend(c *gin.Context, raw json.RawMessage) (interface{}, error) {
	var args struct {
		DeviceID  string `json:"device_id"`
		SessionID string `json:"session_id"`
		ScriptID  string `json:"script_id"`
		Name      string `json:"name"`
	}
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, fmt.Errorf("invalid arguments")
	}
	args.DeviceID = strings.TrimSpace(args.DeviceID)
	args.SessionID = strings.TrimSpace(args.SessionID)
	args.ScriptID = strings.TrimSpace(args.ScriptID)
	args.Name = strings.TrimSpace(args.Name)
	if args.DeviceID == "" && args.SessionID == "" {
		return nil, fmt.Errorf("device_id or session_id is required")
	}
	if args.ScriptID == "" && args.Name == "" {
		return nil, fmt.Errorf("script_id or name is required")
	}
	if h.AgentWS == nil {
		return nil, fmt.Errorf("agent terminal channel is unavailable")
	}

	var quickSend model.Script
	query := h.DB.Model(&model.Script{})
	if args.ScriptID != "" {
		query = query.Where("script_id = ? OR id = ?", args.ScriptID, args.ScriptID)
	} else {
		query = query.Where("name = ?", args.Name)
	}
	if err := query.First(&quickSend).Error; err != nil {
		return nil, fmt.Errorf("quick send not found")
	}

	sess, err := h.resolveTerminalSession(args.DeviceID, args.SessionID)
	if err != nil {
		return nil, err
	}
	chunks := quickSendTerminalChunks(quickSend.Source, quickSend.Language)
	for _, chunk := range chunks {
		if err := h.AgentWS.SendTerminalInput(sess.NodeID, sess.SessionID, encodeTerminalInput(chunk)); err != nil {
			return nil, err
		}
	}

	username, _ := c.Get("username")
	usernameStr, _ := username.(string)
	if err := h.DB.Create(&model.AuditLog{User: usernameStr, Action: "mcp_quick_send", Target: sess.SessionID, Detail: fmt.Sprintf("quick_send: %s, device_id: %s", quickSend.Name, args.DeviceID)}).Error; err != nil {
		mcpLog.Warn("failed to create audit log", log.Err(err))
	}
	return gin.H{"status": "sent", "quick_send": quickSend.Name, "script_id": quickSend.ScriptID, "device_id": args.DeviceID, "session_id": sess.SessionID, "node_id": sess.NodeID}, nil
}

func quickSendTerminalChunks(source, language string) []string {
	trimmed := strings.TrimSpace(source)
	isPython := language == "python" || strings.HasPrefix(trimmed, "#!/usr/bin/env python") || strings.HasPrefix(trimmed, "#!/usr/bin/python")
	if strings.HasPrefix(trimmed, "#!") || isPython {
		ext := "sh"
		runner := "chmod +x %[1]s\n%[1]s\nrm -f %[1]s\n"
		if isPython {
			ext = "py"
			runner = "python3 %[1]s\nrm -f %[1]s\n"
		}
		tmpFile := "/tmp/hubterm_mcp_" + uuid.New().String() + "." + ext
		delimiter := "HUBTERM_MCP_" + strings.ReplaceAll(uuid.New().String(), "-", "_")
		cleanSource := strings.ReplaceAll(source, "\r", "")
		return []string{fmt.Sprintf("cat << '%s' > %s\n%s\n%s\n", delimiter, tmpFile, cleanSource, delimiter) + fmt.Sprintf(runner, tmpFile)}
	}

	lines := strings.Split(source, "\n")
	chunks := make([]string, 0, len(lines))
	for _, line := range lines {
		chunks = append(chunks, strings.TrimRight(line, "\r")+"\r")
	}
	return chunks
}

func (h *MCPHandler) toolSendTerminalInput(c *gin.Context, raw json.RawMessage) (interface{}, error) {
	var args struct {
		DeviceID      string `json:"device_id"`
		SessionID     string `json:"session_id"`
		Input         string `json:"input"`
		AppendNewline *bool  `json:"append_newline"`
	}
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, fmt.Errorf("invalid arguments")
	}
	args.DeviceID = strings.TrimSpace(args.DeviceID)
	args.SessionID = strings.TrimSpace(args.SessionID)
	if args.DeviceID == "" && args.SessionID == "" {
		return nil, fmt.Errorf("device_id or session_id is required")
	}
	if args.Input == "" {
		return nil, fmt.Errorf("input is required")
	}
	if h.AgentWS == nil {
		return nil, fmt.Errorf("agent terminal channel is unavailable")
	}

	sess, err := h.resolveTerminalSession(args.DeviceID, args.SessionID)
	if err != nil {
		return nil, err
	}
	data := args.Input
	if args.AppendNewline == nil || *args.AppendNewline {
		data += "\r"
	}
	encodedData := encodeTerminalInput(data)
	if err := h.AgentWS.SendTerminalInput(sess.NodeID, sess.SessionID, encodedData); err != nil {
		return nil, err
	}

	username, _ := c.Get("username")
	usernameStr, _ := username.(string)
	if err := h.DB.Create(&model.AuditLog{User: usernameStr, Action: "mcp_terminal_input", Target: sess.SessionID, Detail: fmt.Sprintf("device_id: %s, bytes: %d", args.DeviceID, len(data))}).Error; err != nil {
		mcpLog.Warn("failed to create audit log", log.Err(err))
	}
	return gin.H{"status": "sent", "device_id": args.DeviceID, "session_id": sess.SessionID, "node_id": sess.NodeID}, nil
}

func (h *MCPHandler) sendTerminalCommand(deviceID, sessionID, command string) (string, error) {
	sess, err := h.resolveTerminalSession(deviceID, sessionID)
	if err != nil {
		return "", err
	}
	data := encodeTerminalInput(command + "\r")
	if err := h.AgentWS.SendTerminalInput(sess.NodeID, sess.SessionID, data); err != nil {
		return "", err
	}
	cmdID := uuid.New().String()
	h.terminalExecMu.Lock()
	h.terminalExecs[cmdID] = terminalExec{CmdID: cmdID, DeviceID: deviceID, SessionID: sess.SessionID, NodeID: sess.NodeID, Command: command, CreatedAt: time.Now()}
	h.terminalExecMu.Unlock()
	return cmdID, nil
}

func encodeTerminalInput(data string) string {
	return base64.StdEncoding.EncodeToString([]byte(data))
}

func (h *MCPHandler) resolveTerminalSession(deviceID, sessionID string) (*model.Session, error) {
	var sess model.Session
	if sessionID != "" {
		if err := h.DB.Where("session_id = ?", sessionID).First(&sess).Error; err != nil {
			return nil, fmt.Errorf("terminal session not found: %s", sessionID)
		}
		return &sess, nil
	}

	var sessions []model.Session
	if err := h.DB.Find(&sessions).Error; err != nil {
		return nil, fmt.Errorf("query terminal sessions: %w", err)
	}
	for _, candidate := range sessions {
		if sessionMatchesDeviceID(candidate, deviceID) {
			return &candidate, nil
		}
	}
	return nil, fmt.Errorf("terminal session device not found: %s", deviceID)
}

func sessionMatchesDeviceID(sess model.Session, deviceID string) bool {
	if sess.SessionID == deviceID || sess.DisplayName == deviceID || sess.PortName == deviceID {
		return true
	}
	return sanitizeMCPDeviceID(sess.DisplayName) == deviceID || sanitizeMCPDeviceID(sess.PortName) == deviceID
}

var mcpUnsafeDeviceIDChars = regexp.MustCompile(`[^A-Za-z0-9_.-]+`)

func sanitizeMCPDeviceID(value string) string {
	value = strings.TrimSpace(value)
	value = mcpUnsafeDeviceIDChars.ReplaceAllString(value, "-")
	value = strings.Trim(value, "-_.")
	return value
}

func (h *MCPHandler) toolGetTerminalOutput(raw json.RawMessage) (interface{}, error) {
	var args struct {
		DeviceID     string `json:"device_id"`
		SessionID    string `json:"session_id"`
		Limit        int    `json:"limit"`
		IncludeInput bool   `json:"include_input"`
	}
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, fmt.Errorf("invalid arguments")
	}
	args.DeviceID = strings.TrimSpace(args.DeviceID)
	args.SessionID = strings.TrimSpace(args.SessionID)
	if args.DeviceID == "" && args.SessionID == "" {
		return nil, fmt.Errorf("device_id or session_id is required")
	}
	if h.AgentWS == nil {
		return nil, fmt.Errorf("agent terminal channel is unavailable")
	}
	sess, err := h.resolveTerminalSession(args.DeviceID, args.SessionID)
	if err != nil {
		return nil, err
	}
	items := h.AgentWS.GetTerminalData(sess.SessionID, args.Limit, args.IncludeInput)
	return gin.H{"device_id": args.DeviceID, "session_id": sess.SessionID, "items": items}, nil
}

func (h *MCPHandler) toolGetCommandResult(raw json.RawMessage) (interface{}, error) {
	var args struct {
		CmdID string `json:"cmd_id"`
	}
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, fmt.Errorf("invalid arguments")
	}
	args.CmdID = strings.TrimSpace(args.CmdID)
	if args.CmdID == "" {
		return nil, fmt.Errorf("cmd_id is required")
	}
	entry := GetExecResult(args.CmdID)
	if entry == nil {
		if result, ok := h.getTerminalCommandResult(args.CmdID); ok {
			return result, nil
		}
		return gin.H{"status": "not_found"}, nil
	}
	resp := gin.H{"status": entry.Status}
	if entry.Result != nil {
		resp["result"] = entry.Result
	}
	return resp, nil
}

func (h *MCPHandler) getTerminalCommandResult(cmdID string) (gin.H, bool) {
	h.terminalExecMu.RLock()
	exec, ok := h.terminalExecs[cmdID]
	h.terminalExecMu.RUnlock()
	if !ok || h.AgentWS == nil {
		return nil, false
	}
	items := h.AgentWS.GetTerminalData(exec.SessionID, 50, false)
	var stdout strings.Builder
	for _, item := range items {
		decoded, err := base64.StdEncoding.DecodeString(item.Data)
		if err != nil {
			continue
		}
		stdout.Write(decoded)
	}
	return gin.H{
		"status": "completed",
		"mode":   "terminal",
		"result": gin.H{
			"cmd_id":      exec.CmdID,
			"stdout":      stdout.String(),
			"stderr":      "",
			"exit_code":   0,
			"duration_ms": time.Since(exec.CreatedAt).Milliseconds(),
			"session_id":  exec.SessionID,
			"node_id":     exec.NodeID,
		},
	}, true
}

func (h *MCPHandler) toolUploadAndRunScript(c *gin.Context, raw json.RawMessage) (interface{}, error) {
	var args struct {
		Name        string         `json:"name"`
		Description string         `json:"description"`
		Language    string         `json:"language"`
		Source      string         `json:"source"`
		Params      []script.Param `json:"params"`
		Targets     []string       `json:"targets"`
		Timeout     int            `json:"timeout"`
	}
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, fmt.Errorf("invalid arguments")
	}
	args.Name = strings.TrimSpace(args.Name)
	args.Language = strings.TrimSpace(args.Language)
	args.Source = strings.TrimSpace(args.Source)
	if args.Name == "" || args.Source == "" || len(args.Targets) == 0 {
		return nil, fmt.Errorf("name, source, and targets are required")
	}
	if args.Language == "" {
		args.Language = "python"
	}
	if args.Timeout <= 0 {
		args.Timeout = 30
	}
	if args.Language == "python" {
		if err := h.ScriptEngine.Validate(args.Source); err != nil {
			return nil, fmt.Errorf("syntax error: %w", err)
		}
	}
	if h.AgentWS == nil {
		return nil, fmt.Errorf("agent command channel is unavailable")
	}

	paramsJSON := "[]"
	if len(args.Params) > 0 {
		b, err := json.Marshal(args.Params)
		if err != nil {
			return nil, fmt.Errorf("failed to serialize params")
		}
		paramsJSON = string(b)
	}

	username, _ := c.Get("username")
	usernameStr, _ := username.(string)
	scriptModel := model.Script{ScriptID: uuid.New().String(), Name: args.Name, Description: args.Description, Language: args.Language, Source: args.Source, Params: paramsJSON, Timeout: args.Timeout, CreatedBy: usernameStr}
	if err := h.DB.Create(&scriptModel).Error; err != nil {
		return nil, fmt.Errorf("failed to create script")
	}

	type targetResult struct {
		Target string `json:"target"`
		CmdID  string `json:"cmd_id,omitempty"`
		Status string `json:"status"`
		Error  string `json:"error,omitempty"`
	}
	results := make([]targetResult, 0, len(args.Targets))
	for _, target := range args.Targets {
		target = strings.TrimSpace(target)
		tr := targetResult{Target: target}
		var device model.Device
		if err := h.DB.Where("device_id = ?", target).First(&device).Error; err == nil {
			cmdID, err := h.DeviceSvc.Execute(target, args.Source, args.Timeout, func(nodeID, command string, timeout int) (string, error) {
				return h.AgentWS.SendExecCommand(nodeID, command, timeout)
			})
			if err != nil {
				tr.Status = "failed"
				tr.Error = err.Error()
			} else {
				tr.CmdID = cmdID
				tr.Status = "pending"
			}
		} else if h.AgentWS.IsNodeConnected(target) {
			cmdID, err := h.AgentWS.SendExecCommand(target, args.Source, args.Timeout)
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
		results = append(results, tr)
	}

	if err := h.DB.Create(&model.AuditLog{User: usernameStr, Action: "mcp_script", Target: scriptModel.ScriptID, Detail: fmt.Sprintf("script: %s, targets: %d", args.Name, len(args.Targets))}).Error; err != nil {
		mcpLog.Warn("failed to create audit log", log.Err(err))
	}
	return gin.H{"script_id": scriptModel.ScriptID, "results": results}, nil
}

func (h *MCPHandler) mcpError(id json.RawMessage, code int, message string) mcpResponse {
	return mcpResponse{JSONRPC: "2.0", ID: id, Error: &mcpError{Code: code, Message: message}}
}

func mcpTools() []gin.H {
	return []gin.H{
		{"name": "hubterm_discover_devices", "description": "Discover online devices managed by HubTerm.", "inputSchema": gin.H{"type": "object", "properties": gin.H{}}},
		{"name": "hubterm_get_device", "description": "Get details for one HubTerm device.", "inputSchema": gin.H{"type": "object", "properties": gin.H{"device_id": gin.H{"type": "string", "description": "HubTerm device ID."}}, "required": []string{"device_id"}}},
		{"name": "hubterm_get_device_capabilities", "description": "Get capabilities and protocols for one HubTerm device.", "inputSchema": gin.H{"type": "object", "properties": gin.H{"device_id": gin.H{"type": "string", "description": "HubTerm device ID."}}, "required": []string{"device_id"}}},
		{"name": "hubterm_execute_command", "description": "Execute a command asynchronously on a HubTerm device.", "inputSchema": gin.H{"type": "object", "properties": gin.H{"device_id": gin.H{"type": "string", "description": "HubTerm device ID."}, "command": gin.H{"type": "string", "description": "Command to execute."}, "timeout": gin.H{"type": "integer", "description": "Timeout in seconds. Default 30."}}, "required": []string{"device_id", "command"}}},
		{"name": "hubterm_get_command_result", "description": "Fetch the status and output for a previously executed command.", "inputSchema": gin.H{"type": "object", "properties": gin.H{"cmd_id": gin.H{"type": "string", "description": "Command ID returned by hubterm_execute_command."}}, "required": []string{"cmd_id"}}},
		{"name": "hubterm_send_terminal_input", "description": "Send input to an online HubTerm terminal session discovered from active sessions or SSH terminals.", "inputSchema": gin.H{"type": "object", "properties": gin.H{"device_id": gin.H{"type": "string", "description": "Discovered terminal device ID, such as com9-r770."}, "session_id": gin.H{"type": "string", "description": "Optional raw HubTerm session ID."}, "input": gin.H{"type": "string", "description": "Text to send to the terminal."}, "append_newline": gin.H{"type": "boolean", "description": "Append Enter/CR after input. Default true."}}, "required": []string{"input"}}},
		{"name": "hubterm_list_quick_sends", "description": "List saved Quick Send presets from HubTerm Scripts. Use this before hubterm_run_quick_send when the user refers to a preset by name or asks what quick sends are available.", "inputSchema": gin.H{"type": "object", "properties": gin.H{"search": gin.H{"type": "string", "description": "Optional fuzzy search over preset name, description, or language."}, "include_source": gin.H{"type": "boolean", "description": "Include preset source text. Default false."}, "limit": gin.H{"type": "integer", "description": "Maximum presets to return. Default 50, max 200."}}, "required": []string{}}},
		{"name": "hubterm_run_quick_send", "description": "Run a saved Quick Send preset against an active terminal session. It sends shell/text presets line by line and runs Python or shebang scripts through a temporary file, matching the web Quick Send behavior.", "inputSchema": gin.H{"type": "object", "properties": gin.H{"device_id": gin.H{"type": "string", "description": "Discovered terminal device ID, such as r770-com9 or com9-r770."}, "session_id": gin.H{"type": "string", "description": "Optional raw HubTerm session ID."}, "script_id": gin.H{"type": "string", "description": "Quick Send script_id or numeric id returned by hubterm_list_quick_sends."}, "name": gin.H{"type": "string", "description": "Quick Send preset name. Use only when script_id is unknown."}}, "required": []string{}}},
		{"name": "hubterm_get_terminal_output", "description": "Fetch recent output from an online HubTerm terminal session.", "inputSchema": gin.H{"type": "object", "properties": gin.H{"device_id": gin.H{"type": "string", "description": "Discovered terminal device ID, such as com9-r770."}, "session_id": gin.H{"type": "string", "description": "Optional raw HubTerm session ID."}, "limit": gin.H{"type": "integer", "description": "Maximum recent records to return. Default 50, max 200."}, "include_input": gin.H{"type": "boolean", "description": "Include echoed input records. Default false."}}, "required": []string{}}},
		{"name": "hubterm_upload_and_run_script", "description": "Upload a Python or shell script and execute it on devices or nodes.", "inputSchema": gin.H{"type": "object", "properties": gin.H{"name": gin.H{"type": "string", "description": "Script name."}, "description": gin.H{"type": "string", "description": "Optional script description."}, "language": gin.H{"type": "string", "description": "python or shell. Default python."}, "source": gin.H{"type": "string", "description": "Script source code."}, "params": gin.H{"type": "array", "description": "Optional script parameter definitions."}, "targets": gin.H{"type": "array", "items": gin.H{"type": "string"}, "description": "Device IDs or node IDs."}, "timeout": gin.H{"type": "integer", "description": "Timeout in seconds. Default 30."}}, "required": []string{"name", "source", "targets"}}},
	}
}
