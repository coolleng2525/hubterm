package connector

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

func TestHandleSerialCommand(t *testing.T) {
	connector := New("http://center", "node-1", "token")
	received := make(chan *CenterCommand, 1)
	connector.SetCommandHandler(func(cmd *CenterCommand) { received <- cmd })
	cfg := hubtermproto.DefaultSerialConfig("COM3")
	command := CenterCommand{ID: "command-1", Type: "serial_start"}
	command.Payload.SessionID = "session-1"
	command.Payload.Serial = &cfg
	payload, err := json.Marshal(command)
	if err != nil {
		t.Fatal(err)
	}
	connector.handleExecCommand(payload)
	select {
	case cmd := <-received:
		if cmd.Payload.Serial == nil || cmd.Payload.Serial.PortName != "COM3" {
			t.Fatalf("serial payload lost: %+v", cmd.Payload.Serial)
		}
	case <-time.After(time.Second):
		t.Fatal("command handler was not called")
	}
}

func TestDisconnectHandlerRunsAfterEstablishedConnectionCloses(t *testing.T) {
	upgrader := websocket.Upgrader{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Errorf("upgrade: %v", err)
			return
		}
		_ = conn.Close()
	}))
	defer server.Close()

	connector := New(server.URL, "node-1", "token")
	disconnected := make(chan struct{}, 1)
	connector.SetDisconnectHandler(func() { disconnected <- struct{}{} })
	if err := connector.connectOnce(); err == nil || !strings.Contains(err.Error(), "read message") {
		t.Fatalf("connectOnce error = %v", err)
	}
	select {
	case <-disconnected:
	case <-time.After(time.Second):
		t.Fatal("disconnect handler was not called")
	}
}
