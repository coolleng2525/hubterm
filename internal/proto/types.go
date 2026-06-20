// FIXED: renamed from proto to hubtermproto to avoid generic package name
package hubtermproto

// NetworkInterfaceInfo 网络接口信息
type NetworkInterfaceInfo struct {
	Name string `json:"name"`
	IP   string `json:"ip"`
}

// NodeReport 节点上报数据
type NodeReport struct {
	NodeID        string                 `json:"node_id"`
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

// ExecCommand 中心下发的命令执行请求
type ExecCommand struct {
	ID      string `json:"id"`
	Type    string `json:"type"` // exec / shell / ping / restart
	Payload struct {
		Command   string `json:"command,omitempty"`
		Timeout   int    `json:"timeout,omitempty"` // 秒
		SessionID string `json:"session_id,omitempty"`
		Data      string `json:"data,omitempty"`
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
