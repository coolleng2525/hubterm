package reporter

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestReport(t *testing.T) {
	// Create a mock center server
	mux := http.NewServeMux()
	mux.HandleFunc("/api/nodes/report", func(w http.ResponseWriter, r *http.Request) {
		// Verify auth header
		if r.Header.Get("Authorization") != "Bearer test-token" {
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]string{"error": "invalid token"})
			return
		}

		// Verify content type
		if r.Header.Get("Content-Type") != "application/json" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		// Return success with token
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
			"token":   "new-token-from-server",
		})
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	reporter := NewReporter(server.URL, "test-node", "Test Node")
	reporter.SetNodeToken("test-token")

	err := reporter.Report()
	if err != nil {
		t.Fatalf("Report failed: %v", err)
	}

	if reporter.NodeToken != "new-token-from-server" {
		t.Errorf("expected token to be updated to 'new-token-from-server', got '%s'", reporter.NodeToken)
	}
}

func TestReportRetry(t *testing.T) {
	attempts := 0

	mux := http.NewServeMux()
	mux.HandleFunc("/api/nodes/report", func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts < 2 {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": "server error"})
			return
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
		})
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	reporter := NewReporter(server.URL, "test-node-retry", "Test Node Retry")
	reporter.SetNodeToken("test-token")

	// First attempt fails
	err := reporter.Report()
	if err == nil {
		t.Fatal("expected error on first attempt, got nil")
	}

	// Second attempt should succeed
	err = reporter.Report()
	if err != nil {
		t.Fatalf("second Report failed: %v", err)
	}

	if attempts != 2 {
		t.Errorf("expected 2 total attempts, got %d", attempts)
	}
}

func TestReportUnauthorized(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/nodes/report", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]string{"error": "invalid token"})
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	reporter := NewReporter(server.URL, "test-node-unauth", "Test Node")
	reporter.SetNodeToken("wrong-token")

	err := reporter.Report()
	if err == nil {
		t.Fatal("expected error for unauthorized request, got nil")
	}
}

func TestReportServerDown(t *testing.T) {
	// Use a server that's not running
	reporter := NewReporter("http://127.0.0.1:1", "test-node-down", "Test Node Down")
	reporter.SetNodeToken("test-token")

	err := reporter.Report()
	if err == nil {
		t.Fatal("expected error when server is down, got nil")
	}
}
