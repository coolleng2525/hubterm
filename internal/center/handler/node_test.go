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
)

func TestNodeReport(t *testing.T) {
	os.Setenv("JWT_SECRET", "test-secret-key-for-testing")
	defer os.Unsetenv("JWT_SECRET")

	db := setupTestDB(t)
	handler := &NodeHandler{DB: db}

	t.Run("new node registers and gets token", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		body := `{
			"node_id": "node-001",
			"name": "test-node",
			"ip": "192.168.1.100",
			"hostname": "testhost",
			"os": "linux",
			"os_version": "Ubuntu 22.04",
			"arch": "amd64",
			"cpu_percent": 45.5,
			"memory_total": 8589934592,
			"memory_used": 4294967296,
			"memory_percent": 50.0,
			"disk_total": 107374182400,
			"disk_used": 53687091200,
			"serial_ports": [],
			"sessions": []
		}`
		c.Request = httptest.NewRequest("POST", "/api/nodes/report", strings.NewReader(body))
		c.Request.Header.Set("Content-Type", "application/json")

		handler.Report(c)

		if w.Code != http.StatusOK {
			t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
		}

		var resp map[string]interface{}
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("failed to parse response: %v", err)
		}
		if resp["success"] != true {
			t.Error("expected success=true")
		}
		token, ok := resp["token"].(string)
		if !ok || token == "" {
			t.Error("expected non-empty token for new node")
		}

		// Verify node was saved
		var node model.Node
		if err := db.Where("node_id = ?", "node-001").First(&node).Error; err != nil {
			t.Fatalf("node not found in db: %v", err)
		}
		if node.Token != token {
			t.Errorf("token mismatch: %s vs %s", node.Token, token)
		}
		if node.Status != "online" {
			t.Errorf("expected status=online, got %s", node.Status)
		}
	})

	t.Run("existing node updates", func(t *testing.T) {
		// Pre-create a node
		db.Create(&model.Node{
			NodeID: "node-002",
			Name:   "old-name",
			Token:  "existing-token",
			Status: "offline",
		})

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		body := `{
			"node_id": "node-002",
			"name": "new-name",
			"ip": "192.168.1.101",
			"hostname": "newhost",
			"os": "linux",
			"os_version": "Ubuntu 24.04",
			"arch": "amd64",
			"cpu_percent": 30.0,
			"memory_total": 8589934592,
			"memory_used": 2147483648,
			"memory_percent": 25.0,
			"disk_total": 107374182400,
			"disk_used": 26843545600,
			"serial_ports": [],
			"sessions": []
		}`
		c.Request = httptest.NewRequest("POST", "/api/nodes/report", strings.NewReader(body))
		c.Request.Header.Set("Content-Type", "application/json")

		handler.Report(c)

		if w.Code != http.StatusOK {
			t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
		}

		var node model.Node
		if err := db.Where("node_id = ?", "node-002").First(&node).Error; err != nil {
			t.Fatalf("node not found: %v", err)
		}
		if node.Name != "new-name" {
			t.Errorf("expected name=new-name, got %s", node.Name)
		}
		if node.Status != "online" {
			t.Errorf("expected status=online, got %s", node.Status)
		}
		// Token should remain unchanged for existing node
		if node.Token != "existing-token" {
			t.Errorf("expected token to remain existing-token, got %s", node.Token)
		}
	})

	t.Run("report with serial ports and sessions", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		body := `{
			"node_id": "node-003",
			"name": "multi-port-node",
			"ip": "10.0.0.1",
			"hostname": "multihost",
			"os": "linux",
			"os_version": "Debian 12",
			"arch": "arm64",
			"cpu_percent": 10.0,
			"memory_total": 4294967296,
			"memory_used": 1073741824,
			"memory_percent": 25.0,
			"disk_total": 53687091200,
			"disk_used": 13421772800,
			"serial_ports": [
				{"port_name": "/dev/ttyUSB0", "description": "USB-Serial", "status": "online", "baud_rate": 115200},
				{"port_name": "/dev/ttyUSB1", "description": "GPS Module", "status": "busy", "baud_rate": 9600}
			],
			"sessions": [
				{"session_id": "sess-001", "port_name": "/dev/ttyUSB0", "user": "admin", "type": "master", "client_ip": "10.0.0.2", "connected_at": 1700000000}
			]
		}`
		c.Request = httptest.NewRequest("POST", "/api/nodes/report", strings.NewReader(body))
		c.Request.Header.Set("Content-Type", "application/json")

		handler.Report(c)

		if w.Code != http.StatusOK {
			t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
		}

		// Verify ports
		var ports []model.SerialPort
		db.Where("node_id = ?", "node-003").Find(&ports)
		if len(ports) != 2 {
			t.Errorf("expected 2 ports, got %d", len(ports))
		}

		// Verify sessions
		var sessions []model.Session
		db.Where("node_id = ?", "node-003").Find(&sessions)
		if len(sessions) != 1 {
			t.Errorf("expected 1 session, got %d", len(sessions))
		}
	})
}

