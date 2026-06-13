// Package session provides terminal session management with observer (multi-party)
// support. Sessions represent active terminal connections (SSH, serial, telnet)
// and can be shared by multiple viewers.
//
// Reference: Next Terminal server/global/session/session.go
package session

import (
	"encoding/json"
	"sync"

	"github.com/gorilla/websocket"
	"golang.org/x/crypto/ssh"
)

// Message represents a WebSocket message exchanged between the terminal
// handler and the frontend.
type Message struct {
	Type    int    `json:"type"`
	Content string `json:"content"`
}

// ToString serializes the message to JSON string.
func (m Message) ToString() string {
	data, _ := json.Marshal(m)
	return string(data)
}

// NewMessage creates a new Message.
func NewMessage(msgType int, content string) Message {
	return Message{Type: msgType, Content: content}
}

// Session represents an active terminal session.
type Session struct {
	ID         string            // unique session identifier
	Protocol   string            // ssh / serial / telnet
	Mode       string            // master / watcher
	WebSocket  *websocket.Conn   // WebSocket connection to the frontend
	SSHClient  *ssh.Client       // SSH client (nil for serial/telnet)
	SSHChannel ssh.Channel       // SSH channel (nil for serial/telnet)
	Observer   *Manager          // observer manager for session sharing
	mu         sync.Mutex
}

// WriteMessage sends a message to the WebSocket client.
//
// Parameters:
//   - msg: the message to send
//
// Returns:
//   - error: nil on success, or WebSocket write error
func (s *Session) WriteMessage(msg Message) error {
	if s.WebSocket == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.WebSocket.WriteMessage(websocket.TextMessage, []byte(msg.ToString()))
}

// Close cleans up all resources held by this session:
// WebSocket, SSH client/channel, and observer.
func (s *Session) Close() {
	if s.SSHChannel != nil {
		s.SSHChannel.Close()
	}
	if s.SSHClient != nil {
		s.SSHClient.Close()
	}
	if s.WebSocket != nil {
		s.WebSocket.Close()
	}
	if s.Observer != nil {
		s.Observer.Clear()
	}
}

// Manager manages sessions with thread-safe operations.
// Supports the observer pattern: a session can have its own observer
// Manager to broadcast output to watchers.
type Manager struct {
	id       string
	sessions sync.Map
}

// NewManager creates a new empty session manager.
func NewManager() *Manager {
	return &Manager{}
}

// NewObserver creates a session manager intended to act as an observer
// container for a specific session.
//
// Parameters:
//   - id: the session ID this observer belongs to
func NewObserver(id string) *Manager {
	return &Manager{
		id: id,
	}
}

// Add stores a session by its ID.
func (m *Manager) Add(s *Session) {
	m.sessions.Store(s.ID, s)
}

// Get retrieves a session by ID.
//
// Returns nil if not found.
func (m *Manager) Get(id string) *Session {
	val, ok := m.sessions.Load(id)
	if !ok {
		return nil
	}
	return val.(*Session)
}

// Remove deletes a session by ID, closing its resources first.
func (m *Manager) Remove(id string) {
	s := m.Get(id)
	if s != nil {
		s.Close()
		if s.Observer != nil {
			s.Observer.Clear()
		}
	}
	m.sessions.Delete(id)
}

// Clear removes and closes all sessions in this manager.
func (m *Manager) Clear() {
	m.sessions.Range(func(key, value interface{}) bool {
		if s, ok := value.(*Session); ok {
			s.Close()
		}
		m.sessions.Delete(key)
		return true
	})
}

// Range iterates over all sessions, calling f for each.
// Stops early if f returns false.
func (m *Manager) Range(f func(key string, value *Session) bool) {
	m.sessions.Range(func(key, value interface{}) bool {
		if s, ok := value.(*Session); ok {
			return f(key.(string), s)
		}
		return true
	})
}

// GlobalSessionManager is the global session manager instance.
var GlobalSessionManager = NewManager()
