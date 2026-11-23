package tools

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestShellCommand(t *testing.T) {
	sb, _ := newTempSandbox(t)
	tool := NewShellTool(sb, "cmd")
	r, err := tool.Run(context.Background(), mustArgs(t, map[string]any{"command": "echo hello"}))
	if err != nil || !r.OK || !strings.Contains(r.Output, "hello") {
		t.Fatalf("shell: %+v %v", r, err)
	}
	if r.Meta["exit_code"] != "0" {
		t.Fatalf("exit code meta = %v", r.Meta)
	}
}

func TestShellNonZeroExit(t *testing.T) {
	sb, _ := newTempSandbox(t)
	cmd := "exit 3"
	r, err := NewShellTool(sb, "cmd").Run(context.Background(), mustArgs(t, map[string]any{"command": cmd}))
	if err != nil || r.OK || r.Meta["exit_code"] != "3" {
		t.Fatalf("nonzero: %+v %v", r, err)
	}
}

func TestShellTimeout(t *testing.T) {
	// 被 kill 的子进程会短暂持有 cwd 句柄，放在非 t.TempDir 目录避免清理失败。
	dir := filepath.Join(os.TempDir(), "hb-timeout-workdir")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	sb, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer sb.Close()
	cmd := "sleep 30"
	if runtime.GOOS == "windows" {
		cmd = "ping -n 30 127.0.0.1"
	}
	start := time.Now()
	r, err := NewShellTool(sb, "cmd").Run(context.Background(), mustArgs(t, map[string]any{
		"command": cmd, "timeout_ms": 1000,
	}))
	if err != nil || r.OK || !strings.Contains(r.Output, "timeout") {
		t.Fatalf("timeout: %+v %v", r, err)
	}
	if time.Since(start) > 10*time.Second {
		t.Fatal("timeout took too long")
	}
}

func TestTruncateOutput(t *testing.T) {
	data := strings.Repeat("line\n", 300000) // 1.5MB
	out, truncated := truncateOutput([]byte(data))
	if !truncated || !strings.Contains(out, "省略") {
		t.Fatalf("truncate failed: len=%d", len(out))
	}
	short, truncated := truncateOutput([]byte("ok\n"))
	if truncated || short != "ok\n" {
		t.Fatalf("short output changed: %q %v", short, truncated)
	}
}

func TestFilteredEnvExcludesKey(t *testing.T) {
	t.Setenv("HB_API_KEY", "sk-should-not-leak")
	for _, kv := range filteredEnv() {
		if strings.Contains(kv, "HB_API_KEY") || strings.Contains(kv, "sk-should-not-leak") {
			t.Fatalf("key leaked into child env: %s", kv)
		}
	}
}
