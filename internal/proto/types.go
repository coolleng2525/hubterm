// FIXED: renamed from proto to hubtermproto to avoid generic package name
package hubtermproto

import (
	"fmt"
	"strings"
)

const (
	SerialParityNone = "none"
	SerialParityOdd  = "odd"
	SerialParityEven = "even"

	SerialFlowNone   = "none"
	SerialFlowRTSCTS = "rtscts"
)

var supportedSerialBaudRates = map[int]struct{}{
	1200: {}, 2400: {}, 4800: {}, 9600: {}, 19200: {},
	38400: {}, 57600: {}, 115200: {}, 230400: {},
}

// SerialConfig is the persisted and transmitted configuration for a serial port.
type SerialConfig struct {
	PortName    string `json:"port_name"`
	BaudRate    int    `json:"baud_rate"`
	DataBits    int    `json:"data_bits"`
	Parity      string `json:"parity"`
	StopBits    int    `json:"stop_bits"`
	FlowControl string `json:"flow_control"`
}

func DefaultSerialConfig(portName string) SerialConfig {
	return SerialConfig{
		PortName:    portName,
		BaudRate:    115200,
		DataBits:    8,
		Parity:      SerialParityNone,
		StopBits:    1,
		FlowControl: SerialFlowNone,
	}
}

func (c SerialConfig) Validate() error {
	portName := strings.TrimSpace(c.PortName)
	if portName == "" || len(portName) > 256 {
		return fmt.Errorf("invalid serial port name")
	}
	if _, ok := supportedSerialBaudRates[c.BaudRate]; !ok {
		return fmt.Errorf("unsupported baud rate: %d", c.BaudRate)
	}
	if c.DataBits < 5 || c.DataBits > 8 {
		return fmt.Errorf("data bits must be between 5 and 8")
	}
	if c.Parity != SerialParityNone && c.Parity != SerialParityOdd && c.Parity != SerialParityEven {
		return fmt.Errorf("unsupported parity: %s", c.Parity)
	}
	if c.StopBits != 1 && c.StopBits != 2 {
		return fmt.Errorf("stop bits must be 1 or 2")
	}
	if c.FlowControl != SerialFlowNone && c.FlowControl != SerialFlowRTSCTS {
		return fmt.Errorf("unsupported flow control: %s", c.FlowControl)
	}
	return nil
}

// NetworkInterfaceInfo 网络接口信息
type NetworkInterfaceInfo struct {
	Name string `json:"name"`
	IP   string `json:"ip"`
}

// NodeReport 节点上报数据
type NodeReport struct {
	NodeID        string                 `json:"node_id"`
	Source        string                 `json:"source,omitempty"`
	Name          string                 `json:"name"`
	IP            string                 `json:"ip"`
	Hostname      string                 `json:"hostname"`
	OS            string                 `json:"os"`
	OSVersion     string                 `json:"os_version"`
	Arch          string                 `json:"arch"`
	CPUPercent    float64                `json:"cpu_percent"`
	MemoryTotal   uint64                 `json:"memory_total"`
	MemoryUsed    uint64                 `json:"memory_used"`
	MemoryPercent float64                `json:"memory_percent"`
	DiskTotal     uint64                 `json:"disk_total"`
	DiskUsed      uint64                 `json:"disk_used"`
	Interfaces    []NetworkInterfaceInfo `json:"interfaces"`
	SerialPorts   []SerialPortInfo       `json:"serial_ports"`
	Sessions      []SessionInfo          `json:"sessions"`
	Ser2net       *Ser2netStatus         `json:"ser2net,omitempty"`
	Shells        []ShellInfo            `json:"shells,omitempty"`
}

type ShellInfo struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Path string `json:"path"`
}

// Ser2netStatus ser2net 安装和运行状态
type Ser2netStatus struct {
	Installed  bool          `json:"installed"`
	Running    bool          `json:"running"`
	Version    string        `json:"version"`
	ConfigPath string        `json:"config_path"`
	Ports      []Ser2netPort `json:"ports"`
}

// Ser2netPort ser2net 配置中定义的串口映射
type Ser2netPort struct {
	TCPPort  int    `json:"tcp_port"`
	Device   string `json:"device"`
	BaudRate int    `json:"baud_rate"`
	Enabled  bool   `json:"enabled"`
}

