package model

import (
	"time"

	"gorm.io/gorm"
)

// User 用户
type User struct {
	ID           uint   `gorm:"primaryKey" json:"id"`
	Username     string `gorm:"uniqueIndex;size:64;not null" json:"username"`
	PasswordHash string `gorm:"size:256;not null" json:"-"`
	Role         string `gorm:"size:32;not null;default:operator" json:"role"` // admin/operator/readonly
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// Node 节点
type Node struct {
	ID            uint      `gorm:"primaryKey" json:"id"`
	NodeID        string    `gorm:"uniqueIndex;size:64;not null" json:"node_id"`
	Name          string    `gorm:"size:128" json:"name"`
	IP            string    `gorm:"size:64" json:"ip"`
	Hostname      string    `gorm:"size:128" json:"hostname"`
	OS            string    `gorm:"size:64" json:"os"`
	OSVersion     string    `gorm:"size:128" json:"os_version"`
	Arch          string    `gorm:"size:32" json:"arch"`
	Status        string    `gorm:"size:32;not null;default:offline" json:"status"` // online/offline
	CPUPercent    float64   `json:"cpu_percent"`
	MemoryTotal   uint64    `json:"memory_total"`
	MemoryUsed    uint64    `json:"memory_used"`
	MemoryPercent float64   `json:"memory_percent"`
	DiskTotal     uint64    `json:"disk_total"`
	DiskUsed      uint64    `json:"disk_used"`
	LastSeen      time.Time `json:"last_seen"`
	Token         string    `gorm:"size:128" json:"-"` // FIXED: node auth token
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// SerialPort 串口
type SerialPort struct {
	ID              uint      `gorm:"primaryKey" json:"id"`
	NodeID          string    `gorm:"index;size:64;not null" json:"node_id"`
	PortName        string    `gorm:"size:64;not null" json:"port_name"`
	Description     string    `gorm:"size:256" json:"description"`
	Status          string    `gorm:"size:32;not null;default:offline" json:"status"` // online/offline/busy
	BaudRate        int       `json:"baud_rate"`
	CurrentSessionID string   `gorm:"size:64" json:"current_session_id"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
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

// AutoMigrate 自动迁移
func AutoMigrate(db *gorm.DB) error {
	return db.AutoMigrate(&User{}, &Node{}, &SerialPort{}, &Session{}, &AuditLog{})
}
