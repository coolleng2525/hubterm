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
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	Subprotocols: []string{"hubterm"},
	CheckOrigin: func(r *http.Request) bool {
		origin := r.Header.Get("Origin")
		if origin == "" {
			return true
		}
		u, err := url.Parse(origin)
		return err == nil && strings.EqualFold(u.Host, r.Host)
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

var (
	wsClients   = make(map[*websocket.Conn]bool)
	wsClientsMu sync.RWMutex
)

var wsLog = log.New("ws")

// HandleWS handles WebSocket connections with token authentication.
// FIXED: Token is validated via URL query parameter.
func HandleWS(r *http.Request, w http.ResponseWriter) {
	// FIXED: Validate token from URL query parameter
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

	wsClientsMu.Lock()
	wsClients[conn] = true
	wsClientsMu.Unlock()

	wsLog.Info("ws connected",
		log.String("username", claims.Username),
		log.String("user_id", string(rune(claims.UserID))),
	)

	defer func() {
		wsClientsMu.Lock()
		delete(wsClients, conn)
		wsClientsMu.Unlock()
		conn.Close()
	}()

	for {
		_, _, err := conn.ReadMessage()
		if err != nil {
			break
		}
	}
}

// BroadcastNodeUpdate sends a node update to all connected WebSocket clients.
// FIXED: Collect failed connections under RLock, then delete after releasing lock.
func BroadcastNodeUpdate(node model.Node) {
	data, err := json.Marshal(map[string]interface{}{
		"type": "node_update",
		"data": node,
	})
	if err != nil {
		return
	}

	wsClientsMu.RLock()
	var failed []*websocket.Conn
	for conn := range wsClients {
		err := conn.WriteMessage(websocket.TextMessage, data)
		if err != nil {
			wsLog.Warn("ws write error", log.Err(err))
			conn.Close()
			failed = append(failed, conn)
		}
	}
	wsClientsMu.RUnlock()

	// Delete failed connections outside the lock
	if len(failed) > 0 {
		wsClientsMu.Lock()
		for _, conn := range failed {
			delete(wsClients, conn)
		}
		wsClientsMu.Unlock()
	}
}
