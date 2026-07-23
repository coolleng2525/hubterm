// Package connector provides WebSocket connectivity between agent and center.
package connector

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"sync"
	"time"

	hubtermproto "github.com/coolleng2525/hubterm/internal/proto"
	"github.com/gorilla/websocket"
)

// CenterCommand 中心下发的指令
type CenterCommand struct {
	ID      string `json:"id"`
	Type    string `json:"type"` // exec / shell / ping / restart
	Payload struct {
		Command     string                     `json:"command,omitempty"`
		Timeout     int                        `json:"timeout,omitempty"` // 秒
		SessionID   string                     `json:"session_id,omitempty"`
		Data        string                     `json:"data,omitempty"`
		Shell       string                     `json:"shell,omitempty"`
		Rows        int                        `json:"rows,omitempty"`
		Cols        int                        `json:"cols,omitempty"`
		DisplayName string                     `json:"display_name,omitempty"`
		Host        string                     `json:"host,omitempty"`
		Port        int                        `json:"port,omitempty"`
		Username    string                     `json:"username,omitempty"`
		Password    string                     `json:"password,omitempty"`
		PrivateKey  string                     `json:"private_key,omitempty"`
		Passphrase  string                     `json:"passphrase,omitempty"`
		Serial      *hubtermproto.SerialConfig `json:"serial,omitempty"`
	} `json:"payload,omitempty"`
}

// SetNodeToken updates authentication for subsequent reconnects.
func (c *Connector) SetNodeToken(token string) {
	c.mu.Lock()
	if token == "" || token == c.NodeToken {
		c.mu.Unlock()
		return
	}
	c.NodeToken = token
	conn := c.ws
	c.mu.Unlock()
	if conn != nil {
		_ = conn.Close()
	}
}

// Connector 维护与中心的 WebSocket 长连接
type Connector struct {
	CenterURL         string
	NodeID            string
	NodeToken         string
	ws                *websocket.Conn
	done              chan struct{}
	mu                sync.Mutex
	reconnect         bool
	commandHandler    func(cmd *CenterCommand)
	disconnectHandler func()
}

// New 创建新的 WebSocket 连接器
func New(centerURL, nodeID, nodeToken string) *Connector {
	return &Connector{
		CenterURL: centerURL,
		NodeID:    nodeID,
		NodeToken: nodeToken,
		done:      make(chan struct{}),
		reconnect: true,
	}
}

// SetCommandHandler 注册命令处理器，收到中心指令时调用
func (c *Connector) SetCommandHandler(handler func(cmd *CenterCommand)) {
	c.commandHandler = handler
}

// SetDisconnectHandler registers cleanup that must run after an established
// Center connection is lost.
func (c *Connector) SetDisconnectHandler(handler func()) {
	c.mu.Lock()
	c.disconnectHandler = handler
	c.mu.Unlock()
}

// Connect 建立 WebSocket 连接并持续重连
// 返回的 channel 在连接关闭时关闭。
func (c *Connector) Connect() <-chan struct{} {
	go c.connectLoop()
	return c.done
}

// connectLoop 持续尝试连接，断开后自动重连
func (c *Connector) connectLoop() {
	defer close(c.done)

	for c.reconnect {
		if err := c.connectOnce(); err != nil {
			log.Printf("[connector] connection error: %v, retrying in 5s...", err)
		}
		if !c.reconnect {
			break
		}
		time.Sleep(5 * time.Second)
	}
}

