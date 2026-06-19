// Package handler provides HTTP and WebSocket handlers for the HubTerm center service.
package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/coolleng2525/hubterm/internal/center/model"
	"github.com/coolleng2525/hubterm/internal/center/service"
)

func TestDiscoverDevices(t *testing.T) {
	os.Setenv("JWT_SECRET", "test-secret-key-for-testing")
	defer os.Unsetenv("JWT_SECRET")

	db := setupTestDB(t)
	deviceSvc := service.NewDeviceService(db)
	agentWSH := NewAgentWSHandler(db)
	aiH := NewAIHandler(db, deviceSvc, agentWSH)

	// Seed devices.
	devices := []model.Device{
		{DeviceID: "ap-01", Name: "AP-01", Type: "ap", IP: "192.168.1.101", Status: "online",
			Capabilities: `["console","ping","snmp"]`, Tags: `["production","critical"]`, Location: "机房A-3F"},
		{DeviceID: "server-db", Name: "DB-Server", Type: "server", IP: "192.168.1.50", Status: "online",
			Capabilities: `["console","ssh","snmp"]`, Tags: `["production"]`, Location: "机房B-2F"},
		{DeviceID: "switch-01", Name: "Core-Switch", Type: "switch", IP: "192.168.1.1", Status: "offline",
			Capabilities: `["console","snmp"]`, Tags: `["production","critical"]`},
	}
	for _, d := range devices {
		db.Create(&d)
	}

	t.Run("discover returns only online devices", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("GET", "/api/v1/devices", nil)

		aiH.Discover(c)

		if w.Code != http.StatusOK {
			t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
		}

		var resp struct {
			Devices []service.DeviceInfo `json:"devices"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("failed to parse response: %v", err)
		}

		if len(resp.Devices) != 2 {
			t.Errorf("expected 2 online devices, got %d", len(resp.Devices))
		}

		// Verify first device has parsed capabilities.
		if len(resp.Devices) > 0 {
			d := resp.Devices[0]
			if d.ID == "" {
				t.Error("expected non-empty device ID")
			}
			if len(d.Capabilities) == 0 {
				t.Error("expected non-empty capabilities")
			}
			if d.LastSeen == "" {
				t.Error("expected non-empty last_seen")
			}
		}
	})

	t.Run("discover with no devices returns empty array", func(t *testing.T) {
		// Create a fresh handler with empty DB.
		emptyDB := setupTestDB(t)
		emptySvc := service.NewDeviceService(emptyDB)
		emptyAI := NewAIHandler(emptyDB, emptySvc, agentWSH)

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("GET", "/api/v1/devices", nil)

		emptyAI.Discover(c)

		if w.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", w.Code)
		}

		var resp struct {
			Devices []service.DeviceInfo `json:"devices"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("failed to parse response: %v", err)
		}

		if resp.Devices == nil {
			t.Error("expected empty array, not null")
		}
		if len(resp.Devices) != 0 {
			t.Errorf("expected 0 devices, got %d", len(resp.Devices))
		}
	})
}

func TestGetDevice(t *testing.T) {
	os.Setenv("JWT_SECRET", "test-secret-key-for-testing")
	defer os.Unsetenv("JWT_SECRET")

	db := setupTestDB(t)
	deviceSvc := service.NewDeviceService(db)
	agentWSH := NewAgentWSHandler(db)
	aiH := NewAIHandler(db, deviceSvc, agentWSH)

	// Seed a device.
	db.Create(&model.Device{
		DeviceID:     "ap-03",
		Name:         "AP-03",
		Type:         "ap",
		IP:           "192.168.1.103",
		Status:       "online",
		Protocol:     "serial",
		PortName:     "/dev/ttyUSB0",
		Capabilities: `["console","ping"]`,
		Location:     "机房A-3F",
		Tags:         `["production"]`,
	})

	t.Run("get existing device", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("GET", "/api/v1/devices/ap-03", nil)
		c.Params = []gin.Param{{Key: "id", Value: "ap-03"}}

		aiH.GetDevice(c)

		if w.Code != http.StatusOK {
			t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
		}

		var info service.DeviceInfo
		if err := json.Unmarshal(w.Body.Bytes(), &info); err != nil {
			t.Fatalf("failed to parse response: %v", err)
		}

		if info.ID != "ap-03" {
			t.Errorf("expected ID=ap-03, got %s", info.ID)
		}
		if info.Name != "AP-03" {
			t.Errorf("expected Name=AP-03, got %s", info.Name)
		}
		if info.Type != "ap" {
			t.Errorf("expected Type=ap, got %s", info.Type)
		}
		if info.Status != "online" {
			t.Errorf("expected Status=online, got %s", info.Status)
		}
		if len(info.Protocols) == 0 {
			t.Error("expected non-empty protocols")
		}
		if len(info.Capabilities) == 0 {
			t.Error("expected non-empty capabilities")
		}
	})

	t.Run("get nonexistent device returns 404", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("GET", "/api/v1/devices/nonexistent", nil)
		c.Params = []gin.Param{{Key: "id", Value: "nonexistent"}}

		aiH.GetDevice(c)

		if w.Code != http.StatusNotFound {
			t.Errorf("expected 404, got %d", w.Code)
		}
	})
}

