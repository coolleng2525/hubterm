// Package script provides a Python script execution engine for HubTerm.
// It supports running Python scripts locally or dispatching them to remote nodes.
package script

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"strings"
	"time"
)

const maxOutputBytes = 10 * 1024 * 1024

var envNamePattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// Script defines a script that can be executed by the engine.
type Script struct {
	// ID is the unique identifier for the script.
	ID string `json:"id"`
	// Name is the human-readable name of the script.
	Name string `json:"name"`
	// Description is a brief explanation of what the script does.
	Description string `json:"description"`
	// Language specifies the script language (e.g., "python", "shell").
	Language string `json:"language"`
	// Source is the raw source code of the script.
	Source string `json:"source"`
	// Params defines the parameters accepted by the script.
	Params []Param `json:"params,omitempty"`
	// Timeout is the maximum execution time in seconds.
	Timeout int `json:"timeout"`
	// CreatedAt is the timestamp when the script was created.
	CreatedAt time.Time `json:"created_at"`
	// UpdatedAt is the timestamp when the script was last updated.
	UpdatedAt time.Time `json:"updated_at"`
}

// Param defines a single parameter accepted by a script.
type Param struct {
	// Name is the parameter name.
	Name string `json:"name"`
	// Type is the parameter type (string, int, bool, password).
	Type string `json:"type"`
	// Required indicates whether the parameter is mandatory.
	Required bool `json:"required"`
	// Default is the default value when the parameter is not provided.
	Default string `json:"default,omitempty"`
	// Description explains the purpose of this parameter.
	Description string `json:"description,omitempty"`
}

// Result holds the output of a script execution.
type Result struct {
	// ScriptID is the identifier of the executed script.
	ScriptID string `json:"script_id"`
	// NodeID is the identifier of the node where the script ran (empty for local).
	NodeID string `json:"node_id"`
	// Stdout contains the standard output of the script.
	Stdout string `json:"stdout"`
	// Stderr contains the standard error output of the script.
	Stderr string `json:"stderr"`
	// ExitCode is the process exit code (0 for success).
	ExitCode int `json:"exit_code"`
	// Duration is the execution time in milliseconds.
	Duration int64 `json:"duration_ms"`
	// StartedAt is the Unix timestamp (milliseconds) when execution started.
	StartedAt int64 `json:"started_at"`
	// CompletedAt is the Unix timestamp (milliseconds) when execution completed.
	CompletedAt int64 `json:"completed_at"`
}

// Engine is the script execution engine that runs Python scripts locally.
type Engine struct {
	// PythonPath is the path to the Python interpreter.
	PythonPath string
	// Timeout is the default maximum execution duration.
	Timeout time.Duration
}

// NewEngine creates a new script engine with sensible defaults.
// It uses "python3" as the Python interpreter and a 30-second timeout.
func NewEngine() *Engine {
	return &Engine{
		PythonPath: defaultPythonPath(),
		Timeout:    30 * time.Second,
	}
}

func defaultPythonPath() string {
	candidates := []string{"python3", "python"}
	if runtime.GOOS == "windows" {
		candidates = []string{"python", "py"}
	}
	for _, candidate := range candidates {
		if path, err := exec.LookPath(candidate); err == nil {
			return path
		}
	}
	return candidates[0]
}

