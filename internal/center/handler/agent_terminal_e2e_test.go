package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/coolleng2525/hubterm/internal/center/middleware"
	"github.com/coolleng2525/hubterm/internal/center/model"
	hubtermproto "github.com/coolleng2525/hubterm/internal/proto"
	"github.com/gorilla/websocket"
)

func TestAgentBrowserTerminalRoundTrip(t *testing.T) {
	os.Setenv("JWT_SECRET", "test-secret-key-for-testing")
	db := setupTestDB(t)
	sqlDB, _ := db.DB()
	sqlDB.SetMaxOpenConns(1)
	if err := db.Create(&model.Node{NodeID: "node-shared", Token: "node-token"}).Error; err != nil {
		t.Fatal(err)
	}
	agentHandler := NewAgentWSHandler(db)
	agentServer := httptest.NewServer(http.HandlerFunc(agentHandler.HandleAgentWS))
	defer agentServer.Close()
	agentHeader := http.Header{}
	agentHeader.Set("Sec-WebSocket-Protocol", "hubterm, hubterm.node.node-token")
	agentConn, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(agentServer.URL, "http"), agentHeader)
	if err != nil {
		t.Fatalf("dial agent: %v", err)
	}
	defer agentConn.Close()

	report := hubtermproto.NodeReport{Name: "Tabby test", Hostname: "tabby-host", OS: "linux",
		CPUPercent: 12.5, MemoryTotal: 8589934592, MemoryUsed: 4294967296, MemoryPercent: 50,
		DiskTotal: 107374182400, DiskUsed: 53687091200,
		Sessions: []hubtermproto.SessionInfo{{SessionID: "session-shared", PortName: "SSH test", Type: "master", ConnectedAt: time.Now().Unix()}}}
	if err := agentConn.WriteJSON(hubtermproto.WSMessage{Type: "report", Data: report}); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(time.Second)
	for {
		var count int64
		db.Model(&model.Session{}).Where("session_id = ?", "session-shared").Count(&count)
		if count == 1 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("agent report did not create session")
		}
		time.Sleep(time.Millisecond)
	}
	var updatedNode model.Node
	if err := db.Where("node_id = ?", "node-shared").First(&updatedNode).Error; err != nil {
		t.Fatal(err)
	}
	if updatedNode.CPUPercent != report.CPUPercent ||
		updatedNode.MemoryTotal != report.MemoryTotal ||
		updatedNode.MemoryUsed != report.MemoryUsed ||
		updatedNode.MemoryPercent != report.MemoryPercent ||
		updatedNode.DiskTotal != report.DiskTotal ||
		updatedNode.DiskUsed != report.DiskUsed {
		t.Fatalf("agent report did not update metrics: %+v", updatedNode)
	}

	browserServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { HandleWS(r, w, agentHandler) }))
	defer browserServer.Close()
	token, err := middleware.GenerateToken(1, "operator", "operator")
	if err != nil {
		t.Fatal(err)
	}
	browserHeader := http.Header{}
	browserHeader.Set("Sec-WebSocket-Protocol", "hubterm, hubterm.auth."+token)
	browserConn, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(browserServer.URL, "http"), browserHeader)
	if err != nil {
		t.Fatalf("dial browser: %v", err)
	}
	defer browserConn.Close()

	subscription := hubtermproto.TerminalSubscription{NodeID: "node-shared", SessionID: "session-shared"}
	if err := browserConn.WriteJSON(hubtermproto.WSMessage{Type: "terminal_subscribe", Data: subscription}); err != nil {
		t.Fatal(err)
	}
	var subscribed hubtermproto.WSMessage
	if err := browserConn.ReadJSON(&subscribed); err != nil || subscribed.Type != "terminal_subscribed" {
		t.Fatalf("subscription acknowledgement: type=%q err=%v", subscribed.Type, err)
	}

	output := hubtermproto.TerminalData{SessionID: "session-shared", Direction: "output", Data: "5L2g5aW9"}
	if err := agentConn.WriteJSON(hubtermproto.WSMessage{Type: "terminal_data", Data: output}); err != nil {
		t.Fatal(err)
	}
	var browserMessage struct {
		Type string `json:"type"`
		Data struct {
			NodeID   string                    `json:"node_id"`
			Terminal hubtermproto.TerminalData `json:"terminal"`
		} `json:"data"`
	}
	if err := browserConn.ReadJSON(&browserMessage); err != nil {
		t.Fatal(err)
	}
	if browserMessage.Type != "terminal_data" || browserMessage.Data.NodeID != "node-shared" || browserMessage.Data.Terminal.Data != output.Data {
		t.Fatalf("unexpected browser terminal message: %+v", browserMessage)
	}

	input := hubtermproto.TerminalInput{NodeID: "node-shared", SessionID: "session-shared", Data: "bHMK"}
	if err := browserConn.WriteJSON(hubtermproto.WSMessage{Type: "terminal_input", Data: input}); err != nil {
		t.Fatal(err)
	}
	_ = agentConn.SetReadDeadline(time.Now().Add(time.Second))
	var agentMessage struct {
		Type string `json:"type"`
		Data struct {
			Payload struct {
				SessionID string `json:"session_id"`
				Data      string `json:"data"`
			} `json:"payload"`
		} `json:"data"`
	}
	if err := agentConn.ReadJSON(&agentMessage); err != nil {
		t.Fatal(err)
	}
	if agentMessage.Type != "write" || agentMessage.Data.Payload.SessionID != input.SessionID || agentMessage.Data.Payload.Data != input.Data {
		encoded, _ := json.Marshal(agentMessage)
		t.Fatalf("unexpected agent input command: %s", encoded)
	}

	readonlyToken, err := middleware.GenerateToken(2, "viewer", "readonly")
	if err != nil {
		t.Fatal(err)
	}
	readonlyHeader := http.Header{}
	readonlyHeader.Set("Sec-WebSocket-Protocol", "hubterm, hubterm.auth."+readonlyToken)
	readonlyConn, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(browserServer.URL, "http"), readonlyHeader)
	if err != nil {
		t.Fatalf("dial readonly browser: %v", err)
	}
	defer readonlyConn.Close()
	if err := readonlyConn.WriteJSON(hubtermproto.WSMessage{Type: "terminal_subscribe", Data: subscription}); err != nil {
		t.Fatal(err)
	}
	if err := readonlyConn.ReadJSON(&subscribed); err != nil {
		t.Fatal(err)
	}
	if err := readonlyConn.WriteJSON(hubtermproto.WSMessage{Type: "terminal_input", Data: input}); err != nil {
		t.Fatal(err)
	}
	var denied struct {
		Type string            `json:"type"`
		Data map[string]string `json:"data"`
	}
	if err := readonlyConn.ReadJSON(&denied); err != nil {
		t.Fatal(err)
	}
	if denied.Type != "error" || denied.Data["message"] != "operator required" {
		t.Fatalf("readonly terminal input was not denied: %+v", denied)
	}
}
