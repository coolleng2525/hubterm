package discovery

import (
	"errors"
	"net"
	"os"
	"testing"
	"time"
)

// TestDiscoverWithConfig verifies that a pre-configured center URL is returned
// directly without any network discovery.
func TestDiscoverWithConfig(t *testing.T) {
	result, err := DiscoverWithConfig("http://localhost:8080")
	if err != nil {
		t.Fatalf("DiscoverWithConfig failed: %v", err)
	}
	if result.CenterURL != "http://localhost:8080" {
		t.Errorf("expected center URL 'http://localhost:8080', got '%s'", result.CenterURL)
	}
	if result.Method != "config" {
		t.Errorf("expected method 'config', got '%s'", result.Method)
	}
}

// TestDiscoverWithConfigEmpty verifies that an empty URL returns an error.
func TestDiscoverWithConfigEmpty(t *testing.T) {
	_, err := DiscoverWithConfig("")
	if err == nil {
		t.Fatal("expected error for empty URL, got nil")
	}
}

// TestDiscoverWithEnv verifies that the HUBTERM_CENTER_URL environment variable
// is used when no domain is specified.
func TestDiscoverWithEnv(t *testing.T) {
	os.Setenv("HUBTERM_CENTER_URL", "http://test:8080")
	defer os.Unsetenv("HUBTERM_CENTER_URL")

	result, err := Discover("")
	if err != nil {
		t.Fatalf("Discover failed: %v", err)
	}
	if result.CenterURL != "http://test:8080" {
		t.Errorf("expected center URL 'http://test:8080', got '%s'", result.CenterURL)
	}
	if result.Method != "env" {
		t.Errorf("expected method 'env', got '%s'", result.Method)
	}
}

// TestDiscoverNoDomainNoEnv verifies that Discover returns an error when
// no domain is given and HUBTERM_CENTER_URL is not set.
func TestDiscoverNoDomainNoEnv(t *testing.T) {
	os.Unsetenv("HUBTERM_CENTER_URL")
	_, err := Discover("")
	if err == nil {
		t.Fatal("expected error when no domain and no env var, got nil")
	}
}

// TestDiscoverWithTimeoutSuccess verifies that DiscoverWithTimeout returns
// successfully when the underlying discovery completes in time.
func TestDiscoverWithTimeoutSuccess(t *testing.T) {
	os.Setenv("HUBTERM_CENTER_URL", "http://timeout-test:8080")
	defer os.Unsetenv("HUBTERM_CENTER_URL")

	result, err := DiscoverWithTimeout("", 5*time.Second)
	if err != nil {
		t.Fatalf("DiscoverWithTimeout failed: %v", err)
	}
	if result.CenterURL != "http://timeout-test:8080" {
		t.Errorf("expected center URL 'http://timeout-test:8080', got '%s'", result.CenterURL)
	}
}

// TestDiscoverFallbackEnv verifies that when DNS discovery fails (non-existent
// domain), the function falls back to the environment variable.
func TestDiscoverFallbackEnv(t *testing.T) {
	os.Setenv("HUBTERM_CENTER_URL", "http://fallback:8080")
	defer os.Unsetenv("HUBTERM_CENTER_URL")

	originalLookupSRV, originalLookupHost := lookupSRV, lookupHost
	lookupSRV = func(string, string, string) (string, []*net.SRV, error) {
		return "", nil, errors.New("test DNS failure")
	}
	lookupHost = func(string) ([]string, error) {
		return nil, errors.New("test DNS failure")
	}
	defer func() {
		lookupSRV, lookupHost = originalLookupSRV, originalLookupHost
	}()

	result, err := Discover("nonexistent-domain-xyz.invalid")
	if err != nil {
		t.Fatalf("Discover failed: %v", err)
	}
	if result.CenterURL != "http://fallback:8080" {
		t.Errorf("expected center URL 'http://fallback:8080', got '%s'", result.CenterURL)
	}
	if result.Method != "env" {
		t.Errorf("expected method 'env', got '%s'", result.Method)
	}
}

// TestDiscoverWithTimeoutExpiry verifies that DiscoverWithTimeout returns an error
// when the discovery times out.
func TestDiscoverWithTimeoutExpiry(t *testing.T) {
	os.Unsetenv("HUBTERM_CENTER_URL")

	// Use a very short timeout to trigger a timeout before DNS completes.
	_, err := DiscoverWithTimeout("this-domain-definitely-does-not-exist-123456.test", 1*time.Nanosecond)
	if err == nil {
		t.Fatal("expected timeout error, got nil")
	}
}

// TestDiscoverResultFields verifies that the DiscoveryResult struct fields
// are populated correctly.
func TestDiscoverResultFields(t *testing.T) {
	result := &DiscoveryResult{
		CenterURL: "http://example.com:8080",
		Domain:    "example.com",
		Method:    "dns_srv",
		NodeToken: "abc123",
	}
	if result.CenterURL != "http://example.com:8080" {
		t.Errorf("CenterURL field mismatch")
	}
	if result.Domain != "example.com" {
		t.Errorf("Domain field mismatch")
	}
	if result.Method != "dns_srv" {
		t.Errorf("Method field mismatch")
	}
	if result.NodeToken != "abc123" {
		t.Errorf("NodeToken field mismatch")
	}
}
