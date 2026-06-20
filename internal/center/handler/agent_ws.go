// Package handler provides HTTP and WebSocket handlers for the center service.
package handler

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"gorm.io/gorm"

	"github.com/coolleng2525/hubterm/internal/center/model"
	"github.com/coolleng2525/hubterm/internal/pkg/log"
	hubtermproto "github.com/coolleng2525/hubterm/internal/proto"
)

// AgentWSHandler 管理 agent WebSocket 连接和命令下发
type AgentWSHandler struct {
	DB *gorm.DB

	mu            sync.RWMutex
	agentConns    map[string]*agentConnection // nodeID -> connection
	localSessions map[string]string           // sessionID -> nodeID
}

// agentConnection 表示一个 agent 的 WebSocket 连接
type agentConnection struct {
	nodeID string
	conn   *websocket.Conn
	mu     sync.Mutex // 保护 conn 写入
}

var agentWSLog = log.New("agent_ws")

// NewAgentWSHandler 创建 agent WebSocket 处理器
func NewAgentWSHandler(db *gorm.DB) *AgentWSHandler {
	return &AgentWSHandler{
		DB:            db,
		agentConns:    make(map[string]*agentConnection),
		localSessions: make(map[string]string),
	}
}

// HandleAgentWS 处理 agent 的 WebSocket 连接
func (h *AgentWSHandler) HandleAgentWS(w http.ResponseWriter, r *http.Request) {
	nodeID := r.URL.Query().Get("node_id")
	tokenStr := agentToken(r)

	if tokenStr == "" {
		http.Error(w, "missing token", http.StatusUnauthorized)
		return
	}

	// The token is authoritative. This also lets browser-based agents connect
	// without having to discover the center's node ID first.
	var node model.Node
	if err := h.DB.Where("token = ?", tokenStr).First(&node).Error; err != nil {
		agentWSLog.Warn("agent ws auth failed", log.String("node_id", nodeID))
		http.Error(w, "invalid node token", http.StatusUnauthorized)
		return
	}
	nodeID = node.NodeID

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		agentWSLog.Error("agent ws upgrade error", log.Err(err))
		return
	}

	ac := &agentConnection{
		nodeID: nodeID,
		conn:   conn,
	}

	h.mu.Lock()
	// 关闭旧连接
	if old, ok := h.agentConns[nodeID]; ok {
		old.conn.Close()
	}
	h.agentConns[nodeID] = ac
	h.mu.Unlock()

	agentWSLog.Info("agent ws connected", log.String("node_id", nodeID))

	defer func() {
		h.mu.Lock()
		if h.agentConns[nodeID] == ac {
			delete(h.agentConns, nodeID)
		}
		h.mu.Unlock()
		conn.Close()
		agentWSLog.Info("agent ws disconnected", log.String("node_id", nodeID))
	}()

	// 读取 agent 消息
	for {
		_, message, err := conn.ReadMessage()
		if err != nil {
			break
		}

		var msg hubtermproto.WSMessage
		if err := json.Unmarshal(message, &msg); err != nil {
			agentWSLog.Warn("agent ws parse error", log.String("node_id", nodeID), log.Err(err))
			continue
		}

		switch msg.Type {
		case "exec_result":
			h.handleExecResult(nodeID, msg.Data)
		case "pong":
			// ping-pong 保持连接
		case "report":
			h.handleReport(nodeID, msg.Data)
		case "terminal_data":
			var terminalData hubtermproto.TerminalData
			raw, err := json.Marshal(msg.Data)
			if err != nil || json.Unmarshal(raw, &terminalData) != nil || !validTerminalData(terminalData) {
				agentWSLog.Warn("invalid terminal data", log.String("node_id", nodeID))
				continue
			}
			if h.ownsSession(nodeID, terminalData.SessionID) {
				BroadcastTerminalData(nodeID, terminalData)
			}
		default:
			agentWSLog.Debug("unknown agent message", log.String("type", msg.Type))
		}
	}
}

func agentToken(r *http.Request) string {
	auth := r.Header.Get("Authorization")
	if strings.HasPrefix(auth, "Bearer ") {
		return strings.TrimPrefix(auth, "Bearer ")
	}
	for _, protocol := range websocket.Subprotocols(r) {
		if strings.HasPrefix(protocol, "hubterm.node.") {
			return strings.TrimPrefix(protocol, "hubterm.node.")
		}
	}
	return ""
}
func validTerminalData(data hubtermproto.TerminalData) bool {
	if data.SessionID == "" || (data.Direction != "input" && data.Direction != "output") {
		return false
	}
	decoded, err := base64.StdEncoding.DecodeString(data.Data)
	return err == nil && len(decoded) <= 1024*1024
}

