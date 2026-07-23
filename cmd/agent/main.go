package main

import (
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/coolleng2525/hubterm/internal/agent/collector"
	"github.com/coolleng2525/hubterm/internal/agent/connector"
	"github.com/coolleng2525/hubterm/internal/agent/discovery"
	"github.com/coolleng2525/hubterm/internal/agent/executor"
	"github.com/coolleng2525/hubterm/internal/agent/localshell"
	"github.com/coolleng2525/hubterm/internal/agent/remotessh"
	"github.com/coolleng2525/hubterm/internal/agent/reporter"
	"github.com/coolleng2525/hubterm/internal/agent/serialsession"
	"github.com/coolleng2525/hubterm/internal/agent/service"
	"github.com/coolleng2525/hubterm/internal/pkg/session"
	hubtermproto "github.com/coolleng2525/hubterm/internal/proto"
	"github.com/google/uuid"
)

// NodeConfig 节点本地配置
type NodeConfig struct {
	NodeID string `json:"node_id"`
	Token  string `json:"token,omitempty"`
}

// loadOrCreateConfig 加载或创建节点配置
func loadOrCreateConfig(dataDir string) *NodeConfig {
	configPath := filepath.Join(dataDir, "node.json")
	cfg := &NodeConfig{}

	if data, err := os.ReadFile(configPath); err == nil {
		if json.Unmarshal(data, cfg) == nil && cfg.NodeID != "" {
			return cfg
		}
	}

	cfg.NodeID = uuid.New().String()
	os.MkdirAll(dataDir, 0755)
	data, _ := json.Marshal(cfg)
	os.WriteFile(configPath, data, 0600)
	return cfg
}

// saveConfig 保存节点配置到磁盘
func saveConfig(dataDir string, cfg *NodeConfig) {
	configPath := filepath.Join(dataDir, "node.json")
	data, _ := json.Marshal(cfg)
	os.WriteFile(configPath, data, 0600)
}

// installService 安装为系统服务
func installService(execPath string) {
	log.Printf("Installing system service: hubterm-agent")
	if err := service.Install("hubterm-agent", "HubTerm Node Agent", execPath); err != nil {
		log.Fatalf("Failed to install service: %v", err)
	}
	log.Printf("Service installed successfully")
}

