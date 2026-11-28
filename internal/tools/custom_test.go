package tools

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestCustomToolLoadRegisterRun(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "tools.d")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	var command string
	var args []string
	if runtime.GOOS == "windows" {
		command, args = "cmd", []string{"/c", "echo", "{text}"}
	} else {
		command, args = "sh", []string{"-c", "echo {text}"}
	}
	spec := map[string]any{
		"name": "echo_custom", "description": "echo", "command": command, "args": args,
		"risk": "read", "schema": map[string]any{
			"type": "object", "properties": map[string]any{"text": map[string]any{"type": "string"}},
			"required": []string{"text"},
		},
	}
	raw, _ := json.Marshal(spec)
	if err := os.WriteFile(filepath.Join(dir, "echo.json"), raw, 0o600); err != nil {
		t.Fatal(err)
	}
	list, err := LoadCustomTools(dir, root)
	if err != nil || len(list) != 1 {
		t.Fatalf("load: %v %d", err, len(list))
	}
	reg := NewRegistry()
	if err := reg.Register(list[0]); err != nil {
		t.Fatal(err)
	}
	argsJSON, _ := json.Marshal(map[string]string{"text": "hello"})
	res, err := list[0].Run(context.Background(), argsJSON)
	if err != nil {
		t.Fatal(err)
	}
	if !res.OK || res.Output == "" {
		t.Fatalf("result = %+v", res)
	}
}

func TestCustomToolRejectsBadRisk(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "tools.d")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "bad.json"), []byte(`{"name":"bad","command":"x","risk":"root"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadCustomTools(dir, root); err == nil {
		t.Fatal("bad risk should fail")
	}
}
