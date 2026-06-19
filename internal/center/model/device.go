// Package model provides database models for the HubTerm center service.
package model

import (
	"time"
)

// Device represents a managed device (AP, server, switch, etc.) that AI can discover and command.
//
// Fields:
//   - ID: Auto-increment primary key.
//   - DeviceID: Unique string identifier for the device (e.g., "ap-03").
//   - Name: Human-readable display name.
//   - Type: Device type — ap / server / station / switch / router.
//   - IP: Management IP address.
//   - NodeID: The agent node that manages this device (via serial/SSH).
//   - Protocol: Connection protocol — serial / ssh / telnet.
//   - PortName: Serial port name or SSH port number.
//   - Status: online / offline / busy.
//   - Capabilities: JSON array of capability strings (e.g., ["console","ping","snmp"]).
//   - Location: Physical location description.
//   - Tags: JSON array of tag strings (e.g., ["production","critical"]).
//   - Credentials: Encrypted credentials (never exposed in API responses).
//   - LastSeen: Timestamp of last successful contact.
type Device struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	DeviceID     string    `gorm:"uniqueIndex;size:64;not null" json:"device_id"`
	Name         string    `gorm:"size:128" json:"name"`
	Type         string    `gorm:"size:32" json:"type"` // ap / server / station / switch / router
	IP           string    `gorm:"size:64" json:"ip"`
	NodeID       string    `gorm:"index;size:64" json:"node_id"` // 所在节点
	Protocol     string    `gorm:"size:32" json:"protocol"`      // serial / ssh / telnet
	PortName     string    `gorm:"size:64" json:"port_name"`     // 串口名或SSH端口
	Status       string    `gorm:"size:32;default:offline" json:"status"`
	Capabilities string    `gorm:"size:1024" json:"capabilities"` // JSON 数组
	Location     string    `gorm:"size:256" json:"location"`
	Tags         string    `gorm:"size:512" json:"tags"` // JSON 数组
	Credentials  string    `gorm:"size:1024" json:"-"`   // 加密凭证，不暴露给 API
	LastSeen     time.Time `json:"last_seen"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}
