package handler

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/coolleng2525/hubterm/internal/center/model"
	hubtermproto "github.com/coolleng2525/hubterm/internal/proto"
)

func TestTerminalParticipantMasterTransferAndKick(t *testing.T) {
	registry := newTerminalParticipantRegistry()
	admin := &browserClient{username: "admin", authRole: "admin"}
	operator := &browserClient{username: "operator", authRole: "operator"}
	viewer := &browserClient{username: "viewer", authRole: "readonly"}

	adminView, _, _ := registry.register(admin, "node-1", "session-1")
	operatorView, _, _ := registry.register(operator, "node-1", "session-1")
	viewerView, _, _ := registry.register(viewer, "node-1", "session-1")
	if adminView.Role != "master" || operatorView.Role != "observer" || viewerView.Role != "observer" {
		t.Fatalf("unexpected initial roles: admin=%s operator=%s viewer=%s", adminView.Role, operatorView.Role, viewerView.Role)
	}
	if !registry.isMaster(admin, "session-1") || registry.isMaster(operator, "session-1") {
		t.Fatal("initial master was not enforced")
	}
	if _, _, err := registry.assignMaster("session-1", viewerView.ID); err == nil {
		t.Fatal("readonly participant became master")
	}
	if _, _, err := registry.assignMaster("session-1", operatorView.ID); err != nil {
		t.Fatal(err)
	}
	if registry.isMaster(admin, "session-1") || !registry.isMaster(operator, "session-1") {
		t.Fatal("master transfer was not enforced")
	}
	kicked, views, _, empty, err := registry.kick("session-1", operatorView.ID)
	if err != nil || kicked != operator || empty {
		t.Fatalf("kick result: kicked=%p views=%+v empty=%v err=%v", kicked, views, empty, err)
	}
	if !registry.isMaster(admin, "session-1") {
		t.Fatal("oldest eligible participant was not promoted")
	}
}

func TestCloseIdleSerialSession(t *testing.T) {
	db := setupTestDB(t)
	sqlDB, _ := db.DB()
	sqlDB.SetMaxOpenConns(1)
	node := model.Node{NodeID: "node-idle", Status: "online", Token: "token"}
	if err := db.Create(&node).Error; err != nil {
		t.Fatal(err)
	}
	port := model.SerialPort{NodeID: node.NodeID, PortName: "/dev/ttyUSB9", Status: "busy", CurrentSessionID: "serial-idle"}
	if err := db.Create(&port).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.Session{SessionID: "serial-idle", NodeID: node.NodeID, Protocol: "serial", PortName: port.PortName, Type: "master"}).Error; err != nil {
		t.Fatal(err)
	}
	agent := NewAgentWSHandler(db)
	client, cleanup := agentCommandTestConnection(t, agent, node.NodeID)
	defer cleanup()
	result := make(chan error, 1)
	go func() {
		var message struct {
			Data json.RawMessage `json:"data"`
		}
		if err := client.ReadJSON(&message); err != nil {
			result <- err
			return
		}
		var command hubtermproto.ExecCommand
		if err := json.Unmarshal(message.Data, &command); err != nil {
			result <- err
			return
		}
		if command.Type != "serial_close" || command.Payload.SessionID != "serial-idle" {
			result <- &unexpectedSerialCommand{command: command}
			return
		}
		payload, _ := json.Marshal(hubtermproto.ExecResult{CmdID: command.ID, ExitCode: 0})
		agent.AgentExecResultHandler(node.NodeID, payload)
		result <- nil
	}()
	closeIdleSerialSessionAfter(agent, node.NodeID, "serial-idle", time.Millisecond)
	if err := <-result; err != nil {
		t.Fatal(err)
	}
	var count int64
	db.Model(&model.Session{}).Where("session_id = ?", "serial-idle").Count(&count)
	if count != 0 {
		t.Fatal("idle serial session was not removed")
	}
	if err := db.First(&port, port.ID).Error; err != nil {
		t.Fatal(err)
	}
	if port.Status != "online" || port.CurrentSessionID != "" {
		t.Fatalf("serial port was not released: %+v", port)
	}
}

type unexpectedSerialCommand struct {
	command hubtermproto.ExecCommand
}

func (e *unexpectedSerialCommand) Error() string {
	return "unexpected serial command: " + e.command.Type
}
