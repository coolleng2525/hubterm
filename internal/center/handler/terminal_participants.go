package handler

import (
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/coolleng2525/hubterm/internal/center/model"
	"github.com/coolleng2525/hubterm/internal/pkg/log"
	hubtermproto "github.com/coolleng2525/hubterm/internal/proto"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type terminalParticipantView struct {
	ID       string `json:"id"`
	Username string `json:"username"`
	Role     string `json:"role"`
}

type terminalParticipant struct {
	terminalParticipantView
	client   *browserClient
	authRole string
	joinedAt time.Time
}

type terminalParticipantSession struct {
	nodeID       string
	participants map[string]*terminalParticipant
	masterID     string
}

type terminalParticipantRegistry struct {
	mu       sync.RWMutex
	sessions map[string]*terminalParticipantSession
}

func newTerminalParticipantRegistry() *terminalParticipantRegistry {
	return &terminalParticipantRegistry{sessions: make(map[string]*terminalParticipantSession)}
}

var terminalParticipants = newTerminalParticipantRegistry()

func canControlTerminal(authRole string) bool {
	return authRole == "admin" || authRole == "operator"
}

func (r *terminalParticipantRegistry) register(client *browserClient, nodeID, sessionID string) (terminalParticipantView, []terminalParticipantView, []*browserClient) {
	r.mu.Lock()
	defer r.mu.Unlock()
	session := r.sessions[sessionID]
	if session == nil {
		session = &terminalParticipantSession{nodeID: nodeID, participants: make(map[string]*terminalParticipant)}
		r.sessions[sessionID] = session
	}
	for _, existing := range session.participants {
		if existing.client == client {
			return existing.terminalParticipantView, participantViews(session), otherParticipantClients(session, client)
		}
	}
	role := "observer"
	id := uuid.New().String()
	if session.masterID == "" && canControlTerminal(client.authRole) {
		role = "master"
		session.masterID = id
	}
	participant := &terminalParticipant{
		terminalParticipantView: terminalParticipantView{ID: id, Username: client.username, Role: role},
		client:                  client,
		authRole:                client.authRole,
		joinedAt:                time.Now(),
	}
	session.participants[id] = participant
	return participant.terminalParticipantView, participantViews(session), otherParticipantClients(session, client)
}

func (r *terminalParticipantRegistry) unregister(client *browserClient) (string, string, []terminalParticipantView, []*browserClient, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for sessionID, session := range r.sessions {
		for id, participant := range session.participants {
			if participant.client != client {
				continue
			}
			delete(session.participants, id)
			if session.masterID == id {
				session.masterID = ""
				promoteOldestController(session)
			}
			if len(session.participants) == 0 {
				delete(r.sessions, sessionID)
				return session.nodeID, sessionID, nil, nil, true
			}
			return session.nodeID, sessionID, participantViews(session), participantClients(session), false
		}
	}
	return "", "", nil, nil, false
}

func (r *terminalParticipantRegistry) isMaster(client *browserClient, sessionID string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	session := r.sessions[sessionID]
	if session == nil {
		return false
	}
	master := session.participants[session.masterID]
	return master != nil && master.client == client
}

func (r *terminalParticipantRegistry) assignMaster(sessionID, participantID string) ([]terminalParticipantView, []*browserClient, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	session := r.sessions[sessionID]
	if session == nil {
		return nil, nil, fmt.Errorf("terminal session has no participants")
	}
	target := session.participants[participantID]
	if target == nil {
		return nil, nil, fmt.Errorf("participant not found")
	}
	if !canControlTerminal(target.authRole) {
		return nil, nil, fmt.Errorf("readonly participant cannot become master")
	}
	if current := session.participants[session.masterID]; current != nil {
		current.Role = "observer"
	}
	target.Role = "master"
	session.masterID = target.ID
	return participantViews(session), participantClients(session), nil
}

func (r *terminalParticipantRegistry) kick(sessionID, participantID string) (*browserClient, []terminalParticipantView, []*browserClient, bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	session := r.sessions[sessionID]
	if session == nil {
		return nil, nil, nil, false, fmt.Errorf("terminal session has no participants")
	}
	target := session.participants[participantID]
	if target == nil {
		return nil, nil, nil, false, fmt.Errorf("participant not found")
	}
	delete(session.participants, participantID)
	if session.masterID == participantID {
		session.masterID = ""
		promoteOldestController(session)
	}
	if len(session.participants) == 0 {
		delete(r.sessions, sessionID)
		return target.client, nil, nil, true, nil
	}
	return target.client, participantViews(session), participantClients(session), false, nil
}

func (r *terminalParticipantRegistry) count(sessionID string) int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	session := r.sessions[sessionID]
	if session == nil {
		return 0
	}
	return len(session.participants)
}