func TestListNode(t *testing.T) {
	os.Setenv("JWT_SECRET", "test-secret-key-for-testing")
	defer os.Unsetenv("JWT_SECRET")

	db := setupTestDB(t)
	handler := &NodeHandler{DB: db}

	// Seed nodes
	nodes := []model.Node{
		{NodeID: "node-a", Name: "Node A", Status: "online"},
		{NodeID: "node-b", Name: "Node B", Status: "offline"},
		{NodeID: "node-c", Name: "Node C", Status: "online"},
	}
	for _, n := range nodes {
		db.Create(&n)
	}

	t.Run("list all nodes", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("GET", "/api/nodes", nil)

		handler.List(c)

		if w.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", w.Code)
		}

		var result []model.Node
		if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
			t.Fatalf("failed to parse response: %v", err)
		}
		if len(result) != 3 {
			t.Errorf("expected 3 nodes, got %d", len(result))
		}
	})

	t.Run("filter by status", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("GET", "/api/nodes?status=online", nil)

		handler.List(c)

		if w.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", w.Code)
		}

		var result []model.Node
		if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
			t.Fatalf("failed to parse response: %v", err)
		}
		if len(result) != 2 {
			t.Errorf("expected 2 online nodes, got %d", len(result))
		}
	})
}

func TestNodeDetail(t *testing.T) {
	os.Setenv("JWT_SECRET", "test-secret-key-for-testing")
	defer os.Unsetenv("JWT_SECRET")

	db := setupTestDB(t)
	handler := &NodeHandler{DB: db}

	// Seed node with ports and sessions
	node := model.Node{NodeID: "node-detail", Name: "Detail Node", Status: "online"}
	db.Create(&node)
	db.Create(&model.SerialPort{NodeID: "node-detail", PortName: "/dev/ttyS0", Status: "online"})
	db.Create(&model.Session{SessionID: "sess-detail", NodeID: "node-detail", PortName: "/dev/ttyS0", User: "admin", Type: "master"})

	t.Run("get node detail by node_id", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("GET", "/api/nodes/node-detail", nil)
		c.Params = []gin.Param{{Key: "id", Value: "node-detail"}}

		handler.Get(c)

		if w.Code != http.StatusOK {
			t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
		}

		var resp map[string]interface{}
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("failed to parse response: %v", err)
		}
		if resp["node"] == nil {
			t.Error("expected node in response")
		}
		ports, ok := resp["ports"].([]interface{})
		if !ok || len(ports) != 1 {
			t.Errorf("expected 1 port, got %v", ports)
		}
		sessions, ok := resp["sessions"].([]interface{})
		if !ok || len(sessions) != 1 {
			t.Errorf("expected 1 session, got %v", sessions)
		}
	})

	t.Run("get nonexistent node returns 404", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("GET", "/api/nodes/nonexistent", nil)
		c.Params = []gin.Param{{Key: "id", Value: "nonexistent"}}

		handler.Get(c)

		if w.Code != http.StatusNotFound {
			t.Errorf("expected 404, got %d", w.Code)
		}
	})
}
