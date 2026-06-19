// Package recorder provides session recording in asciicast v2 format.
// Recordings capture terminal output with timing information for later
// playback and audit review.
//
// Reference: Next Terminal server/common/term/recorder.go
package recorder

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Env describes the terminal environment for the recording header.
type Env struct {
	Shell string `json:"SHELL"`
	Term  string `json:"TERM"`
}

// Header is the first line of an asciicast v2 file.
type Header struct {
	Title     string `json:"title"`
	Version   int    `json:"version"`
	Height    int    `json:"height"`
	Width     int    `json:"width"`
	Env       Env    `json:"env"`
	Timestamp int64  `json:"timestamp"`
}

// Recorder writes terminal session output to a file in asciicast v2 format.
type Recorder struct {
	File      *os.File
	Timestamp int64 // start timestamp (Unix seconds)
}

// NewRecorder creates a new recorder and writes the asciicast v2 header.
//
// Parameters:
//   - path: file path for the recording (e.g., "/recordings/session-abc.cast")
//   - termType: terminal type (e.g., "xterm-256color")
//   - height: terminal height in rows
//   - width: terminal width in columns
//
// Returns:
//   - *Recorder: ready-to-use recorder
//   - error: nil on success
//
// Error locations:
//   - os.MkdirAll: parent directory creation failure
//   - os.Create: file creation failure
//   - recorder.WriteHeader: header write failure
func NewRecorder(path, termType string, height, width int) (*Recorder, error) {
	parentDir := filepath.Dir(path)
	if err := os.MkdirAll(parentDir, 0755); err != nil {
		return nil, fmt.Errorf("create recording directory %s: %w", parentDir, err)
	}

	file, err := os.Create(path)
	if err != nil {
		return nil, fmt.Errorf("create recording file %s: %w", path, err)
	}

	now := time.Now().Unix()
	r := &Recorder{
		File:      file,
		Timestamp: now,
	}

	header := &Header{
		Title:     "",
		Version:   2,
		Height:    height,
		Width:     width,
		Env:       Env{Shell: "/bin/bash", Term: termType},
		Timestamp: now,
	}

	if err := r.writeHeader(header); err != nil {
		file.Close()
		return nil, fmt.Errorf("write recording header: %w", err)
	}

	return r, nil
}

// WriteData writes a terminal output event to the recording file.
// Each event is a JSON array: [delta_seconds, "o", "output_data"].
//
// Parameters:
//   - data: terminal output string
//
// Returns:
//   - error: nil on success, or file write error
func (r *Recorder) WriteData(data string) error {
	if r.File == nil {
		return nil
	}

	now := time.Now().UnixNano()
	startNs := r.Timestamp * 1e9
	delta := float64(now-startNs) / 1e9

	row := []interface{}{delta, "o", data}
	s, err := json.Marshal(row)
	if err != nil {
		return fmt.Errorf("marshal recording event: %w", err)
	}

	if _, err := fmt.Fprintln(r.File, string(s)); err != nil {
		return fmt.Errorf("write recording event: %w", err)
	}
	return nil
}

// Close flushes and closes the recording file.
func (r *Recorder) Close() {
	if r.File != nil {
		r.File.Close()
		r.File = nil
	}
}

// writeHeader writes the asciicast v2 header as the first line of the file.
func (r *Recorder) writeHeader(header *Header) error {
	data, err := json.Marshal(header)
	if err != nil {
		return err
	}
	if _, err := fmt.Fprintln(r.File, string(data)); err != nil {
		return err
	}
	return nil
}
