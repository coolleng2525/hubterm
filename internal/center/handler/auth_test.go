package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/coolleng2525/hubterm/internal/center/middleware"
	"github.com/coolleng2525/hubterm/internal/center/model"
	"github.com/gin-gonic/gin"
)

func TestLoginSuccess(t *testing.T) {
	resetLoginAttemptsForTest()
	os.Setenv("JWT_SECRET", "test-secret-key-for-testing")
	defer os.Unsetenv("JWT_SECRET")

	db := setupTestDB(t)
	seedUser(t, db, "testuser", "correctpassword", "operator")

	handler := &AuthHandler{DB: db}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/api/auth/login", strings.NewReader(`{"username":"testuser","password":"correctpassword"}`))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.Login(c)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if resp["token"] == nil || resp["token"] == "" {
		t.Error("expected non-empty token in response")
	}
	user, ok := resp["user"].(map[string]interface{})
	if !ok {
		t.Fatal("expected user object in response")
	}
	if user["username"] != "testuser" {
		t.Errorf("expected username=testuser, got %v", user["username"])
	}
	if user["role"] != "operator" {
		t.Errorf("expected role=operator, got %v", user["role"])
	}
}

func TestLoginFail(t *testing.T) {
	resetLoginAttemptsForTest()
	os.Setenv("JWT_SECRET", "test-secret-key-for-testing")
	defer os.Unsetenv("JWT_SECRET")

	db := setupTestDB(t)
	seedUser(t, db, "testuser", "correctpassword", "operator")

	handler := &AuthHandler{DB: db}

	t.Run("wrong password returns 401", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("POST", "/api/auth/login", strings.NewReader(`{"username":"testuser","password":"wrongpassword"}`))
		c.Request.Header.Set("Content-Type", "application/json")

		handler.Login(c)

		if w.Code != http.StatusUnauthorized {
			t.Errorf("expected 401, got %d", w.Code)
		}
	})

	t.Run("nonexistent user returns 401", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("POST", "/api/auth/login", strings.NewReader(`{"username":"nobody","password":"somepassword"}`))
		c.Request.Header.Set("Content-Type", "application/json")

		handler.Login(c)

		if w.Code != http.StatusUnauthorized {
			t.Errorf("expected 401, got %d", w.Code)
		}
	})

	t.Run("invalid JSON returns 400", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("POST", "/api/auth/login", strings.NewReader(`not-json`))
		c.Request.Header.Set("Content-Type", "application/json")

		handler.Login(c)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected 400, got %d", w.Code)
		}
	})
}

func TestGenerateMCPTokenPersistsTokenHash(t *testing.T) {
	os.Setenv("JWT_SECRET", "test-secret-key-for-testing")
	defer os.Unsetenv("JWT_SECRET")

	db := setupTestDB(t)
	userID := seedUser(t, db, "mcpuser", "correctpassword", "operator")
	handler := &AuthHandler{DB: db}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/api/auth/mcp-token", strings.NewReader(`{"days":365}`))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set("user_id", userID)
	c.Set("username", "mcpuser")
	c.Set("role", "operator")

	handler.GenerateMCPToken(c)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	token, _ := resp["token"].(string)
	if token == "" {
		t.Fatal("expected token in response")
	}
	if resp["token_id"] == nil {
		t.Fatal("expected token_id in response")
	}

	claims, err := middleware.ParseToken(token)
	if err != nil {
		t.Fatalf("failed to parse token: %v", err)
	}
	if claims.TokenType != "mcp" {
		t.Fatalf("expected token_type=mcp, got %q", claims.TokenType)
	}

	var saved model.MCPToken
	if err := db.Where("token_hash = ?", middleware.TokenHash(token)).First(&saved).Error; err != nil {
		t.Fatalf("expected mcp token hash saved: %v", err)
	}
	if saved.TokenHash == token {
		t.Fatal("token plaintext must not be stored in database")
	}
	if saved.UserID != userID || saved.Username != "mcpuser" || saved.Role != "operator" {
		t.Fatalf("unexpected saved token metadata: %+v", saved)
	}
	if time.Until(saved.ExpiresAt) < 360*24*time.Hour {
		t.Fatalf("expected long-lived expiry, got %s", saved.ExpiresAt)
	}
}

