package script

import (
	"strings"
	"testing"
	"time"
)

func TestExecuteSimple(t *testing.T) {
	engine := NewEngine()
	script := &Script{
		Name:   "test",
		Source: "print('hello world')",
	}
	result, err := engine.Execute(script, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Stdout != "hello world\n" {
		t.Fatalf("expected 'hello world\\n', got %q", result.Stdout)
	}
	if result.ExitCode != 0 {
		t.Fatalf("expected exit code 0, got %d", result.ExitCode)
	}
	if result.Duration <= 0 {
		t.Fatalf("expected positive duration, got %d", result.Duration)
	}
}

func TestExecuteWithParams(t *testing.T) {
	engine := NewEngine()
	script := &Script{
		Name: "test-params",
		Params: []Param{
			{Name: "name", Type: "string", Required: true},
		},
		Source: "import sys\nname = sys.argv[1] if len(sys.argv) > 1 else 'world'\nprint(f'hello {name}')",
	}
	result, err := engine.Execute(script, map[string]string{"name": "hubterm"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result.Stdout, "hello hubterm") {
		t.Fatalf("expected stdout to contain 'hello hubterm', got %q", result.Stdout)
	}
	if result.ExitCode != 0 {
		t.Fatalf("expected exit code 0, got %d", result.ExitCode)
	}
}

func TestExecuteTimeout(t *testing.T) {
	engine := &Engine{PythonPath: "python3", Timeout: 1 * time.Second}
	script := &Script{
		Name:   "timeout",
		Source: "import time; time.sleep(10); print('done')",
	}
	_, err := engine.Execute(script, nil)
	if err == nil {
		t.Fatal("expected timeout error, got nil")
	}
	if !strings.Contains(err.Error(), "timed out") {
		t.Fatalf("expected error to contain 'timed out', got %q", err.Error())
	}
}

func TestExecuteNilScript(t *testing.T) {
	engine := NewEngine()
	_, err := engine.Execute(nil, nil)
	if err == nil {
		t.Fatal("expected error for nil script, got nil")
	}
}

func TestExecuteExitCodeNonZero(t *testing.T) {
	engine := NewEngine()
	script := &Script{
		Name:   "exit-error",
		Source: "import sys; sys.exit(42)",
	}
	result, err := engine.Execute(script, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ExitCode != 42 {
		t.Fatalf("expected exit code 42, got %d", result.ExitCode)
	}
}

func TestValidateValid(t *testing.T) {
	engine := NewEngine()
	err := engine.Validate("print('hello')")
	if err != nil {
		t.Fatalf("expected no error for valid syntax, got: %v", err)
	}
}

func TestValidateInvalid(t *testing.T) {
	engine := NewEngine()
	err := engine.Validate("print('hello'")
	if err == nil {
		t.Fatal("expected error for invalid syntax, got nil")
	}
}

func TestValidateEmpty(t *testing.T) {
	engine := NewEngine()
	err := engine.Validate("")
	if err == nil {
		t.Fatal("expected error for empty source, got nil")
	}
}

func TestResolveParams(t *testing.T) {
	source := "print('hello ${name}')"
	params := map[string]string{"name": "world"}
	result := resolveParams(source, params)
	if result != "print('hello world')" {
		t.Fatalf("expected 'print('hello world')', got %q", result)
	}
}

func TestResolveParamsMultiple(t *testing.T) {
	source := "${greeting} ${target}"
	params := map[string]string{"greeting": "hello", "target": "world"}
	result := resolveParams(source, params)
	if result != "hello world" {
		t.Fatalf("expected 'hello world', got %q", result)
	}
}

func TestResolveParamsNoMatch(t *testing.T) {
	source := "print('hello ${name}')"
	result := resolveParams(source, nil)
	if result != source {
		t.Fatalf("expected unchanged source, got %q", result)
	}
}

func TestExecuteWithPlaceholderParams(t *testing.T) {
	engine := NewEngine()
	script := &Script{
		Name:   "test-placeholder",
		Source: "name = '${name}'\nprint(f'hello {name}')",
	}
	result, err := engine.Execute(script, map[string]string{"name": "hubterm"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result.Stdout, "hello hubterm") {
		t.Fatalf("expected stdout to contain 'hello hubterm', got %q", result.Stdout)
	}
}

func TestExecuteShell(t *testing.T) {
	engine := NewEngine()
	script := &Script{
		Name:     "test-shell",
		Language: "shell",
		Source:   "echo hello world",
	}
	result, err := engine.Execute(script, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result.Stdout, "hello world") {
		t.Fatalf("expected stdout to contain 'hello world', got %q", result.Stdout)
	}
	if result.ExitCode != 0 {
		t.Fatalf("expected exit code 0, got %d", result.ExitCode)
	}
}

func TestExecuteOnNodeUnimplemented(t *testing.T) {
	engine := NewEngine()
	_, err := engine.ExecuteOnNode(nil, nil, "node-1")
	if err == nil {
		t.Fatal("expected error for unimplemented ExecuteOnNode, got nil")
	}
}
