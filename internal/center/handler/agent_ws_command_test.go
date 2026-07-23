package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	hubtermproto "github.com/coolleng2525/hubterm/internal/proto"
	"github.com/gorilla/websocket"
)

func agentCommandTestConnection(t *testing.T, handler *AgentWSHandler, nodeID string) (*websocket.Conn, func()) {
	t.Helper()
	serverConn := make(chan *websocket.Conn, 1)
	release := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := (&websocket.Upgrader{}).Upgrade(w, r, nil)
		if err != nil {
			t.Errorf("upgrade: %v", err)
			return
		}
		serverConn <- conn
		<-release
		_ = conn.Close()
	}))
	client, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(server.URL, "http"), nil)
	if err != nil {
		server.Close()
		t.Fatal(err)
	}
	conn := <-serverConn
	handler.mu.Lock()
	handler.agentConns[nodeID] = &agentConnection{nodeID: nodeID, conn: conn}
	handler.mu.Unlock()
	cleanup := func() {
		_ = client.Close()
		close(release)
		server.Close()
	}
	return client, cleanup
}

func TestSendCommandAndWaitSuccessAndAgentError(t *testing.T) {
	handler := NewAgentWSHandler(nil)
	client, cleanup := agentCommandTestConnection(t, handler, "node-1")
	defer cleanup()

	t.Run("success", func(t *testing.T) {
		done := make(chan error, 1)
		go func() {
			_, err := handler.SendCommandAndWait("node-1", hubtermproto.ExecCommand{ID: "wait-success", Type: "serial_start"}, time.Second)
			done <- err
		}()
		var sent hubtermproto.WSMessage
		if err := client.ReadJSON(&sent); err != nil {
			t.Fatal(err)
		}
		payload, _ := json.Marshal(hubtermproto.ExecResult{CmdID: "wait-success", ExitCode: 0})
		handler.AgentExecResultHandler("node-1", payload)
		if err := <-done; err != nil {
			t.Fatalf("wait returned error: %v", err)
		}
		if GetExecResult("wait-success") != nil {
			t.Fatal("completed waiter leaked result entry")
		}
	})

	t.Run("agent error", func(t *testing.T) {
		done := make(chan error, 1)
		go func() {
			_, err := handler.SendCommandAndWait("node-1", hubtermproto.ExecCommand{ID: "wait-error", Type: "serial_start"}, time.Second)
			done <- err
		}()
		var sent hubtermproto.WSMessage
		if err := client.ReadJSON(&sent); err != nil {
			t.Fatal(err)
		}
		payload, _ := json.Marshal(hubtermproto.ExecResult{CmdID: "wait-error", Stderr: "permission denied", ExitCode: 1})
		handler.AgentExecResultHandler("node-1", payload)
		if err := <-done; err == nil || !strings.Contains(err.Error(), "permission denied") {
			t.Fatalf("wait error = %v", err)
		}
	})
}

func TestSendCommandAndWaitTimeoutAndDisconnect(t *testing.T) {
	handler := NewAgentWSHandler(nil)
	client, cleanup := agentCommandTestConnection(t, handler, "node-1")
	defer cleanup()

	done := make(chan error, 1)
	go func() {
		_, err := handler.SendCommandAndWait("node-1", hubtermproto.ExecCommand{ID: "wait-timeout", Type: "serial_start"}, 20*time.Millisecond)
		done <- err
	}()
	var sent hubtermproto.WSMessage
	if err := client.ReadJSON(&sent); err != nil {
		t.Fatal(err)
	}
	if err := <-done; err == nil || !strings.Contains(err.Error(), "timed out") {
		t.Fatalf("timeout error = %v", err)
	}
	if GetExecResult("wait-timeout") != nil {
		t.Fatal("timed out waiter leaked result entry")
	}

	go func() {
		_, err := handler.SendCommandAndWait("node-1", hubtermproto.ExecCommand{ID: "wait-disconnect", Type: "serial_start"}, time.Second)
		done <- err
	}()
	if err := client.ReadJSON(&sent); err != nil {
		t.Fatal(err)
	}
	handler.failPendingForNode("node-1", "agent disconnected")
	if err := <-done; err == nil || !strings.Contains(err.Error(), "agent disconnected") {
		t.Fatalf("disconnect error = %v", err)
	}
}
