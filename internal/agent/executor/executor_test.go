package executor

import (
	"strings"
	"testing"
	"time"
)

func TestExecute_Success(t *testing.T) {
	result, err := Execute("echo 'hello world'", 5*time.Second)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	trimmedOut := strings.TrimSpace(result.Stdout)
	if trimmedOut != "hello world" {
		t.Errorf("expected stdout 'hello world', got %q", trimmedOut)
	}

	if result.ExitCode != 0 {
		t.Errorf("expected exit code 0, got %d", result.ExitCode)
	}

	if result.Duration <= 0 {
		t.Errorf("expected positive duration, got %d", result.Duration)
	}
}

func TestExecute_Failure(t *testing.T) {
	// A command that exits with code 42
	result, err := Execute("exit 42", 5*time.Second)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.ExitCode != 42 {
		t.Errorf("expected exit code 42, got %d", result.ExitCode)
	}
}

func TestExecute_Timeout(t *testing.T) {
	// Sleep longer than the timeout threshold
	timeout := 100 * time.Millisecond
	result, err := Execute("sleep 2", timeout)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.ExitCode != -1 {
		t.Errorf("expected exit code -1 for timeout, got %d", result.ExitCode)
	}

	if !strings.Contains(result.Stderr, "timed out") {
		t.Errorf("expected stderr to contain timeout info, got %q", result.Stderr)
	}

	if time.Duration(result.Duration)*time.Millisecond < timeout {
		t.Errorf("duration %dms is less than timeout %v", result.Duration, timeout)
	}
}

func TestCleanScriptOutput(t *testing.T) {
	rawOutput := "Script started on Sun Jun 21 20:00:00 2026\nhello\x01 world\nScript done on Sun Jun 21 20:00:01 2026"
	cleaned := cleanScriptOutput(rawOutput)

	expected := "hello world"
	if cleaned != expected {
		t.Errorf("expected cleaned output %q, got %q", expected, cleaned)
	}
}
