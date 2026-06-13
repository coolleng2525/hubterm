package repair

import (
	"errors"
	"sync"
	"testing"
	"time"
)

func TestAttemptSuccess(t *testing.T) {
	action := Action{
		Name: "test_success",
		Run: func() error {
			return nil
		},
	}
	err := Attempt(action, 3)
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}

func TestAttemptRetry(t *testing.T) {
	var attempts int
	var mu sync.Mutex

	action := Action{
		Name: "test_retry",
		Run: func() error {
			mu.Lock()
			attempts++
			count := attempts
			mu.Unlock()
			if count < 2 {
				return errors.New("not ready yet")
			}
			return nil
		},
	}

	start := time.Now()
	err := Attempt(action, 3)
	duration := time.Since(start)

	if err != nil {
		t.Errorf("expected no error after retry, got %v", err)
	}
	if attempts < 2 {
		t.Errorf("expected at least 2 attempts, got %d", attempts)
	}
	// First retry sleeps 1s (2^0)
	if duration < 1*time.Second {
		t.Errorf("expected backoff delay >= 1s, got %v", duration)
	}
}

func TestAttemptMaxRetries(t *testing.T) {
	action := Action{
		Name: "test_max_retries",
		Run: func() error {
			return errors.New("always fails")
		},
	}

	start := time.Now()
	err := Attempt(action, 3)
	duration := time.Since(start)

	if err == nil {
		t.Fatal("expected error, got nil")
	}
	// Backoff: 1s + 2s = 3s total for 3 retries
	if duration < 3*time.Second {
		t.Errorf("expected at least 3s total backoff, got %v", duration)
	}
}

func TestAttemptSingleRetry(t *testing.T) {
	action := Action{
		Name: "test_single",
		Run: func() error {
			return errors.New("fail once")
		},
	}

	err := Attempt(action, 1)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}
