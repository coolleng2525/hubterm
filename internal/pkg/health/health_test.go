package health

import (
	"sync"
	"testing"
)

func TestRegisterAndRun(t *testing.T) {
	// Reset global state
	mu.Lock()
	checks = nil
	mu.Unlock()

	Register("test_check", func() CheckResult {
		return CheckResult{Name: "test_check", Status: "ok"}
	})

	results := RunAll()
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Name != "test_check" {
		t.Errorf("expected name=test_check, got %s", results[0].Name)
	}
	if results[0].Status != "ok" {
		t.Errorf("expected status=ok, got %s", results[0].Status)
	}
}

func TestRegisterAndRunMultiple(t *testing.T) {
	// Reset global state
	mu.Lock()
	checks = nil
	mu.Unlock()

	Register("check_a", func() CheckResult {
		return CheckResult{Name: "check_a", Status: "ok"}
	})
	Register("check_b", func() CheckResult {
		return CheckResult{Name: "check_b", Status: "degraded", Detail: "high load"}
	})
	Register("check_c", func() CheckResult {
		return CheckResult{Name: "check_c", Status: "down", Detail: "unreachable"}
	})

	results := RunAll()
	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}

	statusMap := make(map[string]string)
	for _, r := range results {
		statusMap[r.Name] = r.Status
	}
	if statusMap["check_a"] != "ok" {
		t.Errorf("expected check_a=ok, got %s", statusMap["check_a"])
	}
	if statusMap["check_b"] != "degraded" {
		t.Errorf("expected check_b=degraded, got %s", statusMap["check_b"])
	}
	if statusMap["check_c"] != "down" {
		t.Errorf("expected check_c=down, got %s", statusMap["check_c"])
	}
}

func TestConcurrentRegister(t *testing.T) {
	// Reset global state
	mu.Lock()
	checks = nil
	mu.Unlock()

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			Register("check", func() CheckResult {
				return CheckResult{Name: "check", Status: "ok"}
			})
		}(i)
	}
	wg.Wait()

	results := RunAll()
	if len(results) != 20 {
		t.Errorf("expected 20 results, got %d", len(results))
	}
}
