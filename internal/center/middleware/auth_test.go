package middleware

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

func TestMain(m *testing.M) {
	os.Setenv("JWT_SECRET", "test-secret-key-for-testing")
	os.Exit(m.Run())
}

func TestGenerateAndParseToken(t *testing.T) {
	t.Run("generate and parse valid token", func(t *testing.T) {
		token, err := GenerateToken(1, "testuser", "admin")
		if err != nil {
			t.Fatalf("GenerateToken failed: %v", err)
		}
		if token == "" {
			t.Fatal("expected non-empty token")
		}

		claims, err := ParseToken(token)
		if err != nil {
			t.Fatalf("ParseToken failed: %v", err)
		}
		if claims.UserID != 1 {
			t.Errorf("expected UserID=1, got %d", claims.UserID)
		}
		if claims.Username != "testuser" {
			t.Errorf("expected Username=testuser, got %s", claims.Username)
		}
		if claims.Role != "admin" {
			t.Errorf("expected Role=admin, got %s", claims.Role)
		}
	})

	t.Run("parse invalid token returns error", func(t *testing.T) {
		_, err := ParseToken("invalid-token-string")
		if err == nil {
			t.Error("expected error for invalid token, got nil")
		}
	})

	t.Run("parse expired token returns error", func(t *testing.T) {
		claims := Claims{
			UserID:   1,
			Username: "testuser",
			Role:     "admin",
			RegisteredClaims: jwt.RegisteredClaims{
				ExpiresAt: jwt.NewNumericDate(time.Now().Add(-1 * time.Hour)),
				IssuedAt:  jwt.NewNumericDate(time.Now().Add(-2 * time.Hour)),
			},
		}
		token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
		tokenStr, err := token.SignedString(getJWTSecret())
		if err != nil {
			t.Fatalf("failed to sign expired token: %v", err)
		}

		_, err = ParseToken(tokenStr)
		if err == nil {
			t.Error("expected error for expired token, got nil")
		}
	})

	t.Run("parse token with wrong secret returns error", func(t *testing.T) {
		claims := Claims{
			UserID:   1,
			Username: "testuser",
			Role:     "admin",
			RegisteredClaims: jwt.RegisteredClaims{
				ExpiresAt: jwt.NewNumericDate(time.Now().Add(1 * time.Hour)),
				IssuedAt:  jwt.NewNumericDate(time.Now()),
			},
		}
		token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
		tokenStr, err := token.SignedString([]byte("wrong-secret"))
		if err != nil {
			t.Fatalf("failed to sign token: %v", err)
		}

		_, err = ParseToken(tokenStr)
		if err == nil {
			t.Error("expected error for token signed with wrong secret, got nil")
		}
	})
}

func TestRefreshToken(t *testing.T) {
	t.Run("refresh valid token", func(t *testing.T) {
		token, err := GenerateToken(1, "testuser", "operator")
		if err != nil {
			t.Fatalf("GenerateToken failed: %v", err)
		}

		newToken, err := RefreshToken(token)
		if err != nil {
			t.Fatalf("RefreshToken failed: %v", err)
		}
		if newToken == "" {
			t.Fatal("expected non-empty refreshed token")
		}

		claims, err := ParseToken(newToken)
		if err != nil {
			t.Fatalf("ParseToken on refreshed token failed: %v", err)
		}
		if claims.UserID != 1 || claims.Username != "testuser" || claims.Role != "operator" {
			t.Errorf("claims mismatch: %+v", claims)
		}
	})

	t.Run("refresh invalid token returns error", func(t *testing.T) {
		_, err := RefreshToken("invalid-token")
		if err == nil {
			t.Error("expected error for invalid token, got nil")
		}
	})
}

func TestAuthMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("valid token passes", func(t *testing.T) {
		token, _ := GenerateToken(1, "testuser", "admin")

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request, _ = http.NewRequest("GET", "/", nil)
		c.Request.Header.Set("Authorization", "Bearer "+token)

		AuthRequired()(c)

		if w.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", w.Code)
		}
		userID, _ := c.Get("user_id")
		username, _ := c.Get("username")
		role, _ := c.Get("role")
		if userID != uint(1) {
			t.Errorf("expected user_id=1, got %v", userID)
		}
		if username != "testuser" {
			t.Errorf("expected username=testuser, got %v", username)
		}
		if role != "admin" {
			t.Errorf("expected role=admin, got %v", role)
		}
	})

	t.Run("missing header returns 401", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request, _ = http.NewRequest("GET", "/", nil)

		AuthRequired()(c)

		if w.Code != http.StatusUnauthorized {
			t.Errorf("expected 401, got %d", w.Code)
		}
	})

	t.Run("invalid token returns 401", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request, _ = http.NewRequest("GET", "/", nil)
		c.Request.Header.Set("Authorization", "Bearer invalid-token")

		AuthRequired()(c)

		if w.Code != http.StatusUnauthorized {
			t.Errorf("expected 401, got %d", w.Code)
		}
	})

	t.Run("expired token returns 401", func(t *testing.T) {
		claims := Claims{
			UserID:   1,
			Username: "testuser",
			Role:     "admin",
			RegisteredClaims: jwt.RegisteredClaims{
				ExpiresAt: jwt.NewNumericDate(time.Now().Add(-1 * time.Hour)),
				IssuedAt:  jwt.NewNumericDate(time.Now().Add(-2 * time.Hour)),
			},
		}
		token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
		tokenStr, _ := token.SignedString(getJWTSecret())

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request, _ = http.NewRequest("GET", "/", nil)
		c.Request.Header.Set("Authorization", "Bearer "+tokenStr)

		AuthRequired()(c)

		if w.Code != http.StatusUnauthorized {
			t.Errorf("expected 401, got %d", w.Code)
		}
	})
}

func TestRoleMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("admin role passes", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request, _ = http.NewRequest("GET", "/", nil)
		c.Set("role", "admin")

		AdminRequired()(c)

		if w.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", w.Code)
		}
	})

	t.Run("operator role is forbidden", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request, _ = http.NewRequest("GET", "/", nil)
		c.Set("role", "operator")

		AdminRequired()(c)

		if w.Code != http.StatusForbidden {
			t.Errorf("expected 403, got %d", w.Code)
		}
	})

	t.Run("readonly role is forbidden", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request, _ = http.NewRequest("GET", "/", nil)
		c.Set("role", "readonly")

		AdminRequired()(c)

		if w.Code != http.StatusForbidden {
			t.Errorf("expected 403, got %d", w.Code)
		}
	})

	t.Run("unset role is forbidden", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request, _ = http.NewRequest("GET", "/", nil)

		AdminRequired()(c)

		if w.Code != http.StatusForbidden {
			t.Errorf("expected 403, got %d", w.Code)
		}
	})
}
