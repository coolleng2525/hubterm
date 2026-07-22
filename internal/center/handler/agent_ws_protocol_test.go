package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coolleng2525/hubterm/internal/center/model"
	hubtermproto "github.com/coolleng2525/hubterm/internal/proto"
	"github.com/gorilla/websocket"
)

func TestAgentToken(t *testing.T) {
	t.Run("authorization header", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/ws/agent", nil)
		req.Header.Set("Authorization", "Bearer header-token")
		if got := agentToken(req); got != "header-token" {
			t.Fatalf("agentToken() = %q", got)
		}
	})
	t.Run("websocket subprotocol", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/ws/agent", nil)
		req.Header.Set("Sec-WebSocket-Protocol", "hubterm, hubterm.node.protocol-token")
		if got := agentToken(req); got != "protocol-token" {
			t.Fatalf("agentToken() = %q", got)
		}
	})
}

func TestValidTerminalData(t *testing.T) {
	valid := hubtermproto.TerminalData{SessionID: "session-1", Direction: "output", Data: "5L2g5aW9"}
	if !validTerminalData(valid) {
		t.Fatal("expected valid terminal data")
	}
	cases := []hubtermproto.TerminalData{
		{Direction: "output", Data: "YQ=="},
		{SessionID: "session-1", Direction: "sideways", Data: "YQ=="},
		{SessionID: "session-1", Direction: "input", Data: "not-base64"},
		{SessionID: "session-1", Direction: "input", Data: strings.Repeat("YQ==", 300000)},
	}
	for _, tc := range cases {
		if validTerminalData(tc) {
			t.Fatalf("expected invalid terminal data: %+v", tc)
		}
	}
}

func TestAgentWebSocketUsesTokenNodeID(t *testing.T) {
	db := setupTestDB(t)
	if err := db.Create(&model.Node{NodeID: "canonical-node", Token: "node-token"}).Error; err != nil {
		t.Fatal(err)
	}
	handler := NewAgentWSHandler(db)
	server := httptest.NewServer(http.HandlerFunc(handler.HandleAgentWS))
	defer server.Close()

	header := http.Header{}
	header.Set("Sec-WebSocket-Protocol", "hubterm, hubterm.node.node-token")
	url := "ws" + strings.TrimPrefix(server.URL, "http") + "?node_id=random-browser-id"
	conn, _, err := websocket.DefaultDialer.Dial(url, header)
	if err != nil {
		t.Fatalf("dial agent websocket: %v", err)
	}
	defer conn.Close()

	deadline := time.Now().Add(time.Second)
	for !handler.IsNodeConnected("canonical-node") && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if !handler.IsNodeConnected("canonical-node") {
		t.Fatal("token's canonical node ID was not connected")
	}
	if handler.IsNodeConnected("random-browser-id") {
		t.Fatal("untrusted query node ID was registered")
	}
}

func TestAgentReportPreservesRenamedNodeName(t *testing.T) {
	db := setupTestDB(t)
	if err := db.Create(&model.Node{
		NodeID:       "node-renamed",
		Name:         "operator-name",
		ReportedName: "agent-old",
		Token:        "node-token",
	}).Error; err != nil {
		t.Fatal(err)
	}
	handler := NewAgentWSHandler(db)

	handler.handleReport("node-renamed", hubtermproto.NodeReport{
		Name:     "agent-latest",
		Hostname: "reported-host",
		Sessions: []hubtermproto.SessionInfo{
			{SessionID: "sess-report-1", DisplayName: "Serial: wch.cn 6&3183D08&0&2", PortName: "COM3", Type: "master"},
		},
	})

	var node model.Node
	if err := db.Where("node_id = ?", "node-renamed").First(&node).Error; err != nil {
		t.Fatalf("node not found: %v", err)
	}
	if node.Name != "operator-name" {
		t.Fatalf("expected renamed node name to be preserved, got %q", node.Name)
	}
	if node.ReportedName != "agent-latest" {
		t.Fatalf("expected reported name to track latest agent report, got %q", node.ReportedName)
	}
}

func TestAgentReportPreservesSessionDisplayNameWhenSessionIDChanges(t *testing.T) {
	db := setupTestDB(t)
	if err := db.Create(&model.Node{NodeID: "node-session-ws", Name: "node", Token: "node-token"}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.Session{
		SessionID:   "old-session",
		NodeID:      "node-session-ws",
		PortName:    "Serial: wch.cn 6&3183D08&0&2",
		User:        "tabby",
		Type:        "master",
		DisplayName: "r770-com9",
	}).Error; err != nil {
		t.Fatal(err)
	}
	handler := NewAgentWSHandler(db)

	handler.handleReport("node-session-ws", hubtermproto.NodeReport{
		Name: "node",
		Sessions: []hubtermproto.SessionInfo{
			{
				SessionID:   "new-session",
				DisplayName: "Serial: wch.cn 6&3183D08&0&2",
				PortName:    "Serial: wch.cn 6&3183D08&0&2",
				User:        "tabby",
				Type:        "master",
				ClientIP:    "tabby",
			},
		},
	})

	var session model.Session
	if err := db.Where("session_id = ?", "new-session").First(&session).Error; err != nil {
		t.Fatalf("session not found: %v", err)
	}
	if session.DisplayName != "r770-com9" {
		t.Fatalf("expected display name to be preserved, got %q", session.DisplayName)
	}
}

func TestAgentReportUsesPersistedSessionDisplayNameOverride(t *testing.T) {
	db := setupTestDB(t)
	if err := db.Create(&model.Node{NodeID: "node-session-ws-override", Name: "node", Token: "node-token"}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.SessionDisplayName{
		NodeID:      "node-session-ws-override",
		PortName:    "Serial: wch.cn 6&3183D08&0&2",
		User:        "tabby",
		DisplayName: "r770-com9",
	}).Error; err != nil {
		t.Fatal(err)
	}
	handler := NewAgentWSHandler(db)

	handler.handleReport("node-session-ws-override", hubtermproto.NodeReport{
		Name: "node",
		Sessions: []hubtermproto.SessionInfo{
			{
				SessionID:   "new-session",
				DisplayName: "Serial: wch.cn 6&3183D08&0&2",
				PortName:    "Serial: wch.cn 6&3183D08&0&2",
				User:        "tabby",
				Type:        "master",
				ClientIP:    "tabby",
			},
		},
	})

	var session model.Session
	if err := db.Where("session_id = ?", "new-session").First(&session).Error; err != nil {
		t.Fatalf("session not found: %v", err)
	}
	if session.DisplayName != "r770-com9" {
		t.Fatalf("expected display name override, got %q", session.DisplayName)
	}
}
