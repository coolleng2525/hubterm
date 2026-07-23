package handler

import (
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/coolleng2525/hubterm/internal/center/model"
	hubtermproto "github.com/coolleng2525/hubterm/internal/proto"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type SerialPortHandler struct {
	DB      *gorm.DB
	AgentWS *AgentWSHandler

	mu      sync.Mutex
	opening map[string]*serialOpen
}

type serialOpen struct {
	done      chan struct{}
	sessionID string
	err       error
}

type serialPortConfigRequest struct {
	Alias       string `json:"alias"`
	BaudRate    int    `json:"baud_rate"`
	DataBits    int    `json:"data_bits"`
	Parity      string `json:"parity"`
	StopBits    int    `json:"stop_bits"`
	FlowControl string `json:"flow_control"`
}

func NewSerialPortHandler(db *gorm.DB, agentWS *AgentWSHandler) *SerialPortHandler {
	return &SerialPortHandler{DB: db, AgentWS: agentWS, opening: make(map[string]*serialOpen)}
}

func (h *SerialPortHandler) List(c *gin.Context) {
	nodeID := c.Query("node_id")
	query := h.DB.Model(&model.SerialPort{})
	if nodeID != "" {
		query = query.Where("node_id = ?", nodeID)
	}
	var ports []model.SerialPort
	if err := query.Order("node_id, port_name").Find(&ports).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list serial ports"})
		return
	}
	if err := applySerialPortConfigs(h.DB, ports); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load serial port settings"})
		return
	}
	c.JSON(http.StatusOK, ports)
}

func applySerialPortConfigs(db *gorm.DB, ports []model.SerialPort) error {
	if len(ports) == 0 {
		return nil
	}
	nodeIDs := make([]string, 0, len(ports))
	seen := make(map[string]struct{})
	for _, port := range ports {
		if _, ok := seen[port.NodeID]; !ok {
			seen[port.NodeID] = struct{}{}
			nodeIDs = append(nodeIDs, port.NodeID)
		}
	}
	var configs []model.SerialPortConfig
	if err := db.Where("node_id IN ?", nodeIDs).Find(&configs).Error; err != nil {
		return err
	}
	byPort := make(map[string]model.SerialPortConfig, len(configs))
	for _, config := range configs {
		byPort[config.NodeID+"\x00"+config.PortName] = config
	}
	for i := range ports {
		defaults := hubtermproto.DefaultSerialConfig(ports[i].PortName)
		if ports[i].BaudRate > 0 {
			defaults.BaudRate = ports[i].BaudRate
		}
		if stored, ok := byPort[ports[i].NodeID+"\x00"+ports[i].PortName]; ok {
			ports[i].Alias = stored.Alias
			defaults.BaudRate = stored.BaudRate
			defaults.DataBits = stored.DataBits
			defaults.Parity = stored.Parity
			defaults.StopBits = stored.StopBits
			defaults.FlowControl = stored.FlowControl
		}
		ports[i].BaudRate = defaults.BaudRate
		ports[i].DataBits = defaults.DataBits
		ports[i].Parity = defaults.Parity
		ports[i].StopBits = defaults.StopBits
		ports[i].FlowControl = defaults.FlowControl
	}
	return nil
}