func TestGetCapabilities(t *testing.T) {
	os.Setenv("JWT_SECRET", "test-secret-key-for-testing")
	defer os.Unsetenv("JWT_SECRET")

	db := setupTestDB(t)
	deviceSvc := service.NewDeviceService(db)
	agentWSH := NewAgentWSHandler(db)
	aiH := NewAIHandler(db, deviceSvc, agentWSH)

	// Seed a device with capabilities.
	db.Create(&model.Device{
		DeviceID:     "ap-04",
		Name:         "AP-04",
		Type:         "ap",
		IP:           "192.168.1.104",
		Status:       "online",
		Protocol:     "ssh",
		PortName:     "22",
		Capabilities: `["console","ping","snmp","traceroute"]`,
		Location:     "机房B-1F",
	})

	t.Run("get capabilities for existing device", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("GET", "/api/v1/devices/ap-04/capabilities", nil)
		c.Params = []gin.Param{{Key: "id", Value: "ap-04"}}

		aiH.GetCapabilities(c)

		if w.Code != http.StatusOK {
			t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
		}

		var resp struct {
			DeviceID     string   `json:"device_id"`
			Name         string   `json:"name"`
			Type         string   `json:"type"`
			Capabilities []string `json:"capabilities"`
			Protocols    []string `json:"protocols"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("failed to parse response: %v", err)
		}

		if resp.DeviceID != "ap-04" {
			t.Errorf("expected device_id=ap-04, got %s", resp.DeviceID)
		}
		if len(resp.Capabilities) != 4 {
			t.Errorf("expected 4 capabilities, got %d: %v", len(resp.Capabilities), resp.Capabilities)
		}
		if len(resp.Protocols) == 0 {
			t.Error("expected non-empty protocols")
		}
	})

	t.Run("get capabilities for nonexistent device returns 404", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("GET", "/api/v1/devices/nonexistent/capabilities", nil)
		c.Params = []gin.Param{{Key: "id", Value: "nonexistent"}}

		aiH.GetCapabilities(c)

		if w.Code != http.StatusNotFound {
			t.Errorf("expected 404, got %d", w.Code)
		}
	})
}

func TestExecuteCommand(t *testing.T) {
	os.Setenv("JWT_SECRET", "test-secret-key-for-testing")
	defer os.Unsetenv("JWT_SECRET")

	db := setupTestDB(t)
	deviceSvc := service.NewDeviceService(db)
	agentWSH := NewAgentWSHandler(db)
	aiH := NewAIHandler(db, deviceSvc, agentWSH)

	// Seed a device with no managing node — should fail with appropriate error.
	db.Create(&model.Device{
		DeviceID: "ap-orphan",
		Name:     "Orphan-AP",
		Type:     "ap",
		IP:       "192.168.1.200",
		Status:   "online",
		NodeID:   "", // no managing node
	})

	t.Run("execute on device with no node returns error", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("POST", "/api/v1/devices/ap-orphan/exec",
			strings.NewReader(`{"command": "show log", "timeout": 30}`))
		c.Request.Header.Set("Content-Type", "application/json")
		c.Params = []gin.Param{{Key: "id", Value: "ap-orphan"}}

		aiH.Execute(c)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected 400, got %d: %s", w.Code, w.Body.String())
		}

		var resp map[string]string
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("failed to parse response: %v", err)
		}
		if resp["error"] == "" {
			t.Error("expected non-empty error message")
		}
	})

	t.Run("execute on nonexistent device returns 404", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("POST", "/api/v1/devices/nonexistent/exec",
			strings.NewReader(`{"command": "ls -la", "timeout": 10}`))
		c.Request.Header.Set("Content-Type", "application/json")
		c.Params = []gin.Param{{Key: "id", Value: "nonexistent"}}

		aiH.Execute(c)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected 400, got %d: %s", w.Code, w.Body.String())
		}
	})

	t.Run("execute with missing command returns 400", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("POST", "/api/v1/devices/ap-orphan/exec",
			strings.NewReader(`{}`))
		c.Request.Header.Set("Content-Type", "application/json")
		c.Params = []gin.Param{{Key: "id", Value: "ap-orphan"}}

		aiH.Execute(c)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected 400, got %d: %s", w.Code, w.Body.String())
		}
	})
}

func TestGetResult(t *testing.T) {
	os.Setenv("JWT_SECRET", "test-secret-key-for-testing")
	defer os.Unsetenv("JWT_SECRET")

	db := setupTestDB(t)
	deviceSvc := service.NewDeviceService(db)
	agentWSH := NewAgentWSHandler(db)
	aiH := NewAIHandler(db, deviceSvc, agentWSH)

	t.Run("get result for nonexistent cmd_id returns 404", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("GET", "/api/v1/devices/ap-01/exec/nonexistent-cmd", nil)
		c.Params = []gin.Param{
			{Key: "id", Value: "ap-01"},
			{Key: "cmd_id", Value: "nonexistent-cmd"},
		}

		aiH.GetResult(c)

		if w.Code != http.StatusNotFound {
			t.Errorf("expected 404, got %d: %s", w.Code, w.Body.String())
		}
	})
}

func TestUploadAndExecute(t *testing.T) {
	os.Setenv("JWT_SECRET", "test-secret-key-for-testing")
	defer os.Unsetenv("JWT_SECRET")

	db := setupTestDB(t)
	deviceSvc := service.NewDeviceService(db)
	agentWSH := NewAgentWSHandler(db)
	aiH := NewAIHandler(db, deviceSvc, agentWSH)

	t.Run("upload script with invalid target returns error", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		body := `{
			"name": "ping-test",
			"source": "print('hello')",
			"targets": ["nonexistent-target"]
		}`
		c.Request = httptest.NewRequest("POST", "/api/v1/scripts", strings.NewReader(body))
		c.Request.Header.Set("Content-Type", "application/json")

		aiH.UploadAndExecute(c)

		if w.Code != http.StatusCreated {
			t.Errorf("expected 201, got %d: %s", w.Code, w.Body.String())
		}

		var resp struct {
			ScriptID string `json:"script_id"`
			Results  []struct {
				Target string `json:"target"`
				Status string `json:"status"`
				Error  string `json:"error,omitempty"`
			} `json:"results"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("failed to parse response: %v", err)
		}

		if resp.ScriptID == "" {
			t.Error("expected non-empty script_id")
		}
		if len(resp.Results) != 1 {
			t.Errorf("expected 1 result, got %d", len(resp.Results))
		}
		if len(resp.Results) > 0 {
			if resp.Results[0].Status != "failed" {
				t.Errorf("expected status=failed, got %s", resp.Results[0].Status)
			}
			if resp.Results[0].Error == "" {
				t.Error("expected non-empty error message")
			}
		}
	})

	t.Run("upload script with missing name returns 400", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		body := `{
			"source": "print('hello')",
			"targets": ["ap-01"]
		}`
		c.Request = httptest.NewRequest("POST", "/api/v1/scripts", strings.NewReader(body))
		c.Request.Header.Set("Content-Type", "application/json")

		aiH.UploadAndExecute(c)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected 400, got %d: %s", w.Code, w.Body.String())
		}
	})

	t.Run("upload script with empty targets returns 400", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		body := `{
			"name": "test",
			"source": "print('hello')",
			"targets": []
		}`
		c.Request = httptest.NewRequest("POST", "/api/v1/scripts", strings.NewReader(body))
		c.Request.Header.Set("Content-Type", "application/json")

		aiH.UploadAndExecute(c)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected 400, got %d: %s", w.Code, w.Body.String())
		}
	})
}