func promoteOldestController(session *terminalParticipantSession) {
	var selected *terminalParticipant
	for _, participant := range session.participants {
		participant.Role = "observer"
		if canControlTerminal(participant.authRole) && (selected == nil || participant.joinedAt.Before(selected.joinedAt)) {
			selected = participant
		}
	}
	if selected != nil {
		selected.Role = "master"
		session.masterID = selected.ID
	}
}

func participantViews(session *terminalParticipantSession) []terminalParticipantView {
	views := make([]terminalParticipantView, 0, len(session.participants))
	for _, participant := range session.participants {
		views = append(views, participant.terminalParticipantView)
	}
	sort.Slice(views, func(i, j int) bool {
		if views[i].Role != views[j].Role {
			return views[i].Role == "master"
		}
		return views[i].Username < views[j].Username
	})
	return views
}

func participantClients(session *terminalParticipantSession) []*browserClient {
	clients := make([]*browserClient, 0, len(session.participants))
	for _, participant := range session.participants {
		clients = append(clients, participant.client)
	}
	return clients
}

func otherParticipantClients(session *terminalParticipantSession, excluded *browserClient) []*browserClient {
	clients := make([]*browserClient, 0, len(session.participants))
	for _, participant := range session.participants {
		if participant.client != excluded {
			clients = append(clients, participant.client)
		}
	}
	return clients
}

func broadcastParticipants(sessionID string, views []terminalParticipantView, clients []*browserClient) {
	message := hubtermproto.WSMessage{Type: "terminal_participants", Data: map[string]interface{}{
		"session_id": sessionID, "participants": views,
	}}
	for _, client := range clients {
		_ = client.writeJSON(message)
	}
}

func closeIdleSerialSession(agentWS *AgentWSHandler, nodeID, sessionID string) {
	closeIdleSerialSessionAfter(agentWS, nodeID, sessionID, 2*time.Second)
}

func closeIdleSerialSessionAfter(agentWS *AgentWSHandler, nodeID, sessionID string, gracePeriod time.Duration) {
	if agentWS == nil || agentWS.DB == nil {
		return
	}
	time.Sleep(gracePeriod)
	if terminalParticipants.count(sessionID) != 0 {
		return
	}
	var session model.Session
	if err := agentWS.DB.Where("node_id = ? AND session_id = ? AND protocol = ?", nodeID, sessionID, "serial").First(&session).Error; err != nil {
		return
	}
	command := hubtermproto.ExecCommand{ID: uuid.New().String(), Type: "serial_close"}
	command.Payload.SessionID = sessionID
	if _, err := agentWS.SendCommandAndWait(nodeID, command, 5*time.Second); err != nil {
		wsLog.Warn("failed to close idle serial session", log.String("session_id", sessionID), log.Err(err))
		return
	}
	_ = agentWS.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("session_id = ?", sessionID).Delete(&model.Session{}).Error; err != nil {
			return err
		}
		return tx.Model(&model.SerialPort{}).
			Where("node_id = ? AND current_session_id = ?", nodeID, sessionID).
			Updates(map[string]interface{}{"status": "online", "current_session_id": ""}).Error
	})
	agentWS.UnregisterTerminalSession(sessionID)
}
