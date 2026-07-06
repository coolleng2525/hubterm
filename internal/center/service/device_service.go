// Package service provides business logic services for the HubTerm center.
package service

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"

	"gorm.io/gorm"

	"github.com/coolleng2525/hubterm/internal/center/model"
	"github.com/coolleng2525/hubterm/internal/pkg/log"
	hubtermproto "github.com/coolleng2525/hubterm/internal/proto"
)

// DeviceService manages device discovery, capability queries, and command execution
// for AI-friendly device management.
type DeviceService struct {
	DB *gorm.DB
}

// NewDeviceService creates a new DeviceService with the given database connection.
func NewDeviceService(db *gorm.DB) *DeviceService {
	return &DeviceService{DB: db}
}

// DeviceInfo is the AI-friendly view of a device, with parsed arrays instead of JSON strings.
type DeviceInfo struct {
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	Type         string   `json:"type"`
	IP           string   `json:"ip"`
	NodeID       string   `json:"node_id,omitempty"`
	PortName     string   `json:"port_name,omitempty"`
	SessionID    string   `json:"session_id,omitempty"`
	Source       string   `json:"source,omitempty"`
	Status       string   `json:"status"`
	Protocols    []string `json:"protocols"`
	Capabilities []string `json:"capabilities"`
	Location     string   `json:"location,omitempty"`
	Tags         []string `json:"tags,omitempty"`
	LastSeen     string   `json:"last_seen"`
}

// Discover returns all online devices with their capabilities.
// Devices are sorted by last_seen descending.
func (s *DeviceService) Discover() []DeviceInfo {
	var devices []model.Device
	s.DB.Where("status = ?", "online").Order("last_seen desc").Find(&devices)

	result := make([]DeviceInfo, 0, len(devices))
	seen := make(map[string]bool)
	for _, d := range devices {
		info := s.toDeviceInfo(d)
		result = append(result, info)
		seen[info.ID] = true
	}
	result = append(result, s.discoverActiveSessions(seen)...)
	sort.SliceStable(result, func(i, j int) bool {
		return result[i].LastSeen > result[j].LastSeen
	})
	return result
}

// DiscoverAll returns all devices regardless of status.
func (s *DeviceService) DiscoverAll() []DeviceInfo {
	var devices []model.Device
	s.DB.Order("last_seen desc").Find(&devices)

	result := make([]DeviceInfo, 0, len(devices))
	for _, d := range devices {
		result = append(result, s.toDeviceInfo(d))
	}
	return result
}

// GetDevice returns a single device by its device_id.
func (s *DeviceService) GetDevice(deviceID string) (*DeviceInfo, error) {
	var d model.Device
	if err := s.DB.Where("device_id = ?", deviceID).First(&d).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("device not found: %s", deviceID)
		}
		return nil, fmt.Errorf("query device %s: %w", deviceID, err)
	}
	info := s.toDeviceInfo(d)
	return &info, nil
}

// GetCapabilities returns the capabilities of a specific device.
func (s *DeviceService) GetCapabilities(deviceID string) (*DeviceInfo, error) {
	return s.GetDevice(deviceID)
}

// Execute sends a command to a device by routing through its managing node.
// It returns a command ID that can be used to poll for results.
// The executor parameter is a function that sends the command to the node
// (injected to avoid circular dependency with handler package).
func (s *DeviceService) Execute(deviceID, command string, timeout int, executor func(nodeID, command string, timeout int) (string, error)) (string, error) {
	var d model.Device
	if err := s.DB.Where("device_id = ?", deviceID).First(&d).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return "", fmt.Errorf("device not found: %s", deviceID)
		}
		return "", fmt.Errorf("query device %s: %w", deviceID, err)
	}

	if d.NodeID == "" {
		return "", fmt.Errorf("device %s has no managing node", deviceID)
	}

	if d.Status != "online" {
		return "", fmt.Errorf("device %s is not online (status: %s)", deviceID, d.Status)
	}

	// Build the full command: prepend protocol-specific connection if needed.
	fullCommand := command
	if d.Protocol == "serial" && d.PortName != "" {
		// For serial devices, wrap command with serial port context.
		fullCommand = fmt.Sprintf("serial:%s %s", d.PortName, command)
	} else if d.Protocol == "ssh" && d.PortName != "" {
		// For SSH devices, wrap with SSH target context.
		fullCommand = fmt.Sprintf("ssh:%s %s", d.PortName, command)
	}

	cmdID, err := executor(d.NodeID, fullCommand, timeout)
	if err != nil {
		return "", fmt.Errorf("execute on node %s: %w", d.NodeID, err)
	}

	svcLog.Info("device command executed",
		log.String("device_id", deviceID),
		log.String("node_id", d.NodeID),
		log.String("cmd_id", cmdID),
	)

	return cmdID, nil
}