func (h *AgentWSHandler) handleReport(nodeID string, data interface{}) {
	raw, err := json.Marshal(data)
	if err != nil {
		return
	}
	var report hubtermproto.NodeReport
	if err := json.Unmarshal(raw, &report); err != nil {
		agentWSLog.Warn("invalid agent report", log.String("node_id", nodeID), log.Err(err))
		return
	}
	if len(report.Sessions) > 1000 {
		agentWSLog.Warn("agent report has too many sessions", log.String("node_id", nodeID))
		return
	}
	now := time.Now()
	tx := h.DB.Begin()
	if err := tx.Model(&model.Node{}).Where("node_id = ?", nodeID).Updates(map[string]interface{}{
		"name": report.Name, "hostname": report.Hostname, "os": report.OS,
		"os_version": report.OSVersion, "arch": report.Arch,
		"status": "online", "last_seen": now, "updated_at": now,
	}).Error; err != nil {
		tx.Rollback()
		return
	}

	sessionIDs := make([]string, 0, len(report.Sessions))
	for _, incoming := range report.Sessions {
		if incoming.SessionID == "" {
			continue
		}
		connectedAt := now
		if incoming.ConnectedAt > 0 {
			connectedAt = time.Unix(incoming.ConnectedAt, 0)
		}
		var session model.Session
		result := tx.Where("session_id = ?", incoming.SessionID).First(&session)
		if result.Error != nil && result.Error != gorm.ErrRecordNotFound {
			tx.Rollback()
			return
		}
		if result.Error == nil && session.NodeID != nodeID {
			agentWSLog.Warn("rejected session owned by another node",
				log.String("node_id", nodeID), log.String("session_id", incoming.SessionID))
			tx.Rollback()
			return
		}
		attrs := map[string]interface{}{
			"node_id": nodeID, "port_name": incoming.PortName, "user": incoming.User,
			"type": incoming.Type, "client_ip": incoming.ClientIP, "connected_at": connectedAt,
		}
		if result.Error == gorm.ErrRecordNotFound {
			session = model.Session{SessionID: incoming.SessionID}
			if err := tx.Model(&session).Assign(attrs).FirstOrCreate(&session).Error; err != nil {
				tx.Rollback()
				return
			}
		} else if err := tx.Model(&session).Updates(attrs).Error; err != nil {
			tx.Rollback()
			return
		}
		sessionIDs = append(sessionIDs, incoming.SessionID)
	}
	stale := tx.Where("node_id = ?", nodeID)
	if len(sessionIDs) > 0 {
		stale = stale.Where("session_id NOT IN ?", sessionIDs)
	}
	if err := stale.Delete(&model.Session{}).Error; err != nil {
		tx.Rollback()
		return
	}
	if err := tx.Commit().Error; err != nil {
		return
	}
	var node model.Node
	if h.DB.Where("node_id = ?", nodeID).First(&node).Error == nil {
		BroadcastNodeUpdate(node)
	}
}

func validTerminalInput(input hubtermproto.TerminalInput) bool {
	if input.NodeID == "" || input.SessionID == "" {
		return false
	}
	decoded, err := base64.StdEncoding.DecodeString(input.Data)
	return err == nil && len(decoded) <= 1024*1024
}

func (h *AgentWSHandler) ownsSession(nodeID, sessionID string) bool {
	h.mu.RLock()
	owner := h.localSessions[sessionID]
	h.mu.RUnlock()
	if owner == nodeID {
		return true
	}
	var count int64
	return h.DB.Model(&model.Session{}).
		Where("node_id = ? AND session_id = ?", nodeID, sessionID).
		Count(&count).Error == nil && count == 1
}

func (h *AgentWSHandler) StartLocalShell(nodeID, shellID, sessionID string, rows, cols int) error {
	cmd := hubtermproto.ExecCommand{ID: uuid.New().String(), Type: "shell_start"}
	cmd.Payload.SessionID, cmd.Payload.Shell, cmd.Payload.Rows, cmd.Payload.Cols = sessionID, shellID, rows, cols
	if err := h.sendCommand(nodeID, cmd); err != nil {
		return err
	}
	h.mu.Lock()
	h.localSessions[sessionID] = nodeID
	h.mu.Unlock()
	return nil
}

func (h *AgentWSHandler) CloseLocalShell(nodeID, sessionID string) error {
	cmd := hubtermproto.ExecCommand{ID: uuid.New().String(), Type: "shell_close"}
	cmd.Payload.SessionID = sessionID
	err := h.sendCommand(nodeID, cmd)
	h.mu.Lock()
	delete(h.localSessions, sessionID)
	h.mu.Unlock()
	return err
}

func (h *AgentWSHandler) SendTerminalInput(nodeID, sessionID, data string) error {
	input := hubtermproto.TerminalInput{NodeID: nodeID, SessionID: sessionID, Data: data}
	if !validTerminalInput(input) {
		return fmt.Errorf("invalid terminal input")
	}
	if !h.ownsSession(nodeID, sessionID) {
		return fmt.Errorf("terminal session not found")
	}
	cmd := hubtermproto.ExecCommand{ID: uuid.New().String(), Type: "write"}
	cmd.Payload.SessionID = sessionID
	cmd.Payload.Data = data
	return h.sendCommand(nodeID, cmd)
}

