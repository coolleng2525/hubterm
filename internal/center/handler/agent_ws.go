// Package handler provides HTTP and WebSocket handlers for the center service.
package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/coolleng2525/hubterm/internal/center/model"
	"github.com/coolleng2525/hubterm/internal/pkg/log"
	hubtermproto "github.com/coolleng2525/hubterm/internal/proto"
)

// AgentWSHandler 管理 agent WebSocket 连接和命令下发
type AgentWSHandler struct {
	DB *gorm.DB

	mu          sync.RWMutex
	agentConns  map[string]*agentConnection // nodeID -> connection
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
		DB:         db,
		agentConns: make(map[string]*agentConnection),
	}
}

// HandleAgentWS 处理 agent 的 WebSocket 连接
func (h *AgentWSHandler) HandleAgentWS(w http.ResponseWriter, r *http.Request) {
	nodeID := r.URL.Query().Get("node_id")
	tokenStr := r.URL.Query().Get("token")

	if nodeID == "" || tokenStr == "" {
		http.Error(w, "missing node_id or token", http.StatusUnauthorized)
		return
	}

	// 验证 node token
	var node model.Node
	if err := h.DB.Where("node_id = ? AND token = ?", nodeID, tokenStr).First(&node).Error; err != nil {
		agentWSLog.Warn("agent ws auth failed", log.String("node_id", nodeID))
		http.Error(w, "invalid node token", http.StatusUnauthorized)
		return
	}

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
			h.handleExecResult(msg.Data)
		case "pong":
			// ping-pong 保持连接
		case "report":
			// agent 通过 WS 上报 — 可选的，HTTP 上报仍是主要方式
		default:
			agentWSLog.Debug("unknown agent message", log.String("type", msg.Type))
		}
	}
}

// SendExecCommand 向指定节点下发命令
// 返回 cmdID 和错误。调用方可通过 GetExecResult 查询结果。
func (h *AgentWSHandler) SendExecCommand(nodeID, command string, timeout int) (string, error) {
	h.mu.RLock()
	ac, ok := h.agentConns[nodeID]
	h.mu.RUnlock()

	if !ok {
		return "", fmt.Errorf("node %s not connected", nodeID)
	}

	cmdID := uuid.New().String()
	msg := hubtermproto.WSMessage{
		Type: "exec",
		Data: hubtermproto.ExecCommand{
			ID:   cmdID,
			Type: "exec",
			Payload: struct {
				Command string `json:"command,omitempty"`
				Timeout int    `json:"timeout,omitempty"`
			}{
				Command: command,
				Timeout: timeout,
			},
		},
	}

	ac.mu.Lock()
	defer ac.mu.Unlock()
	if err := ac.conn.WriteJSON(msg); err != nil {
		return "", fmt.Errorf("write to node %s: %w", nodeID, err)
	}

	agentWSLog.Info("exec command sent",
		log.String("node_id", nodeID),
		log.String("cmd_id", cmdID),
		log.String("command", command),
	)

	return cmdID, nil
}

// handleExecResult 处理 agent 返回的命令执行结果
func (h *AgentWSHandler) handleExecResult(data interface{}) {
	// 将 data 转为 JSON 并传递给 AgentExecResultHandler
	dataJSON, err := json.Marshal(data)
	if err != nil {
		agentWSLog.Error("failed to marshal exec result", log.Err(err))
		return
	}
	h.AgentExecResultHandler(dataJSON)
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
	CmdID    string
	NodeID   string
	Status   string // pending / running / completed / failed
	Result   *hubtermproto.ExecResult
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
func (h *AgentWSHandler) AgentExecResultHandler(data json.RawMessage) {
	var result hubtermproto.ExecResult
	if err := json.Unmarshal(data, &result); err != nil {
		agentWSLog.Error("failed to parse exec result", log.Err(err))
		return
	}

	entry := &execResultEntry{
		CmdID:     result.CmdID,
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
