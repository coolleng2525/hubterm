package remotessh

import (
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/coolleng2525/hubterm/internal/pkg/sshclient"
	hubtermproto "github.com/coolleng2525/hubterm/internal/proto"
	"golang.org/x/crypto/ssh"
)

type Config struct {
	SessionID   string
	DisplayName string
	Host        string
	Port        int
	Username    string
	Password    string
	PrivateKey  string
	Passphrase  string
	Rows        int
	Cols        int
}

type Session struct {
	client      *ssh.Client
	session     *ssh.Session
	stdin       io.WriteCloser
	displayName string
	host        string
	port        int
	username    string
	connectedAt time.Time
}

type Manager struct {
	mu       sync.RWMutex
	sessions map[string]*Session
}

func NewManager() *Manager {
	return &Manager{sessions: make(map[string]*Session)}
}

func (m *Manager) Start(cfg Config, output func([]byte), exited func(error)) error {
	cfg.Host = strings.TrimSpace(cfg.Host)
	cfg.Username = strings.TrimSpace(cfg.Username)
	if cfg.SessionID == "" {
		return fmt.Errorf("session_id is required")
	}
	if cfg.Host == "" {
		return fmt.Errorf("host is required")
	}
	if cfg.Port <= 0 {
		cfg.Port = 22
	}
	if cfg.Rows <= 0 {
		cfg.Rows = 24
	}
	if cfg.Cols <= 0 {
		cfg.Cols = 100
	}

	client, err := sshclient.Dial(cfg.Host, cfg.Port, cfg.Username, cfg.Password, cfg.PrivateKey, cfg.Passphrase)
	if err != nil {
		return err
	}
	sshSession, err := client.NewSession()
	if err != nil {
		_ = client.Close()
		return err
	}
	stdin, err := sshSession.StdinPipe()
	if err != nil {
		_ = sshSession.Close()
		_ = client.Close()
		return err
	}
	stdout, err := sshSession.StdoutPipe()
	if err != nil {
		_ = sshSession.Close()
		_ = client.Close()
		return err
	}
	stderr, err := sshSession.StderrPipe()
	if err != nil {
		_ = sshSession.Close()
		_ = client.Close()
		return err
	}
	modes := ssh.TerminalModes{
		ssh.ECHO:          1,
		ssh.TTY_OP_ISPEED: 14400,
		ssh.TTY_OP_OSPEED: 14400,
	}
	if err := sshSession.RequestPty("xterm-256color", cfg.Rows, cfg.Cols, modes); err != nil {
		_ = sshSession.Close()
		_ = client.Close()
		return err
	}
	if err := sshSession.Shell(); err != nil {
		_ = sshSession.Close()
		_ = client.Close()
		return err
	}

	m.mu.Lock()
	m.sessions[cfg.SessionID] = &Session{
		client:      client,
		session:     sshSession,
		stdin:       stdin,
		displayName: strings.TrimSpace(cfg.DisplayName),
		host:        cfg.Host,
		port:        cfg.Port,
		username:    cfg.Username,
		connectedAt: time.Now(),
	}
	m.mu.Unlock()

	read := func(r io.Reader) {
		buf := make([]byte, 32*1024)
		for {
			n, err := r.Read(buf)
			if n > 0 {
				output(append([]byte(nil), buf[:n]...))
			}
			if err != nil {
				return
			}
		}
	}
	go read(stdout)
	go read(stderr)
	go func() {
		err := sshSession.Wait()
		m.Close(cfg.SessionID)
		exited(err)
	}()
	return nil
}

func (m *Manager) Write(id string, data []byte) error {
	m.mu.RLock()
	s := m.sessions[id]
	m.mu.RUnlock()
	if s == nil {
		return fmt.Errorf("ssh session not found")
	}
	_, err := s.stdin.Write(data)
	return err
}

func (m *Manager) Resize(id string, rows, cols int) error {
	if rows <= 0 || cols <= 0 {
		return nil
	}
	m.mu.RLock()
	s := m.sessions[id]
	m.mu.RUnlock()
	if s == nil {
		return fmt.Errorf("ssh session not found")
	}
	return s.session.WindowChange(rows, cols)
}

func (m *Manager) Close(id string) {
	m.mu.Lock()
	s := m.sessions[id]
	delete(m.sessions, id)
	m.mu.Unlock()
	if s != nil {
		_ = s.session.Close()
		_ = s.client.Close()
	}
}

func (m *Manager) List() []hubtermproto.SessionInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()
	sessions := make([]hubtermproto.SessionInfo, 0, len(m.sessions))
	for id, s := range m.sessions {
		user := s.username
		if user == "" || user == "-" {
			user = "root"
		}
		sessions = append(sessions, hubtermproto.SessionInfo{
			SessionID:   id,
			DisplayName: s.displayName,
			PortName:    fmt.Sprintf("%s:%d", s.host, s.port),
			User:        user,
			Type:        "master",
			ClientIP:    "agent",
			ConnectedAt: s.connectedAt.Unix(),
		})
	}
	return sessions
}
