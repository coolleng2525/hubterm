package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"

	"github.com/coolleng2525/hubterm/internal/center/middleware"
	"github.com/coolleng2525/hubterm/internal/center/model"
	"github.com/coolleng2525/hubterm/internal/pkg/log"
	hubtermproto "github.com/coolleng2525/hubterm/internal/proto"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	Subprotocols: []string{"hubterm"},
	CheckOrigin: func(r *http.Request) bool {
		origin := r.Header.Get("Origin")
		if origin == "" {
			return true
		}
		originURL, err := url.Parse(origin)
		if err != nil {
			return false
		}
		if originURL.Scheme == "file" || originURL.Scheme == "app" {
			return true
		}
		if originURL.Hostname() == "" {
			return false
		}
		requestURL, err := url.Parse("http://" + r.Host)
		return err == nil && strings.EqualFold(originURL.Hostname(), requestURL.Hostname())
	},
}

func AuthenticateWebSocket(r *http.Request) (*middleware.Claims, error) {
	auth := r.Header.Get("Authorization")
	if strings.HasPrefix(auth, "Bearer ") {
		return middleware.ParseToken(strings.TrimPrefix(auth, "Bearer "))
	}
	for _, protocol := range websocket.Subprotocols(r) {
		if strings.HasPrefix(protocol, "hubterm.auth.") {
			return middleware.ParseToken(strings.TrimPrefix(protocol, "hubterm.auth."))
		}
	}
	return nil, fmt.Errorf("missing websocket token")
}

type browserClient struct {
	conn      *websocket.Conn
	writeMu   sync.Mutex
	subMu     sync.RWMutex
	username  string
	authRole  string
	nodeID    string
	sessionID string
}

func (c *browserClient) writeJSON(value interface{}) error {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	return c.conn.WriteJSON(value)
}

func (c *browserClient) subscribe(nodeID, sessionID string) {
	c.subMu.Lock()
	c.nodeID, c.sessionID = nodeID, sessionID
	c.subMu.Unlock()
}

func (c *browserClient) matches(nodeID, sessionID string) bool {
	c.subMu.RLock()
	defer c.subMu.RUnlock()
	return c.nodeID == nodeID && c.sessionID == sessionID
}

func (c *browserClient) subscribedTo(sessionID string) bool {
	c.subMu.RLock()
	defer c.subMu.RUnlock()
	return c.sessionID == sessionID
}

func (c *browserClient) subscriptionNodeID() string {
	c.subMu.RLock()
	defer c.subMu.RUnlock()
	return c.nodeID
}

var (
	wsClients   = make(map[*websocket.Conn]*browserClient)
	wsClientsMu sync.RWMutex
)

var wsLog = log.New("ws")

