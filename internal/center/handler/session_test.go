package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/coolleng2525/hubterm/internal/center/model"
)

type fakeSessionCommander struct {
	commands []string
}

func (f *fakeSessionCommander) SendControlCommand(nodeID, commandType, sessionID string) (string, error) {
	f.commands = append(f.commands, nodeID+":"+commandType+":"+sessionID)
	return "cmd-test", nil
}

func TestKickSession(t *testing.T) {
	os.Setenv("JWT_SECRET", "test-secret-key-for-testing")
	defer os.Unsetenv("JWT_SECRET")

	db := setupTestDB(t)
	commander := &fakeSessionCommander{}
	handler := &SessionHandler{DB: db, AgentWS: commander}

	// Seed a session
	db.Create(&model.Session{
		SessionID: "sess-kick-001",
		NodeID:    "node-001",
		PortName:  "/dev/ttyUSB0",
		User:      "admin",
		Type:      "master",
	})

	t.Run("kick existing session", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("POST", "/api/sessions/sess-kick-001/kick", nil)
		c.Params = []gin.Param{{Key: "id", Value: "sess-kick-001"}}
		c.Set("username", "admin")

		handler.Kick(c)

		if w.Code != http.StatusAccepted {
			t.Errorf("expected 202, got %d: %s", w.Code, w.Body.String())
		}

		var resp map[string]interface{}
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("failed to parse response: %v", err)
		}
		if resp["success"] != true {
			t.Error("expected success=true")
		}

		// Verify session was deleted
		var count int64
		db.Model(&model.Session{}).Where("session_id = ?", "sess-kick-001").Count(&count)
		if count != 0 {
			t.Errorf("expected session to be deleted, count=%d", count)
		}
		if len(commander.commands) != 1 || commander.commands[0] != "node-001:kick_session:sess-kick-001" {
			t.Errorf("unexpected agent commands: %v", commander.commands)
		}
	})

	t.Run("kick nonexistent session returns 404", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("POST", "/api/sessions/nonexistent/kick", nil)
		c.Params = []gin.Param{{Key: "id", Value: "nonexistent"}}
		c.Set("username", "admin")

		handler.Kick(c)

		if w.Code != http.StatusNotFound {
			t.Errorf("expected 404, got %d", w.Code)
		}
	})
}

func TestAssignMaster(t *testing.T) {
	os.Setenv("JWT_SECRET", "test-secret-key-for-testing")
	defer os.Unsetenv("JWT_SECRET")

	db := setupTestDB(t)
	commander := &fakeSessionCommander{}
	handler := &SessionHandler{DB: db, AgentWS: commander}

	// Seed sessions: one master and one watcher on same port
	db.Create(&model.Session{
		SessionID: "sess-master-001",
		NodeID:    "node-001",
		PortName:  "/dev/ttyUSB0",
		User:      "admin",
		Type:      "master",
	})
	db.Create(&model.Session{
		SessionID: "sess-watcher-001",
		NodeID:    "node-001",
		PortName:  "/dev/ttyUSB0",
		User:      "viewer",
		Type:      "watcher",
	})

	t.Run("assign master to watcher session", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("POST", "/api/sessions/sess-watcher-001/assign-master", nil)
		c.Params = []gin.Param{{Key: "id", Value: "sess-watcher-001"}}
		c.Set("username", "admin")

		handler.AssignMaster(c)

		if w.Code != http.StatusAccepted {
			t.Errorf("expected 202, got %d: %s", w.Code, w.Body.String())
		}

		var resp map[string]interface{}
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("failed to parse response: %v", err)
		}
		if resp["success"] != true {
			t.Error("expected success=true")
		}

		// Verify old master was demoted to watcher
		var oldMaster model.Session
		db.Where("session_id = ?", "sess-master-001").First(&oldMaster)
		if oldMaster.Type != "watcher" {
			t.Errorf("expected old master type=watcher, got %s", oldMaster.Type)
		}

		// Verify new master was promoted
		var newMaster model.Session
		db.Where("session_id = ?", "sess-watcher-001").First(&newMaster)
		if newMaster.Type != "master" {
			t.Errorf("expected new master type=master, got %s", newMaster.Type)
		}
		if len(commander.commands) != 1 || commander.commands[0] != "node-001:assign_master:sess-watcher-001" {
			t.Errorf("unexpected agent commands: %v", commander.commands)
		}
	})

	t.Run("assign master to nonexistent session returns 404", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("POST", "/api/sessions/nonexistent/assign-master", nil)
		c.Params = []gin.Param{{Key: "id", Value: "nonexistent"}}
		c.Set("username", "admin")

		handler.AssignMaster(c)

		if w.Code != http.StatusNotFound {
			t.Errorf("expected 404, got %d", w.Code)
		}
	})
}

func TestRenameSession(t *testing.T) {
	db := setupTestDB(t)
	handler := &SessionHandler{DB: db}

	db.Create(&model.Session{
		SessionID: "sess-rename-001",
		NodeID:    "node-001",
		PortName:  "/dev/ttyUSB0",
		User:      "admin",
		Type:      "master",
	})

	t.Run("rename existing session", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		body := bytes.NewBufferString(`{"display_name":"客户A交换机"}`)
		c.Request = httptest.NewRequest("PUT", "/api/sessions/sess-rename-001/rename", body)
		c.Request.Header.Set("Content-Type", "application/json")
		c.Params = []gin.Param{{Key: "id", Value: "sess-rename-001"}}
		c.Set("username", "admin")

		handler.Rename(c)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
		}

		var session model.Session
		if err := db.Where("session_id = ?", "sess-rename-001").First(&session).Error; err != nil {
			t.Fatalf("failed to reload session: %v", err)
		}
		if session.DisplayName != "客户A交换机" {
			t.Errorf("expected display name updated, got %q", session.DisplayName)
		}
	})

	t.Run("trim and clear display name", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		body := bytes.NewBufferString(`{"display_name":"   "}`)
		c.Request = httptest.NewRequest("PUT", "/api/sessions/sess-rename-001/rename", body)
		c.Request.Header.Set("Content-Type", "application/json")
		c.Params = []gin.Param{{Key: "id", Value: "sess-rename-001"}}
		c.Set("username", "admin")

		handler.Rename(c)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
		}

		var session model.Session
		db.Where("session_id = ?", "sess-rename-001").First(&session)
		if session.DisplayName != "" {
			t.Errorf("expected display name cleared, got %q", session.DisplayName)
		}
	})
}
