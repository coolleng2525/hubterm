// Package log provides structured JSON logging for HubTerm.
package log

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sync"
	"time"
)

// Level represents log severity.
type Level int

const (
	LevelDebug Level = iota
	LevelInfo
	LevelWarn
	LevelError
)

func (l Level) String() string {
	switch l {
	case LevelDebug:
		return "debug"
	case LevelInfo:
		return "info"
	case LevelWarn:
		return "warn"
	case LevelError:
		return "error"
	default:
		return "unknown"
	}
}

// Field is a structured log field.
type Field struct {
	Key   string
	Value interface{}
}

// String creates a string field.
func String(key, val string) Field {
	return Field{Key: key, Value: val}
}

// Int creates an int field.
func Int(key string, val int) Field {
	return Field{Key: key, Value: val}
}

// Err creates an error field.
func Err(err error) Field {
	return Field{Key: "error", Value: err.Error()}
}

// Logger provides structured JSON logging.
type Logger struct {
	module string
	out    io.Writer
	mu     sync.Mutex
}

// New creates a Logger for the given module.
func New(module string) *Logger {
	return &Logger{
		module: module,
		out:    os.Stdout,
	}
}

type logEntry struct {
	Time    string      `json:"time"`
	Level   string      `json:"level"`
	Module  string      `json:"module"`
	Msg     string      `json:"msg"`
	NodeID  string      `json:"node_id,omitempty"`
	ReqID   string      `json:"request_id,omitempty"`
	User    string      `json:"username,omitempty"`
	Error   string      `json:"error,omitempty"`
	Extra   interface{} `json:"extra,omitempty"`
}

func (l *Logger) log(level Level, msg string, fields ...Field) {
	entry := logEntry{
		Time:   time.Now().UTC().Format(time.RFC3339Nano),
		Level:  level.String(),
		Module: l.module,
		Msg:    msg,
	}
	for _, f := range fields {
		switch f.Key {
		case "node_id":
			entry.NodeID, _ = f.Value.(string)
		case "request_id":
			entry.ReqID, _ = f.Value.(string)
		case "username":
			entry.User, _ = f.Value.(string)
		case "error":
			entry.Error = fmt.Sprint(f.Value)
		default:
			if entry.Extra == nil {
				entry.Extra = make(map[string]interface{})
			}
			if m, ok := entry.Extra.(map[string]interface{}); ok {
				m[f.Key] = f.Value
			}
		}
	}

	data, _ := json.Marshal(entry)
	l.mu.Lock()
	fmt.Fprintln(l.out, string(data))
	l.mu.Unlock()
}

// Debug logs at debug level.
func (l *Logger) Debug(msg string, fields ...Field) {
	l.log(LevelDebug, msg, fields...)
}

// Info logs at info level.
func (l *Logger) Info(msg string, fields ...Field) {
	l.log(LevelInfo, msg, fields...)
}

// Warn logs at warn level.
func (l *Logger) Warn(msg string, fields ...Field) {
	l.log(LevelWarn, msg, fields...)
}

// Error logs at error level.
func (l *Logger) Error(msg string, fields ...Field) {
	l.log(LevelError, msg, fields...)
}
