// Package handler provides HTTP/WebSocket handlers for the HubTerm center server.
//
// terminal.go — WebSocket terminal handler for SSH and serial connections.
// Supports gateway (jump host) tunneling, session recording, and observer
// (multi-party sharing) mode.
//
// Reference: Next Terminal server/api/term.go, server/api/term_handler.go
package handler

import (
	"bufio"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"path"
	"strconv"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"golang.org/x/crypto/ssh"

	"github.com/coolleng2525/hubterm/internal/pkg/log"
	"github.com/coolleng2525/hubterm/internal/pkg/recorder"
	"github.com/coolleng2525/hubterm/internal/pkg/session"
	"github.com/coolleng2525/hubterm/internal/pkg/sshclient"
	"github.com/coolleng2525/hubterm/internal/pkg/tunnel"
)

// WebSocket message types for terminal protocol.
const (
	MsgClosed    = 0 // connection closed
	MsgConnected = 1 // connection established
	MsgData      = 2 // terminal data
	MsgResize    = 3 // window resize
	MsgPing      = 4 // heartbeat ping
)

// TerminalHandler handles WebSocket-based terminal connections.
type TerminalHandler struct {
	RecordingDir string // base directory for session recordings
}

var terminalLog = log.New("terminal")

// TerminalConnectRequest describes the parameters needed to establish
// a terminal connection.
type TerminalConnectRequest struct {
	SessionID   string `json:"session_id"`              // unique session identifier
	Protocol    string `json:"protocol"`                 // ssh / serial / telnet
	IP          string `json:"ip"`                       // target IP
	Port        int    `json:"port"`                     // target port
	Username    string `json:"username"`                 // SSH username
	Password    string `json:"password"`                 // SSH password
	PrivateKey  string `json:"private_key"`              // SSH private key
	Passphrase  string `json:"passphrase"`               // SSH key passphrase
	Cols        int    `json:"cols"`                     // terminal width
	Rows        int    `json:"rows"`                     // terminal height
	TermType    string `json:"term_type"`                // terminal type (e.g., "xterm-256color")
	GatewayID   string `json:"gateway_id,omitempty"`     // optional jump host gateway ID
	Recording   bool   `json:"recording"`                // enable session recording
	SocksEnable bool   `json:"socks_enable"`             // enable SOCKS5 proxy
	SocksHost   string `json:"socks_host,omitempty"`     // SOCKS5 proxy host
	SocksPort   string `json:"socks_port,omitempty"`     // SOCKS5 proxy port
	SocksUser   string `json:"socks_user,omitempty"`     // SOCKS5 proxy username
	SocksPass   string `json:"socks_pass,omitempty"`     // SOCKS5 proxy password
}

