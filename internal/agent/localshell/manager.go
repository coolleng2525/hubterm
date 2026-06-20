package localshell

import (
	"fmt"
	"io"
	"os/exec"
	"sync"
)

type Session struct {
	cmd   *exec.Cmd
	stdin io.WriteCloser
}
type Manager struct {
	mu       sync.RWMutex
	sessions map[string]*Session
}

func NewManager() *Manager { return &Manager{sessions: make(map[string]*Session)} }

func (m *Manager) Start(sessionID, shellID, path string, output func([]byte), exited func(error)) error {
	args := []string{}
	switch shellID {
	case "cmd":
		args = []string{"/Q"}
	case "powershell", "pwsh":
		args = []string{"-NoLogo"}
	case "git-bash":
		args = []string{"--login", "-i"}
	}
	cmd := exec.Command(path, args...)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	m.mu.Lock()
	m.sessions[sessionID] = &Session{cmd: cmd, stdin: stdin}
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
	go func() { err := cmd.Wait(); m.mu.Lock(); delete(m.sessions, sessionID); m.mu.Unlock(); exited(err) }()
	return nil
}

func (m *Manager) Write(id string, data []byte) error {
	m.mu.RLock()
	s := m.sessions[id]
	m.mu.RUnlock()
	if s == nil {
		return fmt.Errorf("shell session not found")
	}
	_, err := s.stdin.Write(data)
	return err
}
func (m *Manager) Close(id string) {
	m.mu.Lock()
	s := m.sessions[id]
	delete(m.sessions, id)
	m.mu.Unlock()
	if s != nil && s.cmd.Process != nil {
		_ = s.cmd.Process.Kill()
	}
}
