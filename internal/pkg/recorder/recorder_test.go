package recorder

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestRecorder(t *testing.T) {
	// Create a temporary directory for testing
	tmpDir, err := os.MkdirTemp("", "hubterm-recorder-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	castPath := filepath.Join(tmpDir, "session.cast")

	// 1. Test creation of Recorder
	termType := "xterm-256color"
	height := 24
	width := 80

	r, err := NewRecorder(castPath, termType, height, width)
	if err != nil {
		t.Fatalf("failed to create recorder: %v", err)
	}

	// 2. Test writing data
	testOutputs := []string{
		"hello world",
		"ls -la\r\n",
		"exit\r\n",
	}

	for _, out := range testOutputs {
		if err := r.WriteData(out); err != nil {
			t.Errorf("failed to write data %q: %v", out, err)
		}
	}

	// Close the recorder to flush writes to disk
	r.Close()

	// 3. Verify file contents
	file, err := os.Open(castPath)
	if err != nil {
		t.Fatalf("failed to open recording file: %v", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)

	// Line 1: Header
	if !scanner.Scan() {
		t.Fatal("recording file is empty, missing header")
	}

	var header Header
	if err := json.Unmarshal(scanner.Bytes(), &header); err != nil {
		t.Fatalf("failed to unmarshal header: %v", err)
	}

	if header.Version != 2 {
		t.Errorf("expected asciicast version 2, got %d", header.Version)
	}
	if header.Height != height {
		t.Errorf("expected height %d, got %d", height, header.Height)
	}
	if header.Width != width {
		t.Errorf("expected width %d, got %d", width, header.Width)
	}
	if header.Env.Term != termType {
		t.Errorf("expected term type %q, got %q", termType, header.Env.Term)
	}
	if header.Env.Shell != "/bin/bash" {
		t.Errorf("expected default shell /bin/bash, got %q", header.Env.Shell)
	}

	// Lines 2+: Data frames
	var eventCount int
	for scanner.Scan() {
		line := scanner.Bytes()
		var row []interface{}
		if err := json.Unmarshal(line, &row); err != nil {
			t.Fatalf("failed to unmarshal line %d: %v", eventCount+2, err)
		}

		if len(row) != 3 {
			t.Errorf("expected row of length 3, got %d", len(row))
			continue
		}

		// Row structure: [delta, "o", data]
		delta, ok := row[0].(float64)
		if !ok || delta < 0 {
			t.Errorf("invalid delta timestamp: %v", row[0])
		}

		ioType, ok := row[1].(string)
		if !ok || ioType != "o" {
			t.Errorf("invalid I/O type (expected 'o'): %v", row[1])
		}

		content, ok := row[2].(string)
		if !ok || content != testOutputs[eventCount] {
			t.Errorf("expected content %q, got %q", testOutputs[eventCount], row[2])
		}

		eventCount++
	}

	if err := scanner.Err(); err != nil {
		t.Fatalf("scanner error: %v", err)
	}

	if eventCount != len(testOutputs) {
		t.Errorf("expected %d events, got %d", len(testOutputs), eventCount)
	}
}