// HandleTerminal handles WebSocket terminal connections.
//
// Flow:
//  1. Upgrade HTTP to WebSocket
//  2. Read TerminalConnectRequest from first WebSocket message
//  3. If gateway configured, open SSH tunnel via GatewayManager
//  4. Establish SSH connection (direct or via SOCKS5)
//  5. Request PTY and start shell
//  6. If recording enabled, create asciicast recorder
//  7. Register session in GlobalSessionManager
//  8. Loop: read WebSocket input → write to SSH; read SSH output → write to WebSocket
//  9. On disconnect, clean up session and close tunnel
//
// POST /api/v1/terminal/connect
func (h *TerminalHandler) HandleTerminal(c *gin.Context) {
	ws, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		terminalLog.Error("ws upgrade error", log.Err(err))
		return
	}
	defer ws.Close()

	// Read connection parameters from first WebSocket message
	_, msgBytes, err := ws.ReadMessage()
	if err != nil {
		terminalLog.Error("read connect request error", log.Err(err))
		return
	}

	var req TerminalConnectRequest
	if err := json.Unmarshal(msgBytes, &req); err != nil {
		writeWSError(ws, "invalid connect request: "+err.Error())
		return
	}

	if req.SessionID == "" {
		writeWSError(ws, "session_id is required")
		return
	}

	if req.TermType == "" {
		req.TermType = "xterm-256color"
	}

	// Resolve target address via gateway if configured
	targetIP := req.IP
	targetPort := req.Port

	if req.GatewayID != "" {
		gw := tunnel.GlobalGatewayManager.Get(req.GatewayID)
		if gw == nil {
			writeWSError(ws, "gateway not found: "+req.GatewayID)
			return
		}
		localIP, localPort, err := gw.OpenTunnel(req.SessionID, req.IP, req.Port)
		if err != nil {
			writeWSError(ws, "open tunnel: "+err.Error())
			return
		}
		targetIP = localIP
		targetPort = localPort
		defer gw.CloseTunnel(req.SessionID)
	}

	// Establish SSH connection
	var sshCl *ssh.Client
	if req.SocksEnable {
		sshCl, err = sshclient.DialViaSocks(
			targetIP, targetPort, req.Username, req.Password, req.PrivateKey, req.Passphrase,
			req.SocksHost, req.SocksPort, req.SocksUser, req.SocksPass,
		)
	} else {
		sshCl, err = sshclient.Dial(targetIP, targetPort, req.Username, req.Password, req.PrivateKey, req.Passphrase)
	}
	if err != nil {
		writeWSError(ws, "ssh dial: "+err.Error())
		return
	}
	defer sshCl.Close()

	// Create SSH session
	sshSession, err := sshCl.NewSession()
	if err != nil {
		writeWSError(ws, "ssh session: "+err.Error())
		return
	}
	defer sshSession.Close()

	// Setup pipes
	stdoutPipe, err := sshSession.StdoutPipe()
	if err != nil {
		writeWSError(ws, "stdout pipe: "+err.Error())
		return
	}
	stdinPipe, err := sshSession.StdinPipe()
	if err != nil {
		writeWSError(ws, "stdin pipe: "+err.Error())
		return
	}
	defer stdinPipe.Close()

	// Request PTY
	modes := ssh.TerminalModes{
		ssh.ECHO:          1,
		ssh.TTY_OP_ISPEED: 14400,
		ssh.TTY_OP_OSPEED: 14400,
	}
	if err := sshSession.RequestPty(req.TermType, req.Rows, req.Cols, modes); err != nil {
		writeWSError(ws, "request pty: "+err.Error())
		return
	}

	// Start shell
	if err := sshSession.Shell(); err != nil {
		writeWSError(ws, "shell: "+err.Error())
		return
	}

	// Setup recording
	var rec *recorder.Recorder
	if req.Recording {
		recPath := path.Join(h.RecordingDir, req.SessionID, "recording.cast")
		rec, err = recorder.NewRecorder(recPath, req.TermType, req.Rows, req.Cols)
		if err != nil {
			terminalLog.Warn("recorder init failed", log.Err(err))
		}
	}
	if rec != nil {
		defer rec.Close()
	}

	// Register session
	obs := session.NewObserver(req.SessionID)
	ss := &session.Session{
		ID:         req.SessionID,
		Protocol:   req.Protocol,
		Mode:       "master",
		WebSocket:  ws,
		SSHClient:  sshCl,
		SSHChannel: nil,
		Observer:   obs,
	}
	session.GlobalSessionManager.Add(ss)
	defer session.GlobalSessionManager.Remove(req.SessionID)

	// Send connected message
	writeWSMsg(ws, session.NewMessage(MsgConnected, ""))

	// Start terminal I/O goroutines
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var wg sync.WaitGroup
	wg.Add(2)

	// Goroutine: read from SSH stdout → buffer → WebSocket
	stdoutReader := bufio.NewReader(stdoutPipe)
	go func() {
		defer wg.Done()
		writeTerminalOutput(ctx, ws, stdoutReader, rec, req.SessionID)
	}()

	// Goroutine: read from WebSocket → write to SSH stdin
	go func() {
		defer wg.Done()
		readWebSocketInput(ctx, ws, stdinPipe, sshSession)
	}()

	wg.Wait()
}

// HandleMonitor handles observer (monitor) mode WebSocket connections.
// A monitor watches an active session in real-time without sending input.
//
// GET /api/v1/terminal/monitor/:session_id
func (h *TerminalHandler) HandleMonitor(c *gin.Context) {
	sessionID := c.Param("session_id")

	ws, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		terminalLog.Error("ws upgrade error", log.Err(err))
		return
	}
	defer ws.Close()

	// Find the session being monitored
	ss := session.GlobalSessionManager.Get(sessionID)
	if ss == nil {
		writeWSError(ws, "session not found or offline")
		return
	}

	// Register as observer
	obID := fmt.Sprintf("ob-%s-%d", sessionID, time.Now().UnixNano())
	obSession := &session.Session{
		ID:        obID,
		Protocol:  ss.Protocol,
		Mode:      "watcher",
		WebSocket: ws,
	}
	ss.Observer.Add(obSession)
	defer ss.Observer.Remove(obID)

	// Send connected message
	writeWSMsg(ws, session.NewMessage(MsgConnected, ""))

	// Keep connection alive until WebSocket disconnects
	for {
		_, _, err := ws.ReadMessage()
		if err != nil {
			break
		}
	}
}