func (h *SerialPortHandler) UpdateConfig(c *gin.Context) {
	node, port, ok := h.nodeAndPort(c)
	if !ok {
		return
	}
	if port.CurrentSessionID != "" || port.Status == "busy" {
		c.JSON(http.StatusConflict, gin.H{"error": "serial port is in use"})
		return
	}
	var request serialPortConfigRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	request.Alias = strings.TrimSpace(request.Alias)
	if len([]rune(request.Alias)) > 128 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "serial port alias is too long"})
		return
	}
	config := hubtermproto.SerialConfig{
		PortName:    port.PortName,
		BaudRate:    request.BaudRate,
		DataBits:    request.DataBits,
		Parity:      request.Parity,
		StopBits:    request.StopBits,
		FlowControl: request.FlowControl,
	}
	if err := config.Validate(); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	stored := model.SerialPortConfig{}
	result := h.DB.Where("node_id = ? AND port_name = ?", node.NodeID, port.PortName).First(&stored)
	if result.Error != nil && result.Error != gorm.ErrRecordNotFound {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load serial port settings"})
		return
	}
	stored.NodeID = node.NodeID
	stored.PortName = port.PortName
	stored.Alias = request.Alias
	stored.BaudRate = config.BaudRate
	stored.DataBits = config.DataBits
	stored.Parity = config.Parity
	stored.StopBits = config.StopBits
	stored.FlowControl = config.FlowControl
	if err := h.DB.Save(&stored).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save serial port settings"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"alias": request.Alias, "baud_rate": config.BaudRate, "data_bits": config.DataBits,
		"parity": config.Parity, "stop_bits": config.StopBits, "flow_control": config.FlowControl,
	})
}

func (h *SerialPortHandler) Connect(c *gin.Context) {
	node, port, ok := h.nodeAndPort(c)
	if !ok {
		return
	}
	if node.Status != "online" || h.AgentWS == nil || !h.AgentWS.IsNodeConnected(node.NodeID) {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "agent is offline"})
		return
	}
	if existing := h.activeSerialSession(node.NodeID, port.PortName); existing != nil {
		c.JSON(http.StatusOK, serialConnectResponse(*existing, false))
		return
	}

	key := node.NodeID + "\x00" + port.PortName
	h.mu.Lock()
	if h.opening == nil {
		h.opening = make(map[string]*serialOpen)
	}
	if pending := h.opening[key]; pending != nil {
		h.mu.Unlock()
		select {
		case <-pending.done:
			if pending.err != nil {
				c.JSON(http.StatusBadGateway, gin.H{"error": pending.err.Error()})
				return
			}
			var session model.Session
			if err := h.DB.Where("session_id = ?", pending.sessionID).First(&session).Error; err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "serial session was not persisted"})
				return
			}
			c.JSON(http.StatusOK, serialConnectResponse(session, false))
		case <-time.After(12 * time.Second):
			c.JSON(http.StatusGatewayTimeout, gin.H{"error": "serial connection timed out"})
		}
		return
	}
	pending := &serialOpen{done: make(chan struct{})}
	h.opening[key] = pending
	h.mu.Unlock()

	session, err := h.openSerial(c, node, port)
	pending.err = err
	if session != nil {
		pending.sessionID = session.SessionID
	}
	close(pending.done)
	h.mu.Lock()
	delete(h.opening, key)
	h.mu.Unlock()
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, serialConnectResponse(*session, true))
}

func (h *SerialPortHandler) openSerial(c *gin.Context, node model.Node, port model.SerialPort) (*model.Session, error) {
	config, err := h.serialConfig(node.NodeID, port)
	if err != nil {
		return nil, err
	}
	sessionID := uuid.New().String()
	command := hubtermproto.ExecCommand{ID: uuid.New().String(), Type: "serial_start"}
	command.Payload.SessionID = sessionID
	command.Payload.Serial = &config
	if _, err := h.AgentWS.SendCommandAndWait(node.NodeID, command, 10*time.Second); err != nil {
		return nil, fmt.Errorf("open serial port: %w", err)
	}
	username, _ := c.Get("username")
	user, _ := username.(string)
	if user == "" {
		user = "unknown"
	}
	session := &model.Session{
		SessionID: sessionID, NodeID: node.NodeID, Protocol: "serial", PortName: port.PortName,
		User: user, Type: "master", ClientIP: c.ClientIP(), ConnectedAt: time.Now(),
	}
	err = h.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(session).Error; err != nil {
			return err
		}
		return tx.Model(&model.SerialPort{}).Where("id = ?", port.ID).Updates(map[string]interface{}{
			"status": "busy", "current_session_id": sessionID,
		}).Error
	})
	if err != nil {
		closeCommand := hubtermproto.ExecCommand{ID: uuid.New().String(), Type: "serial_close"}
		closeCommand.Payload.SessionID = sessionID
		_, _ = h.AgentWS.SendCommandAndWait(node.NodeID, closeCommand, 3*time.Second)
		return nil, fmt.Errorf("persist serial session: %w", err)
	}
	h.AgentWS.RegisterTerminalSession(node.NodeID, sessionID)
	_ = h.DB.Create(&model.AuditLog{User: user, Action: "serial_connect", Target: sessionID, Detail: port.PortName, IP: c.ClientIP()}).Error
	return session, nil
}

