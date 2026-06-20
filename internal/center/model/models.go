package model

import (
	"time"

	"gorm.io/gorm"
)

// User 用户
type User struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	Username     string    `gorm:"uniqueIndex;size:64;not null" json:"username"`
	PasswordHash string    `gorm:"size:256;not null" json:"-"`
	Role         string    `gorm:"size:32;not null;default:operator" json:"role"` // admin/operator/readonly
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// SSHProfile stores a user's reusable SSH connection settings. Secrets are encrypted at rest.
type SSHProfile struct {
	ID                  uint      `gorm:"primaryKey" json:"id"`
	UserID              uint      `gorm:"not null;index;uniqueIndex:idx_user_ssh_name" json:"-"`
	Name                string    `gorm:"size:128;not null;uniqueIndex:idx_user_ssh_name" json:"name"`
	NodeID              string    `gorm:"size:64;index" json:"node_id"`
	Host                string    `gorm:"size:256;not null" json:"host"`
	Port                int       `gorm:"not null;default:22" json:"port"`
	Username            string    `gorm:"size:128;not null" json:"username"`
	AuthType            string    `gorm:"size:16;not null" json:"auth_type"`
	EncryptedPassword   string    `gorm:"type:text" json:"-"`
	EncryptedPrivateKey string    `gorm:"type:text" json:"-"`
	EncryptedPassphrase string    `gorm:"type:text" json:"-"`
	CreatedAt           time.Time `json:"created_at"`
	UpdatedAt           time.Time `json:"updated_at"`
}

// Node 节点
type Node struct {
	ID            uint      `gorm:"primaryKey" json:"id"`
	NodeID        string    `gorm:"uniqueIndex;size:64;not null" json:"node_id"`
	Source        string    `gorm:"size:32;not null;default:agent" json:"source"`
	Name          string    `gorm:"size:128" json:"name"`
	IP            string    `gorm:"size:64" json:"ip"`
	Hostname      string    `gorm:"size:128" json:"hostname"`
	OS            string    `gorm:"size:64" json:"os"`
	OSVersion     string    `gorm:"size:128" json:"os_version"`
	Arch          string    `gorm:"size:32" json:"arch"`
	Status        string    `gorm:"size:32;not null;default:offline" json:"status"`
	CPUPercent    float64   `json:"cpu_percent"`
	MemoryTotal   uint64    `json:"memory_total"`
	MemoryUsed    uint64    `json:"memory_used"`
	MemoryPercent float64   `json:"memory_percent"`
	DiskTotal     uint64    `json:"disk_total"`
	DiskUsed      uint64    `json:"disk_used"`
	Interfaces    string    `gorm:"size:1024" json:"interfaces"` // JSON: [{"name":"eth0","ip":"192.168.1.55"}]
	Shells        string    `gorm:"size:4096" json:"shells"`
	LastSeen      time.Time `json:"last_seen"`
	Token         string    `gorm:"size:128" json:"-"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// SerialPort 串口
type SerialPort struct {
	ID               uint      `gorm:"primaryKey" json:"id"`
	NodeID           string    `gorm:"index;size:64;not null" json:"node_id"`
	PortName         string    `gorm:"size:64;not null" json:"port_name"`
	Description      string    `gorm:"size:256" json:"description"`
	Status           string    `gorm:"size:32;not null;default:offline" json:"status"` // online/offline/busy
	BaudRate         int       `json:"baud_rate"`
	CurrentSessionID string    `gorm:"size:64" json:"current_session_id"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

// Session 会话
type Session struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	SessionID   string    `gorm:"uniqueIndex;size:64;not null" json:"session_id"`
	NodeID      string    `gorm:"index;size:64;not null" json:"node_id"`
	PortName    string    `gorm:"size:64;not null" json:"port_name"`
	User        string    `gorm:"size:64" json:"user"`
	Type        string    `gorm:"size:32;not null;default:watcher" json:"type"` // master/watcher
	ClientIP    string    `gorm:"size:64" json:"client_ip"`
	ConnectedAt time.Time `json:"connected_at"`
	CreatedAt   time.Time `json:"created_at"`
}

// AuditLog 审计日志
type AuditLog struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	User      string    `gorm:"size:64" json:"user"`
	Action    string    `gorm:"size:64;not null;index" json:"action"`
	Target    string    `gorm:"size:256" json:"target"`
	Detail    string    `gorm:"size:1024" json:"detail"`
	IP        string    `gorm:"size:64" json:"ip"`
	CreatedAt time.Time `json:"created_at"`
}

// DeviceAlias 虚拟设备名 — hubterm://xxx 到真实设备的映射
type DeviceAlias struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Alias     string    `gorm:"uniqueIndex;size:128;not null" json:"alias"` // hubterm://ap-03
	DeviceID  string    `gorm:"size:64;not null" json:"device_id"`
	NodeID    string    `gorm:"size:64" json:"node_id"`
	Protocol  string    `gorm:"size:32" json:"protocol"` // serial/ssh
	CreatedAt time.Time `json:"created_at"`
}

// RemoteCenter 远程中心
type RemoteCenter struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Name      string    `gorm:"size:128" json:"name"`
	URL       string    `gorm:"size:256;not null" json:"url"`
	Token     string    `gorm:"size:256" json:"token"`
	Status    string    `gorm:"size:32;default:unknown" json:"status"`
	LastSync  time.Time `json:"last_sync"`
	CreatedAt time.Time `json:"created_at"`
}

// DeviceGroup 设备分组
type DeviceGroup struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Name      string    `gorm:"size:128;not null" json:"name"`
	Desc      string    `gorm:"size:256" json:"desc"`
	CreatedAt time.Time `json:"created_at"`
}

// DeviceGroupMember 设备组成员
type DeviceGroupMember struct {
	ID       uint   `gorm:"primaryKey" json:"id"`
	GroupID  uint   `gorm:"index;not null" json:"group_id"`
	DeviceID string `gorm:"size:64;not null" json:"device_id"`
}

// BatchResult 批量命令执行结果
type BatchResult struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	BatchID   string    `gorm:"uniqueIndex;size:64;not null" json:"batch_id"`
	NodeID    string    `gorm:"size:64;not null" json:"node_id"`
	Command   string    `gorm:"size:1024" json:"command"`
	Stdout    string    `gorm:"type:text" json:"stdout"`
	Stderr    string    `gorm:"type:text" json:"stderr"`
	ExitCode  int       `json:"exit_code"`
	Status    string    `gorm:"size:32;default:pending" json:"status"` // pending/running/completed/failed
	StartedAt time.Time `json:"started_at"`
	CreatedAt time.Time `json:"created_at"`
}

// AutoMigrate 自动迁移
func AutoMigrate(db *gorm.DB) error {
	return db.AutoMigrate(
		&User{}, &SSHProfile{}, &Node{}, &SerialPort{}, &Session{}, &AuditLog{},
		&Script{}, &ScriptResult{}, &Device{},
		&DeviceAlias{}, &RemoteCenter{}, &DeviceGroup{}, &DeviceGroupMember{}, &BatchResult{},
	)
}