// writeTerminalOutput reads from the SSH stdout reader and sends data
// to the WebSocket client, optionally recording and broadcasting to observers.
func writeTerminalOutput(ctx context.Context, ws *websocket.Conn, reader *bufio.Reader, rec *recorder.Recorder, sessionID string) {
	tick := time.NewTicker(60 * time.Millisecond)
	defer tick.Stop()

	var buf bytesBuffer
	for {
		select {
		case <-ctx.Done():
			return
		default:
			rn, size, err := reader.ReadRune()
			if err != nil {
				if err != io.EOF {
					terminalLog.Warn("read rune error", log.Err(err))
				}
				return
			}
			if size > 0 {
				if rn != utf8.RuneError {
					p := make([]byte, utf8.RuneLen(rn))
					utf8.EncodeRune(p, rn)
					buf.Write(p)
				} else {
					buf.Write([]byte("@"))
				}
			}

			// Flush on tick
			select {
			case <-tick.C:
				s := buf.String()
				if s == "" {
					continue
				}
				msg := session.NewMessage(MsgData, s)
				writeWSMsg(ws, msg)

				// Record
				if rec != nil {
					if err := rec.WriteData(s); err != nil {
						terminalLog.Warn("recorder write error", log.Err(err))
					}
				}

				// Broadcast to observers
				broadcastToObservers(sessionID, s)
				buf.Reset()
			default:
			}
		}
	}
}

// readWebSocketInput reads messages from the WebSocket and writes
// data to the SSH stdin pipe.
func readWebSocketInput(ctx context.Context, ws *websocket.Conn, stdinPipe io.WriteCloser, sshSession *ssh.Session) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
			_, message, err := ws.ReadMessage()
			if err != nil {
				return
			}

			var msg session.Message
			if err := json.Unmarshal(message, &msg); err != nil {
				continue
			}

			switch msg.Type {
			case MsgData:
				if _, err := stdinPipe.Write([]byte(msg.Content)); err != nil {
					terminalLog.Warn("stdin write error", log.Err(err))
					return
				}
			case MsgResize:
				var winSize struct {
					Rows int `json:"rows"`
					Cols int `json:"cols"`
				}
				decoded, err := base64.StdEncoding.DecodeString(msg.Content)
				if err != nil {
					continue
				}
				if err := json.Unmarshal(decoded, &winSize); err != nil {
					continue
				}
				_ = sshSession.WindowChange(winSize.Rows, winSize.Cols)
			case MsgPing:
				// Respond with pong
				writeWSMsg(ws, session.NewMessage(MsgPing, ""))
			case MsgClosed:
				return
			}
		}
	}
}

// broadcastToObservers sends terminal output to all observers of a session.
func broadcastToObservers(sessionID, data string) {
	ss := session.GlobalSessionManager.Get(sessionID)
	if ss == nil || ss.Observer == nil {
		return
	}
	msg := session.NewMessage(MsgData, data)
	ss.Observer.Range(func(key string, ob *session.Session) bool {
		if err := ob.WriteMessage(msg); err != nil {
			terminalLog.Warn("observer write error",
				log.String("observer_id", key),
				log.Err(err),
			)
		}
		return true
	})
}

// writeWSMsg sends a session.Message to the WebSocket.
func writeWSMsg(ws *websocket.Conn, msg session.Message) {
	if err := ws.WriteMessage(websocket.TextMessage, []byte(msg.ToString())); err != nil {
		terminalLog.Warn("ws write error", log.Err(err))
	}
}

// writeWSError sends an error (MsgClosed) message to the WebSocket.
func writeWSError(ws *websocket.Conn, errMsg string) {
	msg := session.NewMessage(MsgClosed, errMsg)
	writeWSMsg(ws, msg)
}

// bytesBuffer is a simple goroutine-safe byte buffer for terminal output.
type bytesBuffer struct {
	mu  sync.Mutex
	buf []byte
}

func (b *bytesBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.buf = append(b.buf, p...)
	return len(p), nil
}

func (b *bytesBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	s := string(b.buf)
	b.buf = b.buf[:0]
	return s
}

func (b *bytesBuffer) Reset() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.buf = b.buf[:0]
}

// Ensure strconv import is used (referenced in interface but not directly in this file).
var _ = strconv.Itoa