// SerialPortInfo 串口信息
type SerialPortInfo struct {
	PortName    string `json:"port_name"`
	Description string `json:"description"`
	Status      string `json:"status"` // online/offline/busy
	BaudRate    int    `json:"baud_rate"`
}

// SessionInfo 会话信息
type SessionInfo struct {
	SessionID   string `json:"session_id"`
	DisplayName string `json:"display_name,omitempty"`
	Protocol    string `json:"protocol,omitempty"`
	PortName    string `json:"port_name"`
	User        string `json:"user"`
	Type        string `json:"type"` // master/watcher
	ClientIP    string `json:"client_ip"`
	ConnectedAt int64  `json:"connected_at"`
}

// CommandRequest 指令下发
type CommandRequest struct {
	NodeID  string `json:"node_id"`
	Command string `json:"command"`
	Params  string `json:"params,omitempty"`
}

// CommandResponse 指令响应
type CommandResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

// WSMessage WebSocket 消息
type WSMessage struct {
	Type string      `json:"type"`
	Data interface{} `json:"data"`
}

// TerminalData carries terminal bytes between an agent and center.
type TerminalData struct {
	SessionID string `json:"session_id"`
	Direction string `json:"direction"` // input / output
	Data      string `json:"data"`      // base64 encoded bytes
}

type TerminalSubscription struct {
	NodeID    string `json:"node_id"`
	SessionID string `json:"session_id"`
}

type TerminalInput struct {
	NodeID    string `json:"node_id"`
	SessionID string `json:"session_id"`
	Data      string `json:"data"` // base64 encoded bytes
}

// TerminalState reports lifecycle changes that happen after a command reply.
type TerminalState struct {
	SessionID string `json:"session_id"`
	Status    string `json:"status"` // open / closed / error
	Error     string `json:"error,omitempty"`
}

// ExecCommand 中心下发的命令执行请求
type ExecCommand struct {
	ID      string `json:"id"`
	Type    string `json:"type"` // exec / shell / ping / restart
	Payload struct {
		Command     string        `json:"command,omitempty"`
		Timeout     int           `json:"timeout,omitempty"` // 秒
		SessionID   string        `json:"session_id,omitempty"`
		Data        string        `json:"data,omitempty"`
		Shell       string        `json:"shell,omitempty"`
		Rows        int           `json:"rows,omitempty"`
		Cols        int           `json:"cols,omitempty"`
		DisplayName string        `json:"display_name,omitempty"`
		Host        string        `json:"host,omitempty"`
		Port        int           `json:"port,omitempty"`
		Username    string        `json:"username,omitempty"`
		Password    string        `json:"password,omitempty"`
		PrivateKey  string        `json:"private_key,omitempty"`
		Passphrase  string        `json:"passphrase,omitempty"`
		Serial      *SerialConfig `json:"serial,omitempty"`
	} `json:"payload,omitempty"`
}

// ExecResult 命令执行结果
type ExecResult struct {
	CmdID    string `json:"cmd_id"`
	Stdout   string `json:"stdout"`
	Stderr   string `json:"stderr"`
	ExitCode int    `json:"exit_code"`
	Duration int64  `json:"duration_ms"`
}

// ExecResponse 中心返回给节点的执行结果确认
type ExecResponse struct {
	CmdID  string      `json:"cmd_id"`
	Status string      `json:"status"` // pending / running / completed / failed
	Result *ExecResult `json:"result,omitempty"`
}

// ExecRequest API 请求体 — 向节点下发命令
type ExecRequest struct {
	Command string `json:"command"`
	Timeout int    `json:"timeout,omitempty"` // 秒，默认 30
}

// ExecStatusQuery 查询命令执行状态
type ExecStatusQuery struct {
	CmdID string `json:"cmd_id"`
}

// RegisterMessage 节点注册消息，agent 连接时发送给中心
// 包含节点标识信息和自发现域名。
type RegisterMessage struct {
	// NodeID 节点唯一标识
	NodeID string `json:"node_id"`
	// NodeName 节点显示名称
	NodeName string `json:"node_name"`
	// Token 节点认证令牌
	Token string `json:"token"`
	// Domain 自发现域名，agent 通过 --domain 指定
	// 中心可用此字段将节点分组到对应域下
	Domain string `json:"domain,omitempty"`
}
