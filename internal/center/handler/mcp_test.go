package handler

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/coolleng2525/hubterm/internal/center/model"
)

func TestMCPToolsIncludeQuickSend(t *testing.T) {
	tools := mcpTools()
	foundList := false
	foundRun := false
	for _, tool := range tools {
		name, _ := tool["name"].(string)
		if name == "hubterm_list_quick_sends" {
			foundList = true
		}
		if name == "hubterm_run_quick_send" {
			foundRun = true
		}
	}
	if !foundList || !foundRun {
		t.Fatalf("expected quick send tools, list=%v run=%v", foundList, foundRun)
	}
}

func TestMCPListQuickSends(t *testing.T) {
	db := setupTestDB(t)
	handler := NewMCPHandler(db, nil, nil)
	if err := db.Create(&model.Script{ScriptID: "script-r770", Name: "r770 show", Description: "show r770 status", Language: "shell", Source: "show version"}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.Script{ScriptID: "script-other", Name: "other", Description: "skip", Language: "shell", Source: "noop"}).Error; err != nil {
		t.Fatal(err)
	}

	result, err := handler.toolListQuickSends(json.RawMessage(`{"search":"r770","include_source":true}`))
	if err != nil {
		t.Fatalf("toolListQuickSends failed: %v", err)
	}
	payload, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	text := string(payload)
	if !strings.Contains(text, "script-r770") || !strings.Contains(text, "show version") {
		t.Fatalf("expected matching quick send with source, got %s", text)
	}
	if strings.Contains(text, "script-other") {
		t.Fatalf("unexpected non-matching quick send: %s", text)
	}
}

func TestQuickSendTerminalChunks(t *testing.T) {
	shellChunks := quickSendTerminalChunks("show version\nshow interfaces", "shell")
	if len(shellChunks) != 2 || shellChunks[0] != "show version\r" || shellChunks[1] != "show interfaces\r" {
		t.Fatalf("unexpected shell chunks: %#v", shellChunks)
	}

	pythonChunks := quickSendTerminalChunks("print('ok')", "python")
	if len(pythonChunks) != 1 {
		t.Fatalf("expected one python chunk, got %#v", pythonChunks)
	}
	chunk := pythonChunks[0]
	if !strings.Contains(chunk, "cat << '") || !strings.Contains(chunk, "python3 /tmp/hubterm_mcp_") || !strings.Contains(chunk, "print('ok')") || !strings.Contains(chunk, "rm -f /tmp/hubterm_mcp_") {
		t.Fatalf("unexpected python quick send command: %s", chunk)
	}
}