// GetResult retrieves the execution result for a command.
// It delegates to the provided getter function to avoid circular dependencies.
func (s *DeviceService) GetResult(cmdID string, getter func(string) *struct {
	CmdID  string
	NodeID string
	Status string
	Result *hubtermproto.ExecResult
}) *hubtermproto.ExecResult {
	entry := getter(cmdID)
	if entry == nil {
		return nil
	}
	return entry.Result
}

// toDeviceInfo converts a model.Device to an AI-friendly DeviceInfo with parsed arrays.
func (s *DeviceService) toDeviceInfo(d model.Device) DeviceInfo {
	info := DeviceInfo{
		ID:       d.DeviceID,
		Name:     d.Name,
		Type:     d.Type,
		IP:       d.IP,
		NodeID:   d.NodeID,
		PortName: d.PortName,
		Source:   "device",
		Status:   d.Status,
		Location: d.Location,
		LastSeen: d.LastSeen.Format(time.RFC3339),
	}

	// Parse capabilities JSON array.
	if d.Capabilities != "" {
		var caps []string
		if err := json.Unmarshal([]byte(d.Capabilities), &caps); err == nil {
			info.Capabilities = caps
		}
	}
	if info.Capabilities == nil {
		info.Capabilities = []string{}
	}

	// Parse tags JSON array.
	if d.Tags != "" {
		var tags []string
		if err := json.Unmarshal([]byte(d.Tags), &tags); err == nil {
			info.Tags = tags
		}
	}

	// Derive protocols from device type and protocol field.
	info.Protocols = s.deriveProtocols(d)
	if info.Protocols == nil {
		info.Protocols = []string{}
	}

	return info
}

func (s *DeviceService) discoverActiveSessions(seen map[string]bool) []DeviceInfo {
	var sessions []model.Session
	if err := s.DB.Order("connected_at desc").Find(&sessions).Error; err != nil {
		return nil
	}

	result := make([]DeviceInfo, 0, len(sessions))
	for _, sess := range sessions {
		name := strings.TrimSpace(sess.DisplayName)
		if name == "" {
			name = strings.TrimSpace(sess.PortName)
		}
		if name == "" {
			name = sess.SessionID
		}
		id := sessionDeviceID(sess)
		if id == "" {
			id = "session-" + sess.SessionID
		}
		if seen[id] {
			id = fmt.Sprintf("%s-%s", id, shortID(sess.SessionID))
		}
		seen[id] = true

		result = append(result, DeviceInfo{
			ID:           id,
			Name:         name,
			Type:         sessionDeviceType(sess),
			NodeID:       sess.NodeID,
			PortName:     sess.PortName,
			SessionID:    sess.SessionID,
			Source:       "session",
			Status:       "online",
			Protocols:    []string{"terminal", "console"},
			Capabilities: []string{"terminal_input", "interactive_console"},
			Tags:         []string{"session", sess.Type},
			LastSeen:     sess.ConnectedAt.Format(time.RFC3339),
		})
	}
	return result
}

var unsafeDeviceIDChars = regexp.MustCompile(`[^A-Za-z0-9_.-]+`)

func sessionDeviceID(sess model.Session) string {
	base := strings.TrimSpace(sess.DisplayName)
	if base == "" {
		base = strings.TrimSpace(sess.PortName)
	}
	base = unsafeDeviceIDChars.ReplaceAllString(base, "-")
	base = strings.Trim(base, "-_.")
	if base == "" {
		return ""
	}
	return base
}

func shortID(id string) string {
	if len(id) <= 8 {
		return id
	}
	return id[:8]
}

func sessionDeviceType(sess model.Session) string {
	portName := strings.ToLower(strings.TrimSpace(sess.PortName))
	displayName := strings.ToLower(strings.TrimSpace(sess.DisplayName))
	if strings.Contains(portName, ":") || strings.Contains(displayName, "ssh") {
		return "ssh"
	}
	return "terminal"
}

// deriveProtocols returns the list of supported protocols for a device.
func (s *DeviceService) deriveProtocols(d model.Device) []string {
	protocols := make([]string, 0, 2)
	if d.Protocol != "" {
		protocols = append(protocols, d.Protocol)
	}
	// All devices support console access via their managing node.
	protocols = append(protocols, "console")
	return protocols
}
