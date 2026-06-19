package log

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"
)

func TestLoggerDebug(t *testing.T) {
	t.Run("debug level output", func(t *testing.T) {
		var buf bytes.Buffer
		l := &Logger{module: "test", out: &buf}
		l.Debug("debug message")

		var entry map[string]interface{}
		if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
			t.Fatalf("expected valid JSON: %v", err)
		}
		if entry["level"] != "debug" {
			t.Errorf("expected level=debug, got %v", entry["level"])
		}
		if entry["msg"] != "debug message" {
			t.Errorf("expected msg=debug message, got %v", entry["msg"])
		}
		if entry["module"] != "test" {
			t.Errorf("expected module=test, got %v", entry["module"])
		}
	})
}

func TestLoggerInfo(t *testing.T) {
	t.Run("info level output", func(t *testing.T) {
		var buf bytes.Buffer
		l := &Logger{module: "test", out: &buf}
		l.Info("info message")

		var entry map[string]interface{}
		if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
			t.Fatalf("expected valid JSON: %v", err)
		}
		if entry["level"] != "info" {
			t.Errorf("expected level=info, got %v", entry["level"])
		}
	})
}

func TestLoggerWarn(t *testing.T) {
	t.Run("warn level output", func(t *testing.T) {
		var buf bytes.Buffer
		l := &Logger{module: "test", out: &buf}
		l.Warn("warn message")

		var entry map[string]interface{}
		if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
			t.Fatalf("expected valid JSON: %v", err)
		}
		if entry["level"] != "warn" {
			t.Errorf("expected level=warn, got %v", entry["level"])
		}
	})
}

func TestLoggerError(t *testing.T) {
	t.Run("error level output", func(t *testing.T) {
		var buf bytes.Buffer
		l := &Logger{module: "test", out: &buf}
		l.Error("error message")

		var entry map[string]interface{}
		if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
			t.Fatalf("expected valid JSON: %v", err)
		}
		if entry["level"] != "error" {
			t.Errorf("expected level=error, got %v", entry["level"])
		}
	})
}

func TestLoggerFields(t *testing.T) {
	t.Run("String field goes to extra", func(t *testing.T) {
		var buf bytes.Buffer
		l := &Logger{module: "test", out: &buf}
		l.Info("with string", String("key1", "val1"))

		var entry map[string]interface{}
		if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
			t.Fatalf("expected valid JSON: %v", err)
		}
		extra, ok := entry["extra"].(map[string]interface{})
		if !ok {
			t.Fatal("expected extra object")
		}
		if extra["key1"] != "val1" {
			t.Errorf("expected extra.key1=val1, got %v", extra["key1"])
		}
	})

	t.Run("Int field goes to extra", func(t *testing.T) {
		var buf bytes.Buffer
		l := &Logger{module: "test", out: &buf}
		l.Info("with int", Int("count", 42))

		var entry map[string]interface{}
		if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
			t.Fatalf("expected valid JSON: %v", err)
		}
		extra, ok := entry["extra"].(map[string]interface{})
		if !ok {
			t.Fatal("expected extra object")
		}
		if v, ok := extra["count"].(float64); !ok || v != 42 {
			t.Errorf("expected extra.count=42, got %v", extra["count"])
		}
	})

	t.Run("Err field", func(t *testing.T) {
		var buf bytes.Buffer
		l := &Logger{module: "test", out: &buf}
		l.Info("with error", Err(errors.New("something went wrong")))

		var entry map[string]interface{}
		if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
			t.Fatalf("expected valid JSON: %v", err)
		}
		if entry["error"] != "something went wrong" {
			t.Errorf("expected error=something went wrong, got %v", entry["error"])
		}
	})

	t.Run("known fields mapped to struct fields", func(t *testing.T) {
		var buf bytes.Buffer
		l := &Logger{module: "test", out: &buf}
		l.Info("test", String("node_id", "n1"), String("request_id", "r1"), String("username", "u1"))

		var entry map[string]interface{}
		if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
			t.Fatalf("expected valid JSON: %v", err)
		}
		if entry["node_id"] != "n1" {
			t.Errorf("expected node_id=n1, got %v", entry["node_id"])
		}
		if entry["request_id"] != "r1" {
			t.Errorf("expected request_id=r1, got %v", entry["request_id"])
		}
		if entry["username"] != "u1" {
			t.Errorf("expected username=u1, got %v", entry["username"])
		}
	})
}

func TestLoggerConcurrent(t *testing.T) {
	var buf bytes.Buffer
	l := &Logger{module: "concurrent", out: &buf}

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			l.Info("concurrent", Int("n", n))
		}(i)
	}
	wg.Wait()

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 10 {
		t.Errorf("expected 10 log lines, got %d", len(lines))
	}
	for _, line := range lines {
		var entry map[string]interface{}
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			t.Errorf("expected valid JSON line: %v", err)
		}
	}
}
