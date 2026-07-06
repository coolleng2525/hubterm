// Package service provides business logic services for the HubTerm center.
package service

import (
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/coolleng2525/hubterm/internal/center/model"
)

func setupDeviceTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("failed to open in-memory db: %v", err)
	}
	if err := model.AutoMigrate(db); err != nil {
		t.Fatalf("failed to migrate: %v", err)
	}
	return db
}

func TestDeviceServiceDiscover(t *testing.T) {
	db := setupDeviceTestDB(t)
	svc := NewDeviceService(db)

	// Seed devices.
	now := time.Now()
	devices := []model.Device{
		{DeviceID: "ap-01", Name: "AP-01", Type: "ap", IP: "10.0.0.1", Status: "online",
			Capabilities: `["console","ping"]`, Tags: `["prod"]`, Location: "DC-A", LastSeen: now},
		{DeviceID: "svr-01", Name: "Server-01", Type: "server", IP: "10.0.0.10", Status: "online",
			Capabilities: `["console","ssh"]`, Tags: `["prod","db"]`, Location: "DC-B", LastSeen: now.Add(-time.Hour)},
		{DeviceID: "sw-01", Name: "Switch-01", Type: "switch", IP: "10.0.0.254", Status: "offline",
			Capabilities: `["console"]`, Tags: `[]`, Location: "DC-A", LastSeen: now.Add(-2 * time.Hour)},
	}
	for _, d := range devices {
		if err := db.Create(&d).Error; err != nil {
			t.Fatalf("failed to seed device: %v", err)
		}
	}

	t.Run("Discover returns only online devices", func(t *testing.T) {
		result := svc.Discover()
		if len(result) != 2 {
			t.Errorf("expected 2 online devices, got %d", len(result))
		}
	})

	t.Run("DiscoverAll returns all devices", func(t *testing.T) {
		result := svc.DiscoverAll()
		if len(result) != 3 {
			t.Errorf("expected 3 devices, got %d", len(result))
		}
	})

	t.Run("DeviceInfo has parsed capabilities", func(t *testing.T) {
		result := svc.Discover()
		if len(result) > 0 {
			d := result[0]
			if len(d.Capabilities) == 0 {
				t.Error("expected non-empty capabilities")
			}
			if d.LastSeen == "" {
				t.Error("expected non-empty last_seen")
			}
		}
	})

	t.Run("DeviceInfo has protocols derived from device", func(t *testing.T) {
		result := svc.Discover()
		for _, d := range result {
			if len(d.Protocols) == 0 {
				t.Errorf("device %s has no protocols", d.ID)
			}
		}
	})
}

func TestDeviceServiceDiscoverIncludesActiveSessions(t *testing.T) {
	db := setupDeviceTestDB(t)
	svc := NewDeviceService(db)
	connectedAt := time.Now()

	if err := db.Create(&model.Session{
		SessionID:   "sess-com9",
		NodeID:      "node-20",
		DisplayName: "com9-r770",
		PortName:    "/dev/ttyS9",
		User:        "admin",
		Type:        "master",
		ConnectedAt: connectedAt,
	}).Error; err != nil {
		t.Fatalf("failed to seed session: %v", err)
	}

	result := svc.Discover()
	if len(result) != 1 {
		t.Fatalf("expected 1 discovered session device, got %d", len(result))
	}
	got := result[0]
	if got.ID != "com9-r770" {
		t.Fatalf("expected session display name as device id, got %q", got.ID)
	}
	if got.Source != "session" || got.SessionID != "sess-com9" || got.NodeID != "node-20" {
		t.Fatalf("unexpected session device metadata: %+v", got)
	}
	if got.Status != "online" {
		t.Fatalf("expected online session device, got %q", got.Status)
	}
	if len(got.Capabilities) == 0 || got.Capabilities[0] != "terminal_input" {
		t.Fatalf("expected terminal_input capability, got %v", got.Capabilities)
	}
}

func TestDeviceServiceGetDevice(t *testing.T) {
	db := setupDeviceTestDB(t)
	svc := NewDeviceService(db)

	db.Create(&model.Device{
		DeviceID: "ap-02", Name: "AP-02", Type: "ap", IP: "10.0.0.2",
		Status: "online", Capabilities: `["console"]`,
	})

	t.Run("GetDevice returns device info", func(t *testing.T) {
		info, err := svc.GetDevice("ap-02")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if info.ID != "ap-02" {
			t.Errorf("expected ID=ap-02, got %s", info.ID)
		}
		if info.Name != "AP-02" {
			t.Errorf("expected Name=AP-02, got %s", info.Name)
		}
		if info.Status != "online" {
			t.Errorf("expected Status=online, got %s", info.Status)
		}
	})

	t.Run("GetDevice returns error for nonexistent device", func(t *testing.T) {
		_, err := svc.GetDevice("nonexistent")
		if err == nil {
			t.Fatal("expected error for nonexistent device")
		}
	})
}

