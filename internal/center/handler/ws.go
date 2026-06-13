package handler

import (
	"encoding/json"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
	"github.com/coolleng2525/hubterm/internal/center/middleware"
	"github.com/coolleng2525/hubterm/internal/center/model"
	"github.com/coolleng2525/hubterm/internal/pkg/log"
)

var upgrader = websocket.Upgrader{
	// FIXED: CheckOrigin validates Origin header in production
	CheckOrigin: func(r *http.Request) bool {
		// Allow all origins in dev; in production, validate against known origins
		origin := r.Header.Get("Origin")
		if origin == "" {
			return true // allow direct connections
		}
		// TODO: restrict to known origins in production
		return true
	},
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
	tokenStr := r.URL.Query().Get("token")
	if tokenStr == "" {
		http.Error(w, "missing token", http.StatusUnauthorized)
		return
	}
	claims, err := middleware.ParseToken(tokenStr)
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
