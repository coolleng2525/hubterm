// Package serialsession manages interactive serial port sessions on an Agent.
package serialsession

import (
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"
	"sync"
	"time"

	serial "github.com/jacobsa/go-serial/serial"

	hubtermproto "github.com/coolleng2525/hubterm/internal/proto"
)

type opener func(hubtermproto.SerialConfig) (io.ReadWriteCloser, error)

type managedSession struct {
	port        io.ReadWriteCloser
	config      hubtermproto.SerialConfig
	connectedAt time.Time
	exited      func(error)
	done        chan struct{}
	finishOnce  sync.Once
}

// Manager owns the Agent's active serial port handles.
type Manager struct {
	mu       sync.RWMutex
	sessions map[string]*managedSession
	ports    map[string]string
	open     opener
}

func NewManager() *Manager {
	return newManager(openPort)
}

func newManager(open opener) *Manager {
	return &Manager{
		sessions: make(map[string]*managedSession),
		ports:    make(map[string]string),
		open:     open,
	}
}

func openPort(cfg hubtermproto.SerialConfig) (io.ReadWriteCloser, error) {
	parity := serial.PARITY_NONE
	switch cfg.Parity {
	case hubtermproto.SerialParityOdd:
		parity = serial.PARITY_ODD
	case hubtermproto.SerialParityEven:
		parity = serial.PARITY_EVEN
	}
	return serial.Open(serial.OpenOptions{
		PortName:          cfg.PortName,
		BaudRate:          uint(cfg.BaudRate),
		DataBits:          uint(cfg.DataBits),
		StopBits:          uint(cfg.StopBits),
		ParityMode:        parity,
		RTSCTSFlowControl: cfg.FlowControl == hubtermproto.SerialFlowRTSCTS,
		// A bounded read lets Close interrupt an idle macOS tty. With a fully
		// blocking VMIN=1 read, os.File.Close can wait forever for the device to
		// produce one more byte.
		InterCharacterTimeout: 100,
		MinimumReadSize:       0,
	})
}

// Start opens a serial port and starts forwarding raw output bytes.
func (m *Manager) Start(sessionID string, cfg hubtermproto.SerialConfig, output func([]byte), exited func(error)) error {
	sessionID = strings.TrimSpace(sessionID)
	cfg.PortName = strings.TrimSpace(cfg.PortName)
	if sessionID == "" {
		return fmt.Errorf("session_id is required")
	}
	if err := cfg.Validate(); err != nil {
		return err
	}
	if output == nil {
		output = func([]byte) {}
	}
	if exited == nil {
		exited = func(error) {}
	}

	// Keep the lock while opening so concurrent requests cannot open the same
	// physical port between the duplicate check and session registration.
	m.mu.Lock()
	if _, exists := m.sessions[sessionID]; exists {
		m.mu.Unlock()
		return fmt.Errorf("serial session already exists")
	}
	if existingID := m.ports[cfg.PortName]; existingID != "" {
		m.mu.Unlock()
		return fmt.Errorf("serial port is already in use by session %s", existingID)
	}
	port, err := m.open(cfg)
	if err != nil {
		m.mu.Unlock()
		return fmt.Errorf("open serial port %s: %w", cfg.PortName, err)
	}
	session := &managedSession{port: port, config: cfg, connectedAt: time.Now(), exited: exited, done: make(chan struct{})}
	m.sessions[sessionID] = session
	m.ports[cfg.PortName] = sessionID
	m.mu.Unlock()

	go m.readLoop(sessionID, session, output)
	return nil
}

func (m *Manager) readLoop(sessionID string, session *managedSession, output func([]byte)) {
	buffer := make([]byte, 32*1024)
	for {
		n, err := session.port.Read(buffer)
		if n > 0 {
			output(append([]byte(nil), buffer[:n]...))
		}
		if err != nil {
			if errors.Is(err, io.EOF) {
				select {
				case <-session.done:
					return
				default:
					// With VMIN=0/VTIME>0, an idle tty read is surfaced by
					// os.File as io.EOF. It is a timeout, not a disconnect.
					continue
				}
			}
			m.finish(sessionID, session, err)
			return
		}
	}
}

func (m *Manager) finish(sessionID string, session *managedSession, reason error) {
	session.finishOnce.Do(func() {
		close(session.done)
		m.mu.Lock()
		if m.sessions[sessionID] == session {
			delete(m.sessions, sessionID)
			delete(m.ports, session.config.PortName)
		}
		m.mu.Unlock()
		_ = session.port.Close()
		session.exited(reason)
	})
}

// Write sends all bytes to an active serial session.
func (m *Manager) Write(sessionID string, data []byte) error {
	m.mu.RLock()
	session := m.sessions[sessionID]
	m.mu.RUnlock()
	if session == nil {
		return fmt.Errorf("serial session not found")
	}
	for len(data) > 0 {
		n, err := session.port.Write(data)
		if err != nil {
			return err
		}
		if n <= 0 {
			return io.ErrShortWrite
		}
		data = data[n:]
	}
	return nil
}

func (m *Manager) Close(sessionID string) error {
	m.mu.RLock()
	session := m.sessions[sessionID]
	m.mu.RUnlock()
	if session == nil {
		return nil
	}
	m.finish(sessionID, session, nil)
	return nil
}

func (m *Manager) CloseAll() {
	m.mu.RLock()
	sessions := make(map[string]*managedSession, len(m.sessions))
	for id, session := range m.sessions {
		sessions[id] = session
	}
	m.mu.RUnlock()
	for id, session := range sessions {
		m.finish(id, session, nil)
	}
}

func (m *Manager) List() []hubtermproto.SessionInfo {
	m.mu.RLock()
	result := make([]hubtermproto.SessionInfo, 0, len(m.sessions))
	for id, session := range m.sessions {
		result = append(result, hubtermproto.SessionInfo{
			SessionID:   id,
			DisplayName: "Serial: " + session.config.PortName,
			Protocol:    "serial",
			PortName:    session.config.PortName,
			User:        "serial",
			Type:        "master",
			ClientIP:    "agent",
			ConnectedAt: session.connectedAt.Unix(),
		})
	}
	m.mu.RUnlock()
	sort.Slice(result, func(i, j int) bool { return result[i].SessionID < result[j].SessionID })
	return result
}
