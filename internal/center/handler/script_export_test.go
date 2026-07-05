package handler

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/coolleng2525/hubterm/internal/center/model"
)

func TestScriptTarBundleIncludesPackageInfo(t *testing.T) {
	data, filename, err := buildScriptTarBundle(testPresetScripts(), true, "")
	if err != nil {
		t.Fatalf("buildScriptTarBundle: %v", err)
	}
	if strings.Contains(filename, "-enc") {
		t.Fatalf("plain filename should not include enc: %s", filename)
	}

	files := readGzipTarFiles(t, data)
	if _, ok := files["hubterm-package.json"]; !ok {
		t.Fatal("missing hubterm-package.json")
	}
	if _, ok := files["manifest.json"]; !ok {
		t.Fatal("missing manifest.json")
	}

	var info presetPackageInfo
	if err := json.Unmarshal(files["hubterm-package.json"], &info); err != nil {
		t.Fatalf("unmarshal package info: %v", err)
	}
	if info.PackageVersion == "" || info.BundleVersion == "" {
		t.Fatalf("package version info not populated: %+v", info)
	}
	if info.Encrypted {
		t.Fatal("plain package should not be marked encrypted")
	}
}

func TestEncryptedScriptTarBundleRoundTrip(t *testing.T) {
	data, filename, err := buildScriptTarBundle(testPresetScripts(), true, "secret")
	if err != nil {
		t.Fatalf("buildScriptTarBundle: %v", err)
	}
	if !strings.Contains(filename, "-enc") {
		t.Fatalf("encrypted filename should include enc: %s", filename)
	}

	files := readGzipTarFiles(t, data)
	if _, ok := files["manifest.json"]; ok {
		t.Fatal("encrypted outer package should not expose manifest.json")
	}
	if _, ok := files["payload.enc"]; !ok {
		t.Fatal("missing encrypted payload")
	}

	if _, err := parsePresetBundleFile(filename, data, ""); err == nil || !strings.Contains(err.Error(), "requires password") {
		t.Fatalf("expected password error, got %v", err)
	}

	bundle, err := parsePresetBundleFile(filename, data, "secret")
	if err != nil {
		t.Fatalf("parse encrypted bundle: %v", err)
	}
	if len(bundle.Scripts) != 1 {
		t.Fatalf("expected 1 script, got %d", len(bundle.Scripts))
	}
	if bundle.Scripts[0].Source != "echo hello" {
		t.Fatalf("unexpected source: %q", bundle.Scripts[0].Source)
	}
}

func TestReadPresetScriptFile(t *testing.T) {
	dir := t.TempDir()
	filename := filepath.Join(dir, "check.sh")
	if err := os.WriteFile(filename, []byte("echo check\n"), 0600); err != nil {
		t.Fatalf("write preset script: %v", err)
	}

	entry, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read temp dir: %v", err)
	}
	bundle, err := readPresetPath(filename, entry[0])
	if err != nil {
		t.Fatalf("readPresetPath: %v", err)
	}
	if len(bundle.Scripts) != 1 {
		t.Fatalf("expected 1 script, got %d", len(bundle.Scripts))
	}
	script := bundle.Scripts[0]
	if script.Name != "check" {
		t.Fatalf("expected name check, got %q", script.Name)
	}
	if script.Language != "shell" {
		t.Fatalf("expected shell language, got %q", script.Language)
	}
	if script.Source != "echo check\n" {
		t.Fatalf("unexpected source: %q", script.Source)
	}
}

func TestReadPresetJSONFileWithSiblingSourceFile(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "show.sh"), []byte("echo show\n"), 0600); err != nil {
		t.Fatalf("write source file: %v", err)
	}
	manifest := `{
  "version": "1.0",
  "scripts": [
    {
      "name": "show test",
      "language": "shell",
      "source_file": "show.sh"
    }
  ]
}`
	manifestPath := filepath.Join(dir, "default.json")
	if err := os.WriteFile(manifestPath, []byte(manifest), 0600); err != nil {
		t.Fatalf("write manifest: %v", err)
	}

	bundle, err := readPresetJSONFile(manifestPath)
	if err != nil {
		t.Fatalf("readPresetJSONFile: %v", err)
	}
	if len(bundle.Scripts) != 1 {
		t.Fatalf("expected 1 script, got %d", len(bundle.Scripts))
	}
	if bundle.Scripts[0].Source != "echo show\n" {
		t.Fatalf("unexpected hydrated source: %q", bundle.Scripts[0].Source)
	}
}

func testPresetScripts() []model.Script {
	return []model.Script{{
		ScriptID:    "script-1",
		Name:        "hello",
		Description: "hello script",
		Language:    "shell",
		Source:      "echo hello",
		Params:      "[]",
		Timeout:     30,
	}}
}

func readGzipTarFiles(t *testing.T, data []byte) map[string][]byte {
	t.Helper()
	gz, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("gzip reader: %v", err)
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	files := map[string][]byte{}
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("tar next: %v", err)
		}
		if header.Typeflag != tar.TypeReg {
			continue
		}
		content, err := io.ReadAll(tr)
		if err != nil {
			t.Fatalf("read %s: %v", header.Name, err)
		}
		files[header.Name] = content
	}
	return files
}
