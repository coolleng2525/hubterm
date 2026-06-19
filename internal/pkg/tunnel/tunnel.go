// Package tunnel provides SSH tunnel and gateway management for accessing
// internal network devices through a jump host (bastion).
//
// Reference: Next Terminal server/global/gateway/gateway.go, tunnel.go
package tunnel

import (
	"fmt"
	"io"
	"net"
	"sync"

	"golang.org/x/crypto/ssh"

	"github.com/coolleng2525/hubterm/internal/pkg/sshclient"
)

// Tunnel represents an SSH tunnel that forwards a local listening port
// to a remote target through an SSH connection.
type Tunnel struct {
	ID         string
	LocalHost  string
	LocalPort  int
	RemoteHost string
	RemotePort int
	listener   net.Listener

	mu                sync.Mutex
	localConnections  []net.Conn
	remoteConnections []net.Conn
}

// Open starts the tunnel: listens on LocalHost:LocalPort and forwards
// each incoming connection to RemoteHost:RemotePort via the SSH client.
//
// Parameters:
//   - sshClient: established SSH client used as the tunnel transport
//
// This function blocks while the tunnel is active. Run it in a goroutine.
func (t *Tunnel) Open(sshClient *ssh.Client) {
	for {
		localConn, err := t.listener.Accept()
		if err != nil {
			return
		}

		t.mu.Lock()
		t.localConnections = append(t.localConnections, localConn)
		t.mu.Unlock()

		remoteAddr := fmt.Sprintf("%s:%d", t.RemoteHost, t.RemotePort)
		remoteConn, err := sshClient.Dial("tcp", remoteAddr)
		if err != nil {
			localConn.Close()
			continue
		}

		t.mu.Lock()
		t.remoteConnections = append(t.remoteConnections, remoteConn)
		t.mu.Unlock()

		go copyConn(localConn, remoteConn)
		go copyConn(remoteConn, localConn)
	}
}

// Close stops the tunnel, closes all active connections and the listener.
func (t *Tunnel) Close() {
	t.mu.Lock()
	defer t.mu.Unlock()

	for _, c := range t.localConnections {
		c.Close()
	}
	t.localConnections = nil

	for _, c := range t.remoteConnections {
		c.Close()
	}
	t.remoteConnections = nil

	if t.listener != nil {
		t.listener.Close()
	}
}

// Gateway represents a jump host (bastion) that provides access to
// internal network devices via SSH tunnels.
type Gateway struct {
	ID         string
	IP         string
	Port       int
	Username   string
	Password   string
	PrivateKey string
	Passphrase string
	Connected  bool
	SshClient  *ssh.Client

	mu      sync.Mutex
	tunnels map[string]*Tunnel
}

// OpenTunnel creates an SSH tunnel through this gateway to reach the
// specified remote host and port. Returns the local listening address
// and port that forward to the target.
//
// Parameters:
//   - id: unique identifier for the tunnel (typically the session ID)
//   - remoteIP: target device IP reachable from the gateway
//   - remotePort: target device port
//
// Returns:
//   - localIP: local listening address (hostname of this machine)
//   - localPort: local listening port
//   - err: nil on success
//
// Error locations:
//   - Gateway connection failure (sshclient.Dial)
//   - getAvailablePort: no free port
//   - net.Listen: port binding failure
func (g *Gateway) OpenTunnel(id, remoteIP string, remotePort int) (localIP string, localPort int, err error) {
	g.mu.Lock()
	defer g.mu.Unlock()

	if !g.Connected {
		sshClient, err := sshclient.Dial(g.IP, g.Port, g.Username, g.Password, g.PrivateKey, g.Passphrase)
		if err != nil {
			g.Connected = false
			return "", 0, fmt.Errorf("gateway unreachable: %w", err)
		}
		g.Connected = true
		g.SshClient = sshClient
	}

	localPort, err = getAvailablePort()
	if err != nil {
		return "", 0, fmt.Errorf("get available port: %w", err)
	}

	localAddr := fmt.Sprintf("0.0.0.0:%d", localPort)
	listener, err := net.Listen("tcp", localAddr)
	if err != nil {
		return "", 0, fmt.Errorf("listen on %s: %w", localAddr, err)
	}

	tunnel := &Tunnel{
		ID:         id,
		LocalHost:  "0.0.0.0",
		LocalPort:  localPort,
		RemoteHost: remoteIP,
		RemotePort: remotePort,
		listener:   listener,
	}
	go tunnel.Open(g.SshClient)

	if g.tunnels == nil {
		g.tunnels = make(map[string]*Tunnel)
	}
	g.tunnels[tunnel.ID] = tunnel

	return tunnel.LocalHost, tunnel.LocalPort, nil
}

// CloseTunnel closes the tunnel identified by id and removes it from
// the gateway. If no tunnels remain, the gateway SSH connection is
// also closed.
//
// Parameters:
//   - id: tunnel identifier (typically the session ID)
func (g *Gateway) CloseTunnel(id string) {
	g.mu.Lock()
	defer g.mu.Unlock()

	t := g.tunnels[id]
	if t != nil {
		t.Close()
		delete(g.tunnels, id)
	}

	if len(g.tunnels) == 0 {
		if g.SshClient != nil {
			g.SshClient.Close()
		}
		g.Connected = false
	}
}

// Close closes all tunnels managed by this gateway and shuts down
// the gateway SSH connection.
func (g *Gateway) Close() {
	g.mu.Lock()
	defer g.mu.Unlock()

	for id := range g.tunnels {
		g.tunnels[id].Close()
		delete(g.tunnels, id)
	}

	if g.SshClient != nil {
		g.SshClient.Close()
	}
	g.Connected = false
}

// GatewayManager manages multiple gateways as a global singleton.
// Thread-safe via sync.Map.
type GatewayManager struct {
	gateways sync.Map
}

// GlobalGatewayManager is the global gateway manager instance.
var GlobalGatewayManager = &GatewayManager{}

// Add registers a gateway.
func (m *GatewayManager) Add(g *Gateway) {
	m.gateways.Store(g.ID, g)
}

// Get retrieves a gateway by ID.
//
// Returns nil if not found.
func (m *GatewayManager) Get(id string) *Gateway {
	val, ok := m.gateways.Load(id)
	if !ok {
		return nil
	}
	return val.(*Gateway)
}

// Remove unregisters and closes a gateway.
func (m *GatewayManager) Remove(id string) {
	val, ok := m.gateways.Load(id)
	if ok {
		g := val.(*Gateway)
		g.Close()
		m.gateways.Delete(id)
	}
}

// getAvailablePort finds a free TCP port on localhost.
func getAvailablePort() (int, error) {
	addr, err := net.ResolveTCPAddr("tcp", "localhost:0")
	if err != nil {
		return 0, err
	}
	l, err := net.ListenTCP("tcp", addr)
	if err != nil {
		return 0, err
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port, nil
}

// copyConn copies data bidirectionally between two connections.
func copyConn(writer, reader net.Conn) {
	io.Copy(writer, reader)
}
