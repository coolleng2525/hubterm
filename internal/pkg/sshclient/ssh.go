// Package sshclient provides SSH client dial functions with password and private key
// authentication, plus SOCKS5 proxy support for jump-host scenarios.
//
// Reference: Next Terminal server/common/term/ssh.go
package sshclient

import (
	"fmt"
	"net"
	"time"

	"golang.org/x/crypto/ssh"
	"golang.org/x/net/proxy"
)

// Dial establishes an SSH connection to the given host.
//
// Parameters:
//   - ip: target host IP or hostname
//   - port: SSH port (typically 22)
//   - username: login username; if empty or "-", defaults to "root"
//   - password: password for password auth; "-" treated as empty
//   - privateKey: PEM-encoded private key for key auth; "-" treated as empty
//   - passphrase: passphrase for encrypted private keys; "-" treated as empty
//
// Returns:
//   - *ssh.Client: connected SSH client
//   - error: nil on success, or an error describing the failure
//
// Error locations:
//   - ssh.ParsePrivateKey / ssh.ParsePrivateKeyWithPassphrase: invalid key material
//   - ssh.Dial: network unreachable, auth failure, timeout
func Dial(ip string, port int, username, password, privateKey, passphrase string) (*ssh.Client, error) {
	authMethod, err := buildAuthMethod(password, privateKey, passphrase)
	if err != nil {
		return nil, err
	}

	if username == "-" || username == "" {
		username = "root"
	}

	config := &ssh.ClientConfig{
		Timeout:         3 * time.Second,
		User:            username,
		Auth:            []ssh.AuthMethod{authMethod},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}

	addr := fmt.Sprintf("%s:%d", ip, port)
	return ssh.Dial("tcp", addr, config)
}

// DialViaSocks establishes an SSH connection through a SOCKS5 proxy.
//
// Parameters:
//   - ip: target host IP or hostname
//   - port: SSH port (typically 22)
//   - username: login username; if empty or "-", defaults to "root"
//   - password: password for password auth; "-" treated as empty
//   - privateKey: PEM-encoded private key for key auth; "-" treated as empty
//   - passphrase: passphrase for encrypted private keys; "-" treated as empty
//   - socksHost: SOCKS5 proxy host
//   - socksPort: SOCKS5 proxy port
//   - socksUser: SOCKS5 proxy username (optional)
//   - socksPass: SOCKS5 proxy password (optional)
//
// Returns:
//   - *ssh.Client: connected SSH client
//   - error: nil on success, or an error describing the failure
//
// Error locations:
//   - buildAuthMethod: invalid key material
//   - proxy.SOCKS5: invalid proxy address
//   - socks5.Dial: proxy unreachable
//   - ssh.NewClientConn: SSH handshake failure over proxy
func DialViaSocks(ip string, port int, username, password, privateKey, passphrase string,
	socksHost, socksPort, socksUser, socksPass string) (*ssh.Client, error) {

	authMethod, err := buildAuthMethod(password, privateKey, passphrase)
	if err != nil {
		return nil, err
	}

	if username == "-" || username == "" {
		username = "root"
	}

	config := &ssh.ClientConfig{
		Timeout:         3 * time.Second,
		User:            username,
		Auth:            []ssh.AuthMethod{authMethod},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}

	socksProxyAddr := fmt.Sprintf("%s:%s", socksHost, socksPort)
	socks5, err := proxy.SOCKS5("tcp", socksProxyAddr,
		&proxy.Auth{User: socksUser, Password: socksPass},
		&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("socks5 proxy dial: %w", err)
	}

	addr := fmt.Sprintf("%s:%d", ip, port)
	conn, err := socks5.Dial("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("socks5 dial target: %w", err)
	}

	clientConn, channels, requests, err := ssh.NewClientConn(conn, addr, config)
	if err != nil {
		return nil, fmt.Errorf("ssh handshake over socks5: %w", err)
	}

	return ssh.NewClient(clientConn, channels, requests), nil
}

// buildAuthMethod resolves the authentication method from the provided credentials.
// Private key takes precedence over password when both are provided.
func buildAuthMethod(password, privateKey, passphrase string) (ssh.AuthMethod, error) {
	if password == "-" {
		password = ""
	}
	if privateKey == "-" {
		privateKey = ""
	}
	if passphrase == "-" {
		passphrase = ""
	}

	if privateKey != "" {
		var key ssh.Signer
		var err error
		if passphrase != "" {
			key, err = ssh.ParsePrivateKeyWithPassphrase([]byte(privateKey), []byte(passphrase))
			if err != nil {
				return nil, fmt.Errorf("parse encrypted private key: %w", err)
			}
		} else {
			key, err = ssh.ParsePrivateKey([]byte(privateKey))
			if err != nil {
				return nil, fmt.Errorf("parse private key: %w", err)
			}
		}
		return ssh.PublicKeys(key), nil
	}

	return ssh.Password(password), nil
}
