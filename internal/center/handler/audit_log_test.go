package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/coolleng2525/hubterm/internal/center/model"
)

func TestListAuditLogs(t *testing.T) {
	os.Setenv("JWT_SECRET", "test-secret-key-for-testing")
	defer os.Unsetenv("JWT_SECRET")

	db := setupTestDB(t)
	handler := &AuditLogHandler{DB: db}

	// Seed audit logs
	logs := []model.AuditLog{
		{User: "admin", Action: "login", Target: "admin", Detail: "Admin logged in"},
		{User: "admin", Action: "command", Target: "node-001", Detail: "ls -la"},
		{User: "operator", Action: "login", Target: "operator", Detail: "Operator logged in"},
		{User: "admin", Action: "kick_session", Target: "sess-001", Detail: "Kicked session"},
	}
	for _, l := range logs {
		db.Create(&l)
	}

	t.Run("list all audit logs", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("GET", "/api/audit-logs", nil)

		handler.List(c)

		if w.Code != http.StatusOK {
			t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
		}

		var resp map[string]interface{}
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("failed to parse response: %v", err)
		}

		total, ok := resp["total"].(float64)
		if !ok || int(total) != 4 {
			t.Errorf("expected total=4, got %v", total)
		}

		logsResp, ok := resp["logs"].([]interface{})
		if !ok || len(logsResp) != 4 {
			t.Errorf("expected 4 logs, got %d", len(logsResp))
		}
	})

	t.Run("filter by action", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("GET", "/api/audit-logs?action=login", nil)

		handler.List(c)

		if w.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", w.Code)
		}

		var resp map[string]interface{}
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("failed to parse response: %v", err)
		}

		total, ok := resp["total"].(float64)
		if !ok || int(total) != 2 {
			t.Errorf("expected total=2, got %v", total)
		}
	})

	t.Run("pagination", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("GET", "/api/audit-logs?page=1&page_size=2", nil)

		handler.List(c)

		if w.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", w.Code)
		}

		var resp map[string]interface{}
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("failed to parse response: %v", err)
		}

		logsResp, ok := resp["logs"].([]interface{})
		if !ok || len(logsResp) != 2 {
			t.Errorf("expected 2 logs per page, got %d", len(logsResp))
		}
	})
}

func TestFilterByAction(t *testing.T) {
	os.Setenv("JWT_SECRET", "test-secret-key-for-testing")
	defer os.Unsetenv("JWT_SECRET")

	db := setupTestDB(t)
	handler := &AuditLogHandler{DB: db}

	// Seed logs with different actions
	db.Create(&model.AuditLog{User: "admin", Action: "login", Detail: "login"})
	db.Create(&model.AuditLog{User: "admin", Action: "command", Detail: "cmd1"})
	db.Create(&model.AuditLog{User: "admin", Action: "command", Detail: "cmd2"})
	db.Create(&model.AuditLog{User: "admin", Action: "kick_session", Detail: "kick"})

	t.Run("filter by command action", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("GET", "/api/audit-logs?action=command", nil)

		handler.List(c)

		if w.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", w.Code)
		}

		var resp map[string]interface{}
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("failed to parse response: %v", err)
		}

		total, ok := resp["total"].(float64)
		if !ok || int(total) != 2 {
			t.Errorf("expected total=2 for command action, got %v", total)
		}
	})

	t.Run("filter by nonexistent action returns empty", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("GET", "/api/audit-logs?action=nonexistent", nil)

		handler.List(c)

		if w.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", w.Code)
		}

		var resp map[string]interface{}
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("failed to parse response: %v", err)
		}

		total, ok := resp["total"].(float64)
		if !ok || int(total) != 0 {
			t.Errorf("expected total=0, got %v", total)
		}
	})
}