// Execute runs a script locally with the given parameters.
// It writes the script source to a temporary file, passes parameters through argv
// and environment variables, captures stdout/stderr/exit code, cleans up the temp
// file, and returns the result.
func (e *Engine) Execute(script *Script, params map[string]string) (*Result, error) {
	if script == nil {
		return nil, fmt.Errorf("script is nil")
	}

	// Determine the interpreter based on language.
	interpreter := e.PythonPath

	// Write source to a temporary file.
	tmpPattern := "hubterm-script-*"
	if script.Language == "shell" && runtime.GOOS == "windows" {
		tmpPattern = "hubterm-script-*.cmd"
	}
	tmpFile, err := os.CreateTemp("", tmpPattern)
	if err != nil {
		return nil, fmt.Errorf("failed to create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()

	if _, err := tmpFile.WriteString(script.Source); err != nil {
		tmpFile.Close()
		os.Remove(tmpPath)
		return nil, fmt.Errorf("failed to write temp file: %w", err)
	}
	tmpFile.Close()
	defer os.Remove(tmpPath)

	// Build command arguments: the temp file path followed by parameter values.
	args := []string{tmpPath}
	if script.Language == "shell" {
		if runtime.GOOS == "windows" {
			interpreter = "cmd"
			args = []string{"/c", tmpPath}
		} else {
			interpreter = "bash"
		}
	}
	for _, p := range script.Params {
		if val, ok := params[p.Name]; ok {
			args = append(args, val)
		} else if p.Default != "" {
			args = append(args, p.Default)
		} else if p.Required {
			args = append(args, "")
		}
	}

	// Determine timeout.
	timeout := e.Timeout
	if script.Timeout > 0 {
		timeout = time.Duration(script.Timeout) * time.Second
	}

	// Create a context with timeout.
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	// Prepare and execute the command.
	cmd := exec.CommandContext(ctx, interpreter, args...)
	cmd.Env = append(os.Environ(), paramEnv(script.Params, params)...)

	var stdout, stderr limitedBuffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	startedAt := time.Now()
	err = cmd.Run()
	completedAt := time.Now()
	duration := completedAt.Sub(startedAt).Milliseconds()

	// Handle timeout error.
	if err != nil && ctx.Err() == context.DeadlineExceeded {
		return &Result{
			ScriptID:    script.ID,
			Stdout:      stdout.String(),
			Stderr:      stderr.String(),
			ExitCode:    -1,
			Duration:    duration,
			StartedAt:   startedAt.UnixMilli(),
			CompletedAt: completedAt.UnixMilli(),
		}, fmt.Errorf("execution timed out after %v", timeout)
	}

	// Extract exit code.
	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			exitCode = 1
		}
	}

	return &Result{
		ScriptID:    script.ID,
		Stdout:      stdout.String(),
		Stderr:      stderr.String(),
		ExitCode:    exitCode,
		Duration:    duration,
		StartedAt:   startedAt.UnixMilli(),
		CompletedAt: completedAt.UnixMilli(),
	}, nil
}

// ExecuteOnNode dispatches a script to a remote node for execution.
// It sends the script definition and parameters via the agent's WebSocket connection.
// NOTE: This is a placeholder — full node-side execution requires agent-side script
// execution support to be implemented.
func (e *Engine) ExecuteOnNode(script *Script, params map[string]string, nodeID string) (*Result, error) {
	return nil, fmt.Errorf("remote execution not yet implemented")
}

// Validate checks the syntax of a Python script by running python3 -c with compile().
// It returns nil if the script compiles without errors, or an error describing the issue.
func (e *Engine) Validate(source string) error {
	if source == "" {
		return fmt.Errorf("empty source")
	}

	cmd := exec.Command(e.PythonPath, "-c", `
import sys
try:
    compile(sys.stdin.read(), '<hubterm>', 'exec')
except SyntaxError as e:
    print(f"SyntaxError: {e}", file=sys.stderr)
    sys.exit(1)
`)
	cmd.Stdin = strings.NewReader(source)

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return fmt.Errorf("syntax validation failed: %s", msg)
	}

	return nil
}

func paramEnv(paramDefs []Param, params map[string]string) []string {
	env := make([]string, 0, len(paramDefs)*2)
	for _, p := range paramDefs {
		value, ok := params[p.Name]
		if !ok {
			value = p.Default
		}
		if !ok && p.Default == "" && !p.Required {
			continue
		}
		if envNamePattern.MatchString(p.Name) {
			env = append(env, p.Name+"="+value)
		}
		env = append(env, "HUBTERM_PARAM_"+sanitizeEnvName(p.Name)+"="+value)
	}
	return env
}

func sanitizeEnvName(name string) string {
	if name == "" {
		return "PARAM"
	}
	var b strings.Builder
	for i, r := range name {
		if (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') || r == '_' || (i > 0 && r >= '0' && r <= '9') {
			b.WriteRune(r)
		} else {
			b.WriteByte('_')
		}
	}
	if b.Len() == 0 || !envNamePattern.MatchString(b.String()) {
		return "PARAM_" + b.String()
	}
	return b.String()
}

type limitedBuffer struct {
	buf       bytes.Buffer
	limit     int
	truncated bool
}

func (b *limitedBuffer) Write(p []byte) (int, error) {
	limit := b.limit
	if limit <= 0 {
		limit = maxOutputBytes
	}
	if b.buf.Len() < limit {
		remaining := limit - b.buf.Len()
		if len(p) <= remaining {
			_, _ = b.buf.Write(p)
		} else {
			_, _ = b.buf.Write(p[:remaining])
			b.truncated = true
		}
	} else {
		b.truncated = true
	}
	return len(p), nil
}

func (b *limitedBuffer) String() string {
	if !b.truncated {
		return b.buf.String()
	}
	return b.buf.String() + "\n[output truncated]\n"
}
