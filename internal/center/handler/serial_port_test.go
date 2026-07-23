package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/coolleng2525/hubterm/internal/center/model"
	hubtermproto "github.com/coolleng2525/hubterm/internal/proto"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

func serialHandlerTestRouter(handler *SerialPortHandler) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("username", "operator")
		c.Next()
	})
	router.GET("/serial-ports", handler.List)
	router.PUT("/nodes/:id/serial-ports/:port_id/config", handler.UpdateConfig)
	router.POST("/nodes/:id/serial-ports/:port_id/connect", handler.Connect)
	router.DELETE("/nodes/:id/serial/:session_id", handler.Disconnect)
	return router
}

func seedSerialPort(t *testing.T) (*SerialPortHandler, model.Node, model.SerialPort, *websocket.Conn, func()) {
	t.Helper()
	db := setupTestDB(t)
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	sqlDB.SetMaxOpenConns(1)
	node := model.Node{NodeID: "node-serial", Name: "serial agent", Status: "online", Token: "token"}
	if err := db.Create(&node).Error; err != nil {
		t.Fatal(err)
	}
	port := model.SerialPort{NodeID: node.NodeID, PortName: "/dev/ttyUSB0", Status: "online", BaudRate: 115200}
	if err := db.Create(&port).Error; err != nil {
		t.Fatal(err)
	}
	agent := NewAgentWSHandler(db)
	client, cleanup := agentCommandTestConnection(t, agent, node.NodeID)
	return NewSerialPortHandler(db, agent), node, port, client, cleanup
}

