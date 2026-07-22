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
		if node.ReportedName != "test-node" {
			t.Errorf("expected reported_name=test-node, got %s", node.ReportedName)
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
		c.Request.Header.Set("Authorization", "Bearer existing-token")

		handler.Report(c)

		if w.Code != http.StatusOK {
			t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
		}

		var node model.Node
		if err := db.Where("node_id = ?", "node-002").First(&node).Error; err != nil {
			t.Fatalf("node not found: %v", err)
		}
		if node.Name != "old-name" {
			t.Errorf("expected local name to be preserved, got %s", node.Name)
		}
		if node.ReportedName != "new-name" {
			t.Errorf("expected reported_name=new-name, got %s", node.ReportedName)
		}
		if node.Status != "online" {
			t.Errorf("expected status=online, got %s", node.Status)
		}
		// Token should remain unchanged for existing node
		if node.Token != "existing-token" {
			t.Errorf("expected token to remain existing-token, got %s", node.Token)
		}
		var response map[string]interface{}
		_ = json.Unmarshal(w.Body.Bytes(), &response)
		if _, leaked := response["token"]; leaked {
			t.Error("existing node report must not return its token")
		}
	})

	t.Run("existing node follows reported name until locally renamed", func(t *testing.T) {
		db.Create(&model.Node{
			NodeID:       "node-002-follow",
			Name:         "agent-old",
			ReportedName: "agent-old",
			Token:        "follow-token",
			Status:       "offline",
		})

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		body := `{"node_id":"node-002-follow","name":"agent-new","serial_ports":[],"sessions":[]}`
		c.Request = httptest.NewRequest("POST", "/api/nodes/report", strings.NewReader(body))
		c.Request.Header.Set("Content-Type", "application/json")
		c.Request.Header.Set("Authorization", "Bearer follow-token")

		handler.Report(c)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
		}
		var node model.Node
		db.Where("node_id = ?", "node-002-follow").First(&node)
		if node.Name != "agent-new" || node.ReportedName != "agent-new" {
			t.Fatalf("expected node to follow report, got name=%q reported=%q", node.Name, node.ReportedName)
		}

		if err := db.Model(&node).Update("name", "operator-name").Error; err != nil {
			t.Fatalf("failed to set local name: %v", err)
		}

		w2 := httptest.NewRecorder()
		c2, _ := gin.CreateTestContext(w2)
		body2 := `{"node_id":"node-002-follow","name":"agent-latest","serial_ports":[],"sessions":[]}`
		c2.Request = httptest.NewRequest("POST", "/api/nodes/report", strings.NewReader(body2))
		c2.Request.Header.Set("Content-Type", "application/json")
		c2.Request.Header.Set("Authorization", "Bearer follow-token")
		handler.Report(c2)

		if w2.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", w2.Code, w2.Body.String())
		}
		db.Where("node_id = ?", "node-002-follow").First(&node)
		if node.Name != "operator-name" {
			t.Errorf("expected local node name to be preserved, got %q", node.Name)
		}
		if node.ReportedName != "agent-latest" {
			t.Errorf("expected reported_name to track latest report, got %q", node.ReportedName)
		}
	})

	t.Run("existing node rejects wrong token", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		body := `{"node_id":"node-002","name":"hijacked","serial_ports":[],"sessions":[]}`
		c.Request = httptest.NewRequest("POST", "/api/nodes/report", strings.NewReader(body))
		c.Request.Header.Set("Content-Type", "application/json")
		c.Request.Header.Set("Authorization", "Bearer wrong-token")
		handler.Report(c)
		if w.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401, got %d: %s", w.Code, w.Body.String())
		}
		var node model.Node
		db.Where("node_id = ?", "node-002").First(&node)
		if node.Name == "hijacked" {
			t.Error("unauthorized report changed the node")
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

		var registered model.Node
		db.Where("node_id = ?", "node-003").First(&registered)
		w2 := httptest.NewRecorder()
		c2, _ := gin.CreateTestContext(w2)
		body2 := `{"node_id":"node-003","name":"multi-port-node","serial_ports":[{"port_name":"/dev/ttyUSB0","status":"online"}],"sessions":[]}`
		c2.Request = httptest.NewRequest("POST", "/api/nodes/report", strings.NewReader(body2))
		c2.Request.Header.Set("Content-Type", "application/json")
		c2.Request.Header.Set("Authorization", "Bearer "+registered.Token)
		handler.Report(c2)
		var remaining int64
		db.Model(&model.SerialPort{}).Where("node_id = ?", "node-003").Count(&remaining)
		if remaining != 1 {
			t.Errorf("expected stale port cleanup to leave 1 port, got %d", remaining)
		}
	})
}

func TestNodeReportPreservesSessionDisplayNameWhenSessionIDChanges(t *testing.T) {
	db := setupTestDB(t)
	handler := &NodeHandler{DB: db}

	if err := db.Create(&model.Node{NodeID: "node-session-rename", Name: "node", Token: "node-token"}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.Session{
		SessionID:   "old-session",
		NodeID:      "node-session-rename",
		PortName:    "Serial: wch.cn 6&3183D08&0&2",
		User:        "tabby",
		Type:        "master",
		DisplayName: "r770-com9",
	}).Error; err != nil {
		t.Fatal(err)
	}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	body := `{
		"node_id":"node-session-rename",
		"name":"node",
		"serial_ports":[],
		"sessions":[{
			"session_id":"new-session",
			"display_name":"Serial: wch.cn 6&3183D08&0&2",
			"port_name":"Serial: wch.cn 6&3183D08&0&2",
			"user":"tabby",
			"type":"master",
			"client_ip":"tabby",
			"connected_at":1700000000
		}]
	}`
	c.Request = httptest.NewRequest("POST", "/api/nodes/report", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Request.Header.Set("Authorization", "Bearer node-token")

	handler.Report(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var session model.Session
	if err := db.Where("session_id = ?", "new-session").First(&session).Error; err != nil {
		t.Fatalf("session not found: %v", err)
	}
	if session.DisplayName != "r770-com9" {
		t.Fatalf("expected display name to be preserved, got %q", session.DisplayName)
	}
}

func TestNodeReportUsesPersistedSessionDisplayNameOverride(t *testing.T) {
	db := setupTestDB(t)
	handler := &NodeHandler{DB: db}

	if err := db.Create(&model.Node{NodeID: "node-session-override", Name: "node", Token: "node-token"}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.SessionDisplayName{
		NodeID:      "node-session-override",
		PortName:    "Serial: wch.cn 6&3183D08&0&2",
		User:        "tabby",
		DisplayName: "r770-com9",
	}).Error; err != nil {
		t.Fatal(err)
	}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	body := `{
		"node_id":"node-session-override",
		"name":"node",
		"serial_ports":[],
		"sessions":[{
			"session_id":"new-session",
			"display_name":"Serial: wch.cn 6&3183D08&0&2",
			"port_name":"Serial: wch.cn 6&3183D08&0&2",
			"user":"tabby",
			"type":"master",
			"client_ip":"tabby",
			"connected_at":1700000000
		}]
	}`
	c.Request = httptest.NewRequest("POST", "/api/nodes/report", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Request.Header.Set("Authorization", "Bearer node-token")

	handler.Report(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var session model.Session
	if err := db.Where("session_id = ?", "new-session").First(&session).Error; err != nil {
		t.Fatalf("session not found: %v", err)
	}
	if session.DisplayName != "r770-com9" {
		t.Fatalf("expected display name override, got %q", session.DisplayName)
	}
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