func main() {
	centerURL := flag.String("center", "", "Center service URL (e.g. http://localhost:8080)")
	nodeName := flag.String("name", "", "Node display name (default: hostname)")
	nodeIP := flag.String("ip", "", "Reported node IP (default: outbound IP toward center)")
	dataDir := flag.String("data", "./data", "Data directory for node config")
	installFlag := flag.Bool("install", false, "Install as system service")
	domain := flag.String("domain", "", "Auto-discovery domain (e.g. mycompany.com)")
	flag.Parse()

	// 安装模式
	if *installFlag {
		execPath, _ := os.Executable()
		installService(execPath)
		return
	}

	// 确定中心地址（优先级：--center > --domain 自发现 > 环境变量）
	centerAddr := *centerURL
	if centerAddr == "" {
		if *domain != "" {
			log.Printf("Discovering center via domain: %s", *domain)
			result, err := discovery.Discover(*domain)
			if err != nil {
				log.Fatalf("Auto-discovery failed for domain %q: %v", *domain, err)
			}
			centerAddr = result.CenterURL
			log.Printf("Discovered center at %s (method: %s)", centerAddr, result.Method)
		} else if envURL := os.Getenv("HUBTERM_CENTER_URL"); envURL != "" {
			centerAddr = envURL
			log.Printf("Using center URL from HUBTERM_CENTER_URL: %s", centerAddr)
		} else {
			fmt.Println("Error: no center URL specified.")
			fmt.Println("Provide one of:")
			fmt.Println("  --center <url>         Center service URL directly")
			fmt.Println("  --domain <domain>      Auto-discover center via DNS/mDNS")
			fmt.Println("  HUBTERM_CENTER_URL     Environment variable")
			flag.Usage()
			os.Exit(1)
		}
	}

	cfg := loadOrCreateConfig(*dataDir)

	if *nodeName == "" {
		hostname, _ := os.Hostname()
		*nodeName = hostname
	}

	log.Printf("Agent starting: node_id=%s center=%s name=%s", cfg.NodeID, centerAddr, *nodeName)

	shellManager := localshell.NewManager()
	sshManager := remotessh.NewManager()
	serialManager := serialsession.NewManager()

	// 创建上报器
	rep := reporter.NewReporter(centerAddr, cfg.NodeID, *nodeName)
	rep.NodeIP = *nodeIP
	rep.SetSessionProvider(func() []hubtermproto.SessionInfo {
		sessions := shellManager.List()
		sessions = append(sessions, sshManager.List()...)
		sessions = append(sessions, serialManager.List()...)
		return sessions
	})
	conn := connector.New(centerAddr, cfg.NodeID, cfg.Token)
	conn.SetDisconnectHandler(serialManager.CloseAll)
	rep.SetTokenHandler(func(token string) {
		cfg.Token = token
		saveConfig(*dataDir, cfg)
		conn.SetNodeToken(token)
		log.Printf("Node token saved to disk")
	})
	if cfg.Token != "" {
		rep.SetNodeToken(cfg.Token)
		log.Printf("Loaded saved node token")
	}

	// 首次立即上报
	if err := rep.Report(); err != nil {
		log.Printf("Initial report error: %v", err)
	}

	// 保存从首次上报获取的 token
	// 定期上报 (每 3 秒)
	go rep.Start(3 * time.Second)

	// 建立 WebSocket 连接
	// 注册命令处理器
	conn.SetCommandHandler(func(cmd *connector.CenterCommand) {
		log.Printf("Received command: id=%s type=%s command=%s",
			cmd.ID, cmd.Type, cmd.Payload.Command)

		switch cmd.Type {
		case "shell_start":
			var shellPath string
			for _, shell := range collector.ScanShells() {
				if shell.ID == cmd.Payload.Shell {
					shellPath = shell.Path
					break
				}
			}
			if shellPath == "" {
				_ = conn.SendTerminalData(cmd.Payload.SessionID, "output", base64.StdEncoding.EncodeToString([]byte("Shell is not installed\r\n")))
				return
			}
			err := shellManager.Start(cmd.Payload.SessionID, cmd.Payload.Shell, shellPath, func(data []byte) {
				_ = conn.SendTerminalData(cmd.Payload.SessionID, "output", base64.StdEncoding.EncodeToString(data))
			}, func(err error) {
				_ = conn.SendTerminalData(cmd.Payload.SessionID, "output", base64.StdEncoding.EncodeToString([]byte("\r\nShell exited\r\n")))
			})
			if err != nil {
				_ = conn.SendTerminalData(cmd.Payload.SessionID, "output", base64.StdEncoding.EncodeToString([]byte(err.Error()+"\r\n")))
			}
		case "ssh_start":
			err := sshManager.Start(remotessh.Config{
				SessionID:   cmd.Payload.SessionID,
				DisplayName: cmd.Payload.DisplayName,
				Host:        cmd.Payload.Host,
				Port:        cmd.Payload.Port,
				Username:    cmd.Payload.Username,
				Password:    cmd.Payload.Password,
				PrivateKey:  cmd.Payload.PrivateKey,
				Passphrase:  cmd.Payload.Passphrase,
				Rows:        cmd.Payload.Rows,
				Cols:        cmd.Payload.Cols,
			}, func(data []byte) {
				_ = conn.SendTerminalData(cmd.Payload.SessionID, "output", base64.StdEncoding.EncodeToString(data))
			}, func(err error) {
				_ = conn.SendTerminalData(cmd.Payload.SessionID, "output", base64.StdEncoding.EncodeToString([]byte("\r\nSSH session exited\r\n")))
			})
			if err != nil {
				_ = conn.SendTerminalData(cmd.Payload.SessionID, "output", base64.StdEncoding.EncodeToString([]byte("SSH error: "+err.Error()+"\r\n")))
				_ = conn.SendResult(cmd.ID, &hubtermproto.ExecResult{CmdID: cmd.ID, Stderr: err.Error(), ExitCode: 1})
				return
			}
			_ = conn.SendResult(cmd.ID, &hubtermproto.ExecResult{CmdID: cmd.ID, ExitCode: 0})
		case "serial_start":
			if cmd.Payload.Serial == nil {
				_ = conn.SendResult(cmd.ID, &hubtermproto.ExecResult{CmdID: cmd.ID, Stderr: "serial config is required", ExitCode: 1})
				return
			}
			err := serialManager.Start(cmd.Payload.SessionID, *cmd.Payload.Serial, func(data []byte) {
				_ = conn.SendTerminalData(cmd.Payload.SessionID, "output", base64.StdEncoding.EncodeToString(data))
			}, func(err error) {
				state := hubtermproto.TerminalState{SessionID: cmd.Payload.SessionID, Status: "closed"}
				if err != nil {
					state.Status = "error"
					state.Error = err.Error()
				}
				_ = conn.SendTerminalState(state)
			})
			if err != nil {
				_ = conn.SendResult(cmd.ID, &hubtermproto.ExecResult{CmdID: cmd.ID, Stderr: err.Error(), ExitCode: 1})
				return
			}
			_ = conn.SendTerminalState(hubtermproto.TerminalState{SessionID: cmd.Payload.SessionID, Status: "open"})
			_ = conn.SendResult(cmd.ID, &hubtermproto.ExecResult{CmdID: cmd.ID, ExitCode: 0})
		case "write":
			data, err := base64.StdEncoding.DecodeString(cmd.Payload.Data)
			if err == nil {
				if err := serialManager.Write(cmd.Payload.SessionID, data); err != nil {
					if err := shellManager.Write(cmd.Payload.SessionID, data); err != nil {
						_ = sshManager.Write(cmd.Payload.SessionID, data)
					}
				}
			}
		case "shell_close":
			shellManager.Close(cmd.Payload.SessionID)
			sshManager.Close(cmd.Payload.SessionID)
		case "serial_close":
			_ = serialManager.Close(cmd.Payload.SessionID)
			_ = conn.SendResult(cmd.ID, &hubtermproto.ExecResult{CmdID: cmd.ID, ExitCode: 0})
		case "resize":
			_ = sshManager.Resize(cmd.Payload.SessionID, cmd.Payload.Rows, cmd.Payload.Cols)
		case "exec":
			timeout := time.Duration(cmd.Payload.Timeout) * time.Second
			if timeout <= 0 {
				timeout = 30 * time.Second // 默认 30 秒超时
			}
			result, err := executor.Execute(cmd.Payload.Command, timeout)
			if err != nil {
				log.Printf("Execute error: %v", err)
				_ = conn.SendResult(cmd.ID, &hubtermproto.ExecResult{
					CmdID:    cmd.ID,
					Stdout:   "",
					Stderr:   err.Error(),
					ExitCode: -1,
					Duration: 0,
				})
				return
			}
			_ = conn.SendResult(cmd.ID, &hubtermproto.ExecResult{
				CmdID:    cmd.ID,
				Stdout:   result.Stdout,
				Stderr:   result.Stderr,
				ExitCode: result.ExitCode,
				Duration: result.Duration,
			})
			log.Printf("Command completed: id=%s exit_code=%d duration=%dms",
				cmd.ID, result.ExitCode, result.Duration)

		case "ping":
			_ = conn.SendReport(map[string]string{
				"type":    "pong",
				"node_id": cfg.NodeID,
			})

		case "kick_session":
			session.GlobalSessionManager.Remove(cmd.Payload.SessionID)
			shellManager.Close(cmd.Payload.SessionID)
			sshManager.Close(cmd.Payload.SessionID)
			_ = serialManager.Close(cmd.Payload.SessionID)
			_ = conn.SendResult(cmd.ID, &hubtermproto.ExecResult{CmdID: cmd.ID, ExitCode: 0})

		case "assign_master":
			target := session.GlobalSessionManager.Get(cmd.Payload.SessionID)
			if target == nil {
				_ = conn.SendResult(cmd.ID, &hubtermproto.ExecResult{CmdID: cmd.ID, Stderr: "session not found", ExitCode: 1})
				return
			}
			target.SetMode("master")
			_ = conn.SendResult(cmd.ID, &hubtermproto.ExecResult{CmdID: cmd.ID, ExitCode: 0})

		default:
			log.Printf("Unknown command type: %s", cmd.Type)
		}
	})

	// 启动 WebSocket 连接
	go conn.Connect()

	// 等待退出信号
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	sig := <-sigCh
	fmt.Printf("\nReceived signal %v, shutting down...\n", sig)
	serialManager.CloseAll()
	conn.Close()
}