func TestSerialPortConfigDefaultsAndUpdate(t *testing.T) {
	handler, node, port, _, cleanup := seedSerialPort(t)
	defer cleanup()
	router := serialHandlerTestRouter(handler)

	list := httptest.NewRecorder()
	router.ServeHTTP(list, httptest.NewRequest(http.MethodGet, "/serial-ports?node_id="+node.NodeID, nil))
	if list.Code != http.StatusOK {
		t.Fatalf("list status = %d, body = %s", list.Code, list.Body.String())
	}
	var ports []model.SerialPort
	if err := json.Unmarshal(list.Body.Bytes(), &ports); err != nil {
		t.Fatal(err)
	}
	if len(ports) != 1 || ports[0].DataBits != 8 || ports[0].Parity != "none" || ports[0].StopBits != 1 || ports[0].FlowControl != "none" {
		t.Fatalf("unexpected defaults: %+v", ports)
	}

	body := []byte(`{"alias":"  brown-serial  ","baud_rate":9600,"data_bits":7,"parity":"even","stop_bits":2,"flow_control":"none"}`)
	update := httptest.NewRecorder()
	path := "/nodes/" + node.NodeID + "/serial-ports/" + strconv.FormatUint(uint64(port.ID), 10) + "/config"
	req := httptest.NewRequest(http.MethodPut, path, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(update, req)
	if update.Code != http.StatusOK {
		t.Fatalf("update status = %d, body = %s", update.Code, update.Body.String())
	}
	var stored model.SerialPortConfig
	if err := handler.DB.Where("node_id = ? AND port_name = ?", node.NodeID, port.PortName).First(&stored).Error; err != nil {
		t.Fatal(err)
	}
	if stored.Alias != "brown-serial" || stored.BaudRate != 9600 || stored.DataBits != 7 || stored.Parity != "even" || stored.StopBits != 2 {
		t.Fatalf("unexpected stored config: %+v", stored)
	}

	updatedList := httptest.NewRecorder()
	router.ServeHTTP(updatedList, httptest.NewRequest(http.MethodGet, "/serial-ports?node_id="+node.NodeID, nil))
	if updatedList.Code != http.StatusOK {
		t.Fatalf("updated list status = %d, body = %s", updatedList.Code, updatedList.Body.String())
	}
	if err := json.Unmarshal(updatedList.Body.Bytes(), &ports); err != nil {
		t.Fatal(err)
	}
	if len(ports) != 1 || ports[0].Alias != "brown-serial" {
		t.Fatalf("updated alias was not returned: %+v", ports)
	}

	invalid := httptest.NewRecorder()
	invalidBody := []byte(`{"baud_rate":12345,"data_bits":8,"parity":"none","stop_bits":1,"flow_control":"none"}`)
	invalidReq := httptest.NewRequest(http.MethodPut, path, bytes.NewReader(invalidBody))
	invalidReq.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(invalid, invalidReq)
	if invalid.Code != http.StatusBadRequest {
		t.Fatalf("invalid config status = %d, body = %s", invalid.Code, invalid.Body.String())
	}

	tooLongAlias := httptest.NewRecorder()
	tooLongBody, err := json.Marshal(serialPortConfigRequest{
		Alias: strings.Repeat("线", 129), BaudRate: 115200, DataBits: 8,
		Parity: "none", StopBits: 1, FlowControl: "none",
	})
	if err != nil {
		t.Fatal(err)
	}
	tooLongReq := httptest.NewRequest(http.MethodPut, path, bytes.NewReader(tooLongBody))
	tooLongReq.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(tooLongAlias, tooLongReq)
	if tooLongAlias.Code != http.StatusBadRequest {
		t.Fatalf("long alias status = %d, body = %s", tooLongAlias.Code, tooLongAlias.Body.String())
	}
}

func TestSerialConnectAgentFailureDoesNotPersist(t *testing.T) {
	handler, node, port, agentClient, cleanup := seedSerialPort(t)
	defer cleanup()
	router := serialHandlerTestRouter(handler)
	go func() {
		var message struct {
			Data json.RawMessage `json:"data"`
		}
		if agentClient.ReadJSON(&message) != nil {
			return
		}
		var command hubtermproto.ExecCommand
		if json.Unmarshal(message.Data, &command) != nil {
			return
		}
		payload, _ := json.Marshal(hubtermproto.ExecResult{CmdID: command.ID, ExitCode: 1, Stderr: "permission denied"})
		handler.AgentWS.AgentExecResultHandler(node.NodeID, payload)
	}()
	path := "/nodes/" + node.NodeID + "/serial-ports/" + strconv.FormatUint(uint64(port.ID), 10) + "/connect"
	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodPost, path, nil))
	if response.Code != http.StatusBadGateway {
		t.Fatalf("connect failure status = %d, body = %s", response.Code, response.Body.String())
	}
	var count int64
	handler.DB.Model(&model.Session{}).Where("node_id = ?", node.NodeID).Count(&count)
	if count != 0 {
		t.Fatal("failed serial open persisted a session")
	}
}

func TestConcurrentSerialConnectUsesOneSession(t *testing.T) {
	handler, node, port, agentClient, cleanup := seedSerialPort(t)
	defer cleanup()
	router := serialHandlerTestRouter(handler)
	go func() {
		var message struct {
			Data json.RawMessage `json:"data"`
		}
		if agentClient.ReadJSON(&message) != nil {
			return
		}
		var command hubtermproto.ExecCommand
		if json.Unmarshal(message.Data, &command) != nil {
			return
		}
		time.Sleep(20 * time.Millisecond)
		payload, _ := json.Marshal(hubtermproto.ExecResult{CmdID: command.ID, ExitCode: 0})
		handler.AgentWS.AgentExecResultHandler(node.NodeID, payload)
	}()

	path := "/nodes/" + node.NodeID + "/serial-ports/" + strconv.FormatUint(uint64(port.ID), 10) + "/connect"
	responses := make(chan *httptest.ResponseRecorder, 2)
	for i := 0; i < 2; i++ {
		go func() {
			response := httptest.NewRecorder()
			router.ServeHTTP(response, httptest.NewRequest(http.MethodPost, path, nil))
			responses <- response
		}()
	}
	first, second := <-responses, <-responses
	statuses := map[int]int{first.Code: 1}
	statuses[second.Code]++
	if statuses[http.StatusCreated] != 1 || statuses[http.StatusOK] != 1 {
		t.Fatalf("concurrent statuses = %d, %d; bodies = %s / %s", first.Code, second.Code, first.Body.String(), second.Body.String())
	}
	var firstBody, secondBody struct {
		SessionID string `json:"session_id"`
	}
	_ = json.Unmarshal(first.Body.Bytes(), &firstBody)
	_ = json.Unmarshal(second.Body.Bytes(), &secondBody)
	if firstBody.SessionID == "" || firstBody.SessionID != secondBody.SessionID {
		t.Fatalf("concurrent connects did not reuse session: %+v / %+v", firstBody, secondBody)
	}
}