// SendExecCommand 向指定节点下发命令
// 返回 cmdID 和错误。调用方可通过 GetExecResult 查询结果。
func (h *AgentWSHandler) SendExecCommand(nodeID, command string, timeout int) (string, error) {
	cmdID := uuid.New().String()
	cmd := hubtermproto.ExecCommand{ID: cmdID, Type: "exec"}
	cmd.Payload.Command = command
	cmd.Payload.Timeout = timeout
	if err := h.sendCommand(nodeID, cmd); err != nil {
		return "", err
	}
	StoreExecResult(&execResultEntry{CmdID: cmdID, NodeID: nodeID, Status: "pending", CreatedAt: time.Now()})
	return cmdID, nil
}

func (h *AgentWSHandler) SendControlCommand(nodeID, commandType, sessionID string) (string, error) {
	cmdID := uuid.New().String()
	cmd := hubtermproto.ExecCommand{ID: cmdID, Type: commandType}
	cmd.Payload.SessionID = sessionID
	if err := h.sendCommand(nodeID, cmd); err != nil {
		return "", err
	}
	StoreExecResult(&execResultEntry{CmdID: cmdID, NodeID: nodeID, Status: "pending", CreatedAt: time.Now()})
	return cmdID, nil
}

func (h *AgentWSHandler) sendCommand(nodeID string, cmd hubtermproto.ExecCommand) error {
	h.mu.RLock()
	ac, ok := h.agentConns[nodeID]
	h.mu.RUnlock()

	if !ok {
		return fmt.Errorf("node %s not connected", nodeID)
	}

	msg := hubtermproto.WSMessage{
		Type: cmd.Type,
		Data: cmd,
	}

	ac.mu.Lock()
	defer ac.mu.Unlock()
	if err := ac.conn.WriteJSON(msg); err != nil {
		return fmt.Errorf("write to node %s: %w", nodeID, err)
	}

	agentWSLog.Info("exec command sent",
		log.String("node_id", nodeID),
		log.String("cmd_id", cmd.ID),
		log.String("command_type", cmd.Type),
	)

	return nil
}

// handleExecResult 处理 agent 返回的命令执行结果
func (h *AgentWSHandler) handleExecResult(nodeID string, data interface{}) {
	// 将 data 转为 JSON 并传递给 AgentExecResultHandler
	dataJSON, err := json.Marshal(data)
	if err != nil {
		agentWSLog.Error("failed to marshal exec result", log.Err(err))
		return
	}
	h.AgentExecResultHandler(nodeID, dataJSON)
}

// IsNodeConnected 检查节点是否在线
func (h *AgentWSHandler) IsNodeConnected(nodeID string) bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	_, ok := h.agentConns[nodeID]
	return ok
}

// GetConnectedNodes 获取所有已连接节点 ID
func (h *AgentWSHandler) GetConnectedNodes() []string {
	h.mu.RLock()
	defer h.mu.RUnlock()
	ids := make([]string, 0, len(h.agentConns))
	for id := range h.agentConns {
		ids = append(ids, id)
	}
	return ids
}

// --- Exec API handlers ---

// ExecCommandHandler 处理 POST /api/nodes/:id/exec — 向节点下发命令
func (h *AgentWSHandler) ExecCommandHandler(caughtContext interface{}) {
	// This is a placeholder — actual handler is registered in cmd/center/main.go
}

// --- 存储执行结果的内存表 ---

type execResultEntry struct {
	CmdID     string
	NodeID    string
	Status    string // pending / running / completed / failed
	Result    *hubtermproto.ExecResult
	CreatedAt time.Time
}

var (
	execResults   = make(map[string]*execResultEntry)
	execResultsMu sync.RWMutex
)

// StoreExecResult 存储命令执行结果
func StoreExecResult(entry *execResultEntry) {
	execResultsMu.Lock()
	defer execResultsMu.Unlock()
	execResults[entry.CmdID] = entry
}

// GetExecResult 查询命令执行结果
func GetExecResult(cmdID string) *execResultEntry {
	execResultsMu.RLock()
	defer execResultsMu.RUnlock()
	return execResults[cmdID]
}

// AgentExecResultHandler 处理 agent 返回的执行结果（从 WS 消息中解析）
func (h *AgentWSHandler) AgentExecResultHandler(nodeID string, data json.RawMessage) {
	var result hubtermproto.ExecResult
	if err := json.Unmarshal(data, &result); err != nil {
		agentWSLog.Error("failed to parse exec result", log.Err(err))
		return
	}
	pending := GetExecResult(result.CmdID)
	if pending == nil || pending.NodeID != nodeID {
		agentWSLog.Warn("rejected unmatched exec result", log.String("cmd_id", result.CmdID), log.String("node_id", nodeID))
		return
	}

	entry := &execResultEntry{
		CmdID:     result.CmdID,
		NodeID:    nodeID,
		Status:    "completed",
		Result:    &result,
		CreatedAt: time.Now(),
	}
	StoreExecResult(entry)

	agentWSLog.Info("exec result stored",
		log.String("cmd_id", result.CmdID),
		log.Int("exit_code", result.ExitCode),
	)
}