func TestListAndRevokeMCPTokens(t *testing.T) {
	os.Setenv("JWT_SECRET", "test-secret-key-for-testing")
	defer os.Unsetenv("JWT_SECRET")

	db := setupTestDB(t)
	userID := seedUser(t, db, "mcpuser", "correctpassword", "operator")
	handler := &AuthHandler{DB: db}

	active := model.MCPToken{TokenHash: "hash-active", UserID: userID, Username: "mcpuser", Role: "operator", ExpiresAt: time.Now().Add(time.Hour)}
	expired := model.MCPToken{TokenHash: "hash-expired", UserID: userID, Username: "mcpuser", Role: "operator", ExpiresAt: time.Now().Add(-time.Hour)}
	other := model.MCPToken{TokenHash: "hash-other", UserID: userID + 1, Username: "other", Role: "operator", ExpiresAt: time.Now().Add(time.Hour)}
	if err := db.Create(&active).Error; err != nil {
		t.Fatalf("failed to create active token: %v", err)
	}
	if err := db.Create(&expired).Error; err != nil {
		t.Fatalf("failed to create expired token: %v", err)
	}
	if err := db.Create(&other).Error; err != nil {
		t.Fatalf("failed to create other token: %v", err)
	}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/auth/mcp-tokens", nil)
	c.Set("user_id", userID)
	c.Set("username", "mcpuser")
	c.Set("role", "operator")
	handler.ListMCPTokens(c)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var listResp struct {
		Tokens []struct {
			ID     uint   `json:"id"`
			Status string `json:"status"`
		} `json:"tokens"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &listResp); err != nil {
		t.Fatalf("failed to parse list response: %v", err)
	}
	if len(listResp.Tokens) != 2 {
		t.Fatalf("expected only current user's 2 tokens, got %d", len(listResp.Tokens))
	}

	w = httptest.NewRecorder()
	c, _ = gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/api/auth/mcp-tokens/revoke", nil)
	c.Params = gin.Params{{Key: "id", Value: fmt.Sprintf("%d", active.ID)}}
	c.Set("user_id", userID)
	c.Set("username", "mcpuser")
	c.Set("role", "operator")
	handler.RevokeMCPToken(c)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 revoke, got %d: %s", w.Code, w.Body.String())
	}
	var revoked model.MCPToken
	if err := db.First(&revoked, active.ID).Error; err != nil {
		t.Fatalf("failed to reload revoked token: %v", err)
	}
	if !revoked.Revoked {
		t.Fatal("expected token revoked")
	}
}

func TestLoginRateLimit(t *testing.T) {
	resetLoginAttemptsForTest()
	now := time.Now()
	for i := 0; i < loginRateLimitMaxAttempts; i++ {
		if !allowLoginAttempt("192.0.2.10", now) {
			t.Fatalf("attempt %d was unexpectedly denied", i+1)
		}
	}
	if allowLoginAttempt("192.0.2.10", now) {
		t.Fatal("expected login attempt to be rate limited")
	}
	if !allowLoginAttempt("192.0.2.10", now.Add(loginRateLimitWindow+time.Second)) {
		t.Fatal("expected login attempts to reset after window")
	}
}

func TestRegister(t *testing.T) {
	os.Setenv("JWT_SECRET", "test-secret-key-for-testing")
	defer os.Unsetenv("JWT_SECRET")

	db := setupTestDB(t)
	handler := &AuthHandler{DB: db}

	t.Run("register new user", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("POST", "/api/auth/register", strings.NewReader(`{"username":"newuser","password":"newpass","role":"operator"}`))
		c.Request.Header.Set("Content-Type", "application/json")

		handler.Register(c)

		if w.Code != http.StatusCreated {
			t.Errorf("expected 201, got %d: %s", w.Code, w.Body.String())
		}

		var resp map[string]interface{}
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("failed to parse response: %v", err)
		}
		if resp["username"] != "newuser" {
			t.Errorf("expected username=newuser, got %v", resp["username"])
		}
	})

	t.Run("register duplicate username returns 409", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("POST", "/api/auth/register", strings.NewReader(`{"username":"newuser","password":"anotherpass"}`))
		c.Request.Header.Set("Content-Type", "application/json")

		handler.Register(c)

		if w.Code != http.StatusConflict {
			t.Errorf("expected 409, got %d", w.Code)
		}
	})

	t.Run("register with default role", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("POST", "/api/auth/register", strings.NewReader(`{"username":"defaultrole","password":"pass"}`))
		c.Request.Header.Set("Content-Type", "application/json")

		handler.Register(c)

		if w.Code != http.StatusCreated {
			t.Errorf("expected 201, got %d", w.Code)
		}

		var resp map[string]interface{}
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("failed to parse response: %v", err)
		}
		if resp["role"] != "operator" {
			t.Errorf("expected default role=operator, got %v", resp["role"])
		}
	})
}

func resetLoginAttemptsForTest() {
	loginAttemptsMu.Lock()
	defer loginAttemptsMu.Unlock()
	loginAttempts = make(map[string]loginAttemptBucket)
}
