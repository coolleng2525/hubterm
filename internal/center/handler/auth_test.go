package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

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