// connectOnce 执行一次 WebSocket 连接
func (c *Connector) connectOnce() error {
	u, err := url.Parse(c.CenterURL)
	if err != nil {
		return fmt.Errorf("parse center URL: %w", err)
	}

	// 构建 WebSocket URL
	scheme := "ws"
	if u.Scheme == "https" {
		scheme = "wss"
	}
	c.mu.Lock()
	token := c.NodeToken
	c.mu.Unlock()
	if token == "" {
		return fmt.Errorf("node token is not available yet")
	}
	wsURL := fmt.Sprintf("%s://%s/api/ws/agent?node_id=%s", scheme, u.Host, url.QueryEscape(c.NodeID))
	headers := http.Header{}
	headers.Set("Authorization", "Bearer "+token)

	conn, _, err := websocket.DefaultDialer.Dial(wsURL, headers)
	if err != nil {
		return fmt.Errorf("dial websocket: %w", err)
	}

	c.mu.Lock()
	c.ws = conn
	c.mu.Unlock()

	log.Printf("[connector] connected to center: node=%s", c.NodeID)

	// 读取消息循环
	for {
		_, message, err := conn.ReadMessage()
		if err != nil {
			c.mu.Lock()
			c.ws = nil
			disconnected := c.disconnectHandler
			c.mu.Unlock()
			conn.Close()
			if disconnected != nil {
				disconnected()
			}
			return fmt.Errorf("read message: %w", err)
		}

		// 解析消息类型
		var msg struct {
			Type string          `json:"type"`
			Data json.RawMessage `json:"data"`
		}
		if err := json.Unmarshal(message, &msg); err != nil {
			log.Printf("[connector] failed to parse message: %v", err)
			continue
		}

		// 根据消息类型分发
		switch msg.Type {
		case "ping":
			c.sendPong()
		case "exec", "shell_start", "ssh_start", "serial_start", "write", "shell_close", "serial_close", "resize":
			c.handleExecCommand(msg.Data)
		default:
			log.Printf("[connector] unknown message type: %s", msg.Type)
		}
	}
}

func (c *Connector) SendTerminalData(sessionID, direction, data string) error {
	return c.writeJSON(hubtermproto.WSMessage{Type: "terminal_data", Data: hubtermproto.TerminalData{SessionID: sessionID, Direction: direction, Data: data}})
}

func (c *Connector) SendTerminalState(state hubtermproto.TerminalState) error {
	return c.writeJSON(hubtermproto.WSMessage{Type: "terminal_state", Data: state})
}

// SendReport 发送节点上报数据到中心
func (c *Connector) SendReport(report interface{}) error {
	msg := hubtermproto.WSMessage{
		Type: "report",
		Data: report,
	}
	return c.writeJSON(msg)
}

// SendResult 发送命令执行结果到中心
func (c *Connector) SendResult(cmdID string, result *hubtermproto.ExecResult) error {
	msg := hubtermproto.WSMessage{
		Type: "exec_result",
		Data: result,
	}
	return c.writeJSON(msg)
}

// Listen 监听中心指令，收到后回调 handler
// 注意：此方法会阻塞，应在 goroutine 中调用。
// Deprecated: Use Connect() instead, which handles commands internally.
func (c *Connector) Listen(handler func(cmd *CenterCommand)) {
	// 空实现 — 指令处理已集成到 connectLoop
	<-c.done
}

// Close 关闭连接
func (c *Connector) Close() {
	c.reconnect = false
	c.mu.Lock()
	if c.ws != nil {
		c.ws.Close()
		c.ws = nil
	}
	c.mu.Unlock()
}

// sendPong 响应 ping
func (c *Connector) sendPong() {
	msg := hubtermproto.WSMessage{
		Type: "pong",
		Data: map[string]string{"node_id": c.NodeID},
	}
	_ = c.writeJSON(msg)
}

// handleExecCommand 处理中心下发的 exec 指令
func (c *Connector) handleExecCommand(data json.RawMessage) {
	var cmd CenterCommand
	if err := json.Unmarshal(data, &cmd); err != nil {
		log.Printf("[connector] failed to parse exec command: %v", err)
		return
	}
	log.Printf("[connector] received exec command: id=%s type=%s command=%s",
		cmd.ID, cmd.Type, cmd.Payload.Command)

	if c.commandHandler != nil {
		c.commandHandler(&cmd)
	}
}

// writeJSON 线程安全地写入 WebSocket 消息
func (c *Connector) writeJSON(v interface{}) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.ws == nil {
		return fmt.Errorf("websocket not connected")
	}
	return c.ws.WriteJSON(v)
}