func (h *SerialPortHandler) Disconnect(c *gin.Context) {
	var node model.Node
	if err := h.DB.Where("node_id = ? OR id = ?", c.Param("id"), c.Param("id")).First(&node).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "node not found"})
		return
	}
	var session model.Session
	if err := h.DB.Where("node_id = ? AND session_id = ? AND protocol = ?", node.NodeID, c.Param("session_id"), "serial").First(&session).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "serial session not found"})
		return
	}
	command := hubtermproto.ExecCommand{ID: uuid.New().String(), Type: "serial_close"}
	command.Payload.SessionID = session.SessionID
	if h.AgentWS == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "agent command channel is unavailable"})
		return
	}
	if _, err := h.AgentWS.SendCommandAndWait(node.NodeID, command, 5*time.Second); err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": fmt.Sprintf("close serial port: %v", err)})
		return
	}
	if err := h.clearSerialSession(session); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to clear serial session"})
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *SerialPortHandler) clearSerialSession(session model.Session) error {
	err := h.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("session_id = ?", session.SessionID).Delete(&model.Session{}).Error; err != nil {
			return err
		}
		return tx.Model(&model.SerialPort{}).
			Where("node_id = ? AND port_name = ? AND current_session_id = ?", session.NodeID, session.PortName, session.SessionID).
			Updates(map[string]interface{}{"status": "online", "current_session_id": ""}).Error
	})
	if err == nil && h.AgentWS != nil {
		h.AgentWS.UnregisterTerminalSession(session.SessionID)
	}
	return err
}

func (h *SerialPortHandler) nodeAndPort(c *gin.Context) (model.Node, model.SerialPort, bool) {
	var node model.Node
	if err := h.DB.Where("node_id = ? OR id = ?", c.Param("id"), c.Param("id")).First(&node).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "node not found"})
		return node, model.SerialPort{}, false
	}
	var port model.SerialPort
	if err := h.DB.Where("id = ? AND node_id = ?", c.Param("port_id"), node.NodeID).First(&port).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "serial port not found"})
		return node, port, false
	}
	return node, port, true
}

func (h *SerialPortHandler) serialConfig(nodeID string, port model.SerialPort) (hubtermproto.SerialConfig, error) {
	config := hubtermproto.DefaultSerialConfig(port.PortName)
	if port.BaudRate > 0 {
		config.BaudRate = port.BaudRate
	}
	var stored model.SerialPortConfig
	err := h.DB.Where("node_id = ? AND port_name = ?", nodeID, port.PortName).First(&stored).Error
	if err != nil && err != gorm.ErrRecordNotFound {
		return config, err
	}
	if err == nil {
		config.BaudRate = stored.BaudRate
		config.DataBits = stored.DataBits
		config.Parity = stored.Parity
		config.StopBits = stored.StopBits
		config.FlowControl = stored.FlowControl
	}
	return config, config.Validate()
}

func (h *SerialPortHandler) activeSerialSession(nodeID, portName string) *model.Session {
	var session model.Session
	if err := h.DB.Where("node_id = ? AND port_name = ? AND protocol = ?", nodeID, portName, "serial").Order("connected_at DESC").First(&session).Error; err != nil {
		return nil
	}
	return &session
}

func serialConnectResponse(session model.Session, created bool) gin.H {
	return gin.H{
		"session_id": session.SessionID, "node_id": session.NodeID, "port_name": session.PortName,
		"protocol": "serial", "created": created,
	}
}
