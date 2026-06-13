// FIXED: renamed from proto to hubtermproto to avoid generic package name
package hubtermproto

// NodeReport 节点上报数据
type NodeReport struct {
	NodeID   string       `json:"node_id"`
	Name     string       `json:"name"`
	IP       string       `json:"ip"`
	Hostname string       `json:"hostname"`
	OS       string       `json:"os"`
	OSVersion string      `json:"os_version"`
	Arch     string       `json:"arch"`
	CPUPercent float64    `json:"cpu_percent"`
	MemoryTotal uint64    `json:"memory_total"`
	MemoryUsed  uint64    `json:"memory_used"`
	MemoryPercent float64 `json:"memory_percent"`
	DiskTotal   uint64    `json:"disk_total"`
	DiskUsed    uint64    `json:"disk_used"`
	SerialPorts []SerialPortInfo `json:"serial_ports"`
	Sessions   []SessionInfo    `json:"sessions"`
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