func TestDeviceServiceGetCapabilities(t *testing.T) {
	db := setupDeviceTestDB(t)
	svc := NewDeviceService(db)

	db.Create(&model.Device{
		DeviceID:     "ap-03",
		Name:         "AP-03",
		Type:         "ap",
		Status:       "online",
		Capabilities: `["console","ping","snmp"]`,
		Protocol:     "ssh",
	})

	t.Run("GetCapabilities returns device info with capabilities", func(t *testing.T) {
		info, err := svc.GetCapabilities("ap-03")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(info.Capabilities) != 3 {
			t.Errorf("expected 3 capabilities, got %d: %v", len(info.Capabilities), info.Capabilities)
		}
		if len(info.Protocols) == 0 {
			t.Error("expected non-empty protocols")
		}
	})
}

func TestDeviceServiceExecute(t *testing.T) {
	db := setupDeviceTestDB(t)
	svc := NewDeviceService(db)

	t.Run("Execute on device with no node returns error", func(t *testing.T) {
		db.Create(&model.Device{
			DeviceID: "ap-orphan",
			Name:     "Orphan",
			Status:   "online",
			NodeID:   "",
		})

		_, err := svc.Execute("ap-orphan", "show log", 30, func(nodeID, command string, timeout int) (string, error) {
			return "cmd-1", nil
		})
		if err == nil {
			t.Error("expected error for device with no node")
		}
	})

	t.Run("Execute on offline device returns error", func(t *testing.T) {
		db.Create(&model.Device{
			DeviceID: "ap-offline",
			Name:     "Offline-AP",
			Status:   "offline",
			NodeID:   "node-01",
		})

		_, err := svc.Execute("ap-offline", "show log", 30, func(nodeID, command string, timeout int) (string, error) {
			return "cmd-2", nil
		})
		if err == nil {
			t.Error("expected error for offline device")
		}
	})

	t.Run("Execute on nonexistent device returns error", func(t *testing.T) {
		_, err := svc.Execute("nonexistent", "ls", 30, func(nodeID, command string, timeout int) (string, error) {
			return "cmd-3", nil
		})
		if err == nil {
			t.Error("expected error for nonexistent device")
		}
	})

	t.Run("Execute on online device with node succeeds", func(t *testing.T) {
		db.Create(&model.Device{
			DeviceID: "ap-online",
			Name:     "Online-AP",
			Status:   "online",
			NodeID:   "node-01",
			Protocol: "serial",
			PortName: "/dev/ttyS0",
		})

		cmdID, err := svc.Execute("ap-online", "show version", 30, func(nodeID, command string, timeout int) (string, error) {
			if nodeID != "node-01" {
				t.Errorf("expected nodeID=node-01, got %s", nodeID)
			}
			return "cmd-exec-1", nil
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cmdID != "cmd-exec-1" {
			t.Errorf("expected cmdID=cmd-exec-1, got %s", cmdID)
		}
	})
}

func TestDeviceServiceToDeviceInfo(t *testing.T) {
	db := setupDeviceTestDB(t)
	svc := NewDeviceService(db)

	t.Run("empty capabilities and tags produce empty slices", func(t *testing.T) {
		d := model.Device{
			DeviceID: "test-device",
			Name:     "Test",
			Type:     "ap",
			Status:   "online",
		}
		info := svc.toDeviceInfo(d)
		if info.Capabilities == nil {
			t.Error("expected non-nil capabilities")
		}
		if len(info.Capabilities) != 0 {
			t.Errorf("expected 0 capabilities, got %d", len(info.Capabilities))
		}
		if info.Tags != nil && len(info.Tags) != 0 {
			t.Errorf("expected empty tags, got %v", info.Tags)
		}
	})

	t.Run("malformed JSON capabilities returns empty slice", func(t *testing.T) {
		d := model.Device{
			DeviceID:     "test-device-2",
			Name:         "Test2",
			Type:         "server",
			Status:       "online",
			Capabilities: "not-valid-json",
		}
		info := svc.toDeviceInfo(d)
		if info.Capabilities == nil {
			t.Error("expected non-nil capabilities")
		}
		if len(info.Capabilities) != 0 {
			t.Errorf("expected 0 capabilities for malformed JSON, got %d", len(info.Capabilities))
		}
	})
}