// HandleWS handles authenticated browser WebSocket connections.
func HandleWS(r *http.Request, w http.ResponseWriter, agentWS *AgentWSHandler) {
	claims, err := AuthenticateWebSocket(r)
	if err != nil {
		wsLog.Warn("ws auth failed", log.String("reason", err.Error()))
		http.Error(w, "invalid token", http.StatusUnauthorized)
		return
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		wsLog.Error("ws upgrade error", log.Err(err))
		return
	}
	client := &browserClient{conn: conn, username: claims.Username, authRole: claims.Role}
	wsClientsMu.Lock()
	wsClients[conn] = client
	wsClientsMu.Unlock()

	defer func() {
		nodeID, sessionID, views, clients, empty := terminalParticipants.unregister(client)
		if sessionID != "" {
			broadcastParticipants(sessionID, views, clients)
			if empty {
				go closeIdleSerialSession(agentWS, nodeID, sessionID)
			}
		}
		wsClientsMu.Lock()
		delete(wsClients, conn)
		wsClientsMu.Unlock()
		_ = conn.Close()
	}()

	for {
		var msg hubtermproto.WSMessage
		if err := conn.ReadJSON(&msg); err != nil {
			break
		}
		raw, err := json.Marshal(msg.Data)
		if err != nil {
			continue
		}
		switch msg.Type {
		case "terminal_subscribe":
			var sub hubtermproto.TerminalSubscription
			if json.Unmarshal(raw, &sub) == nil && sub.NodeID != "" && sub.SessionID != "" {
				if agentWS == nil || !agentWS.ownsSession(sub.NodeID, sub.SessionID) {
					_ = client.writeJSON(hubtermproto.WSMessage{Type: "error", Data: map[string]string{"message": "terminal session not found"}})
					continue
				}
				client.subscribe(sub.NodeID, sub.SessionID)
				participant, views, existingClients := terminalParticipants.register(client, sub.NodeID, sub.SessionID)
				_ = client.writeJSON(hubtermproto.WSMessage{Type: "terminal_subscribed", Data: map[string]interface{}{
					"node_id": sub.NodeID, "session_id": sub.SessionID, "participant_id": participant.ID,
					"role": participant.Role, "participants": views,
				}})
				broadcastParticipants(sub.SessionID, views, existingClients)
			}
		case "terminal_input":
			if claims.Role != "admin" && claims.Role != "operator" {
				_ = client.writeJSON(hubtermproto.WSMessage{Type: "error", Data: map[string]string{"message": "operator required"}})
				continue
			}
			var input hubtermproto.TerminalInput
			if json.Unmarshal(raw, &input) != nil || !client.matches(input.NodeID, input.SessionID) || !validTerminalInput(input) {
				continue
			}
			if agentWS == nil {
				continue
			}
			if !terminalParticipants.isMaster(client, input.SessionID) {
				_ = client.writeJSON(hubtermproto.WSMessage{Type: "error", Data: map[string]string{"message": "terminal is read-only"}})
				continue
			}
			if err := agentWS.SendTerminalInput(input.NodeID, input.SessionID, input.Data); err != nil {
				_ = client.writeJSON(hubtermproto.WSMessage{Type: "error", Data: map[string]string{"message": err.Error()}})
			}
		case "terminal_assign_master":
			if claims.Role != "admin" {
				_ = client.writeJSON(hubtermproto.WSMessage{Type: "error", Data: map[string]string{"message": "admin required"}})
				continue
			}
			var req struct {
				SessionID     string `json:"session_id"`
				ParticipantID string `json:"participant_id"`
			}
			if json.Unmarshal(raw, &req) != nil || !client.subscribedTo(req.SessionID) {
				continue
			}
			views, clients, err := terminalParticipants.assignMaster(req.SessionID, req.ParticipantID)
			if err != nil {
				_ = client.writeJSON(hubtermproto.WSMessage{Type: "error", Data: map[string]string{"message": err.Error()}})
				continue
			}
			broadcastParticipants(req.SessionID, views, clients)
		case "terminal_kick_participant":
			if claims.Role != "admin" {
				_ = client.writeJSON(hubtermproto.WSMessage{Type: "error", Data: map[string]string{"message": "admin required"}})
				continue
			}
			var req struct {
				SessionID     string `json:"session_id"`
				ParticipantID string `json:"participant_id"`
			}
			if json.Unmarshal(raw, &req) != nil || !client.subscribedTo(req.SessionID) {
				continue
			}
			kicked, views, clients, empty, err := terminalParticipants.kick(req.SessionID, req.ParticipantID)
			if err != nil {
				_ = client.writeJSON(hubtermproto.WSMessage{Type: "error", Data: map[string]string{"message": err.Error()}})
				continue
			}
			if kicked != nil {
				_ = kicked.writeJSON(hubtermproto.WSMessage{Type: "terminal_kicked", Data: map[string]string{"session_id": req.SessionID}})
				_ = kicked.conn.Close()
			}
			broadcastParticipants(req.SessionID, views, clients)
			if empty {
				go closeIdleSerialSession(agentWS, client.subscriptionNodeID(), req.SessionID)
			}
		}
	}
}

func snapshotBrowserClients() []*browserClient {
	wsClientsMu.RLock()
	defer wsClientsMu.RUnlock()
	clients := make([]*browserClient, 0, len(wsClients))
	for _, client := range wsClients {
		clients = append(clients, client)
	}
	return clients
}

func removeBrowserClients(failed []*browserClient) {
	if len(failed) == 0 {
		return
	}
	wsClientsMu.Lock()
	defer wsClientsMu.Unlock()
	for _, client := range failed {
		delete(wsClients, client.conn)
		_ = client.conn.Close()
	}
}

// BroadcastNodeUpdate sends a node update to all connected browser clients.
func BroadcastNodeUpdate(node model.Node) {
	msg := hubtermproto.WSMessage{Type: "node_update", Data: node}
	var failed []*browserClient
	for _, client := range snapshotBrowserClients() {
		if err := client.writeJSON(msg); err != nil {
			failed = append(failed, client)
		}
	}
	removeBrowserClients(failed)
}

// BroadcastTerminalData forwards traffic only to browsers subscribed to this session.
func BroadcastTerminalData(nodeID string, terminalData hubtermproto.TerminalData) {
	msg := hubtermproto.WSMessage{
		Type: "terminal_data",
		Data: map[string]interface{}{"node_id": nodeID, "terminal": terminalData},
	}
	var failed []*browserClient
	for _, client := range snapshotBrowserClients() {
		if !client.matches(nodeID, terminalData.SessionID) {
			continue
		}
		if err := client.writeJSON(msg); err != nil {
			failed = append(failed, client)
		}
	}
	removeBrowserClients(failed)
}

func BroadcastTerminalState(nodeID string, state hubtermproto.TerminalState) {
	msg := hubtermproto.WSMessage{
		Type: "terminal_state",
		Data: map[string]interface{}{"node_id": nodeID, "terminal": state},
	}
	var failed []*browserClient
	for _, client := range snapshotBrowserClients() {
		if !client.matches(nodeID, state.SessionID) {
			continue
		}
		if err := client.writeJSON(msg); err != nil {
			failed = append(failed, client)
		}
	}
	removeBrowserClients(failed)
}