func TestSerialConnectReusesSessionAndDisconnects(t *testing.T) {
	handler, node, port, agentClient, cleanup := seedSerialPort(t)
	defer cleanup()
	router := serialHandlerTestRouter(handler)
	commands := make(chan hubtermproto.ExecCommand, 2)
	go func() {
		for i := 0; i < 2; i++ {
			var message struct {
				Type string          `json:"type"`
				Data json.RawMessage `json:"data"`
			}
			if err := agentClient.ReadJSON(&message); err != nil {
				return
			}
			var command hubtermproto.ExecCommand
			if err := json.Unmarshal(message.Data, &command); err != nil {
				return
			}
			commands <- command
			result, _ := json.Marshal(hubtermproto.ExecResult{CmdID: command.ID, ExitCode: 0})
			handler.AgentWS.AgentExecResultHandler(node.NodeID, result)
		}
	}()

	path := "/nodes/" + node.NodeID + "/serial-ports/" + strconv.FormatUint(uint64(port.ID), 10) + "/connect"
	connected := httptest.NewRecorder()
	router.ServeHTTP(connected, httptest.NewRequest(http.MethodPost, path, nil))
	if connected.Code != http.StatusCreated {
		t.Fatalf("connect status = %d, body = %s", connected.Code, connected.Body.String())
	}
	var response struct {
		SessionID string `json:"session_id"`
		Created   bool   `json:"created"`
	}
	if err := json.Unmarshal(connected.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	command := <-commands
	if command.Type != "serial_start" || command.Payload.SessionID != response.SessionID || command.Payload.Serial == nil {
		t.Fatalf("unexpected start command: %+v", command)
	}
	if config := command.Payload.Serial; config.PortName != port.PortName || config.BaudRate != 115200 || config.DataBits != 8 {
		t.Fatalf("unexpected serial config: %+v", config)
	}

	reused := httptest.NewRecorder()
	router.ServeHTTP(reused, httptest.NewRequest(http.MethodPost, path, nil))
	if reused.Code != http.StatusOK {
		t.Fatalf("reuse status = %d, body = %s", reused.Code, reused.Body.String())
	}
	var reusedResponse struct {
		SessionID string `json:"session_id"`
		Created   bool   `json:"created"`
	}
	_ = json.Unmarshal(reused.Body.Bytes(), &reusedResponse)
	if reusedResponse.Created || reusedResponse.SessionID != response.SessionID {
		t.Fatalf("unexpected reused response: %+v", reusedResponse)
	}

	closed := httptest.NewRecorder()
	closePath := "/nodes/" + node.NodeID + "/serial/" + response.SessionID
	router.ServeHTTP(closed, httptest.NewRequest(http.MethodDelete, closePath, nil))
	if closed.Code != http.StatusNoContent {
		t.Fatalf("disconnect status = %d, body = %s", closed.Code, closed.Body.String())
	}
	closeCommand := <-commands
	if closeCommand.Type != "serial_close" || closeCommand.Payload.SessionID != response.SessionID {
		t.Fatalf("unexpected close command: %+v", closeCommand)
	}
	var count int64
	handler.DB.Model(&model.Session{}).Where("session_id = ?", response.SessionID).Count(&count)
	if count != 0 {
		t.Fatal("serial session was not removed")
	}
}
