// Package e2e 提供二进制级端到端测试：真实编译 hb.exe、真实起模型服务、
// 真实跑 CLI 进程并断言退出码与输出。
//
// 默认跳过（go test ./... 保持快速）；用 scripts/e2e.ps1 或
// 设置 HB_E2E=1 后运行：
//
//	$env:HB_E2E="1"; go test ./e2e -v -count=1
package e2e

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/MC5lan/HonestBoy-CLI/internal/llm/testserver"
)

func skipUnlessE2E(t *testing.T) {
	t.Helper()
	if os.Getenv("HB_E2E") != "1" {
		t.Skip("set HB_E2E=1 to run binary-level e2e tests")
	}
}

// moduleRoot 从测试工作目录向上找 go.mod。
func moduleRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("go.mod not found")
		}
		dir = parent
	}
}

func buildBinary(t *testing.T, root string) string {
	t.Helper()
	name := "hb"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	out := filepath.Join(t.TempDir(), name)
	cmd := exec.Command("go", "build", "-o", out, "./cmd/hb")
	cmd.Dir = root
	if b, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build failed: %v\n%s", err, b)
	}
	return out
}

// isolatedEnv 把 LOCALAPPDATA/HOME 指到临时目录，避免污染真实用户数据。
func isolatedEnv(t *testing.T, root string) []string {
	t.Helper()
	home := t.TempDir()
	env := os.Environ()
	env = append(env,
		"LOCALAPPDATA="+home,
		"HOME="+home,
		"USERPROFILE="+home,
		"HB_API_KEY=sk-e2e",
	)
	return env
}

func writeConfig(t *testing.T, home, baseURL string) {
	t.Helper()
	dir := filepath.Join(home, "HB")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	cfg := map[string]any{
		"schema_version": 1,
		"model":          "e2e-model",
		"protocol":       "openai-chat",
		"base_url":       baseURL,
		"api_key_env":    "HB_API_KEY",
		"shell":          "cmd",
		"theme":          "none",
		"max_turns":      6,
		"timeout":        "60s",
		"approval":       map[string]string{"write": "auto", "exec": "auto", "net": "auto"},
	}
	raw, _ := json.MarshalIndent(cfg, "", "  ")
	if err := os.WriteFile(filepath.Join(dir, "config.json"), raw, 0o600); err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" {
		cfg["shell"] = "sh"
		raw, _ = json.MarshalIndent(cfg, "", "  ")
		_ = os.WriteFile(filepath.Join(dir, "config.json"), raw, 0o600)
	}
}

func chatDelta(text string) string {
	b, _ := json.Marshal(map[string]any{
		"choices": []any{map[string]any{
			"delta": map[string]any{"content": text}, "finish_reason": nil,
		}},
	})
	return string(b)
}

func chatTool(index int, id, name, args string) string {
	tc := map[string]any{"index": index}
	if id != "" {
		tc["id"] = id
	}
	if name != "" || args != "" {
		fn := map[string]any{}
		if name != "" {
			fn["name"] = name
		}
		if args != "" {
			fn["arguments"] = args
		}
		tc["function"] = fn
	}
	b, _ := json.Marshal(map[string]any{
		"choices": []any{map[string]any{
			"delta": map[string]any{"tool_calls": []any{tc}},
		}},
	})
	return string(b)
}

// TestBinaryRunHappyPath 走完整链路：CLI 进程 → 引擎 → 模型 HTTP → shell 工具 → 终态 done。
func TestBinaryRunHappyPath(t *testing.T) {
	skipUnlessE2E(t)
	root := moduleRoot(t)
	bin := buildBinary(t, root)

	srv := testserver.New(t, testserver.Script{Responses: [][]string{
		{
			chatDelta("[SELF]\nconf: high\nmissing: none\nassumption: none\n"),
			chatTool(0, "call_1", "shell", `{"command":"echo e2e-ok"}`),
		},
		{
			chatDelta("[SELF]\nconf: high\nmissing: none\nassumption: none\n"),
			chatTool(0, "call_2", "request_done", `{"summary":"已执行 echo"}`),
		},
	}})

	home := t.TempDir()
	writeConfig(t, home, srv.URL)
	work := t.TempDir()
	if err := os.WriteFile(filepath.Join(work, "README.md"), []byte("# e2e\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	cmd := exec.Command(bin, "run", "--cwd", work, "--timeout", "60s", "跑一条 echo")
	cmd.Env = isolatedEnv(t, root)
	cmd.Env = append(cmd.Env, "LOCALAPPDATA="+home)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("hb run failed: %v\n%s", err, out)
	}
	text := string(out)
	if !strings.Contains(text, "e2e-ok") {
		t.Fatalf("tool output missing from CLI output:\n%s", text)
	}
	if !strings.Contains(text, "done") && !strings.Contains(text, "完成") {
		t.Fatalf("terminal not reported:\n%s", text)
	}
	if n := srv.RequestCount(); n != 2 {
		t.Fatalf("model requests = %d, want 2", n)
	}
}

// TestBinaryOfflineCommands 验证无需模型的命令与退出码契约。
func TestBinaryOfflineCommands(t *testing.T) {
	skipUnlessE2E(t)
	root := moduleRoot(t)
	bin := buildBinary(t, root)
	home := t.TempDir()

	run := func(args ...string) (string, int) {
		cmd := exec.Command(bin, args...)
		cmd.Env = isolatedEnv(t, root)
		cmd.Env = append(cmd.Env, "LOCALAPPDATA="+home)
		out, err := cmd.CombinedOutput()
		code := 0
		if ee, ok := err.(*exec.ExitError); ok {
			code = ee.ExitCode()
		} else if err != nil {
			t.Fatalf("%v: %v", args, err)
		}
		return string(out), code
	}

	if out, code := run("--version"); code != 0 || !strings.HasPrefix(strings.TrimSpace(out), "hb ") {
		t.Fatalf("--version: code=%d out=%q", code, out)
	}
	if out, code := run("explain", "HB-NET-001"); code != 0 || !strings.Contains(out, "HB-NET-001") {
		t.Fatalf("explain: code=%d out=%q", code, out)
	}
	// 未知错误码必须是用法错误（非 0），而不是假装成功。
	if _, code := run("explain", "HB-NOT-A-CODE"); code == 0 {
		t.Fatal("unknown error code should exit non-zero")
	}
	if _, code := run("gc", "--dry-run"); code != 0 {
		t.Fatalf("gc --dry-run should succeed, code=%d", code)
	}
}

// TestBinaryNoKeyExit 验证无配置/无密钥时退出码 6，不尝试假跑。
func TestBinaryNoKeyExit(t *testing.T) {
	skipUnlessE2E(t)
	root := moduleRoot(t)
	bin := buildBinary(t, root)
	home := t.TempDir()
	work := t.TempDir()
	env := isolatedEnv(t, root)
	filtered := env[:0]
	for _, e := range env {
		if !strings.HasPrefix(e, "HB_API_KEY=") {
			filtered = append(filtered, e)
		}
	}
	cmd := exec.Command(bin, "run", "--cwd", work, "你好")
	cmd.Env = append(filtered, "LOCALAPPDATA="+home)
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("no key must fail:\n%s", out)
	}
	if code := exitCode(err); code != 6 {
		t.Fatalf("exit=%d want 6:\n%s", code, out)
	}
}

// TestBinaryDeniedExit 验证无证据完成申请被 Gate 拒绝，退出码 4。
func TestBinaryDeniedExit(t *testing.T) {
	skipUnlessE2E(t)
	root := moduleRoot(t)
	bin := buildBinary(t, root)
	srv := testserver.New(t, testserver.Script{Responses: [][]string{
		{
			chatDelta("[SELF]\nconf: high\nmissing: none\nassumption: none\n"),
			chatTool(0, "w1", "write_file", `{"path":"a.go","content":"package main\n"}`),
		},
		{chatTool(0, "done1", "request_done", `{"summary":"完成了"}`)},
		{chatTool(0, "done2", "request_done", `{"summary":"完成了"}`)},
		{chatTool(0, "done3", "request_done", `{"summary":"完成了"}`)},
	}})
	home := t.TempDir()
	writeConfig(t, home, srv.URL)
	work := t.TempDir()
	cmd := exec.Command(bin, "run", "--cwd", work, "--max-turns", "6", "直接完成")
	cmd.Env = append(isolatedEnv(t, root), "LOCALAPPDATA="+home)
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("denied turn must fail:\n%s", out)
	}
	if code := exitCode(err); code != 4 {
		t.Fatalf("exit=%d want 4:\n%s", code, out)
	}
}

func exitCode(err error) int {
	if ee, ok := err.(*exec.ExitError); ok {
		return ee.ExitCode()
	}
	return -1
}

// TestBinaryUntestedExit 验证没有证据时终态是 untested 且退出码非 0。
func TestBinaryUntestedExit(t *testing.T) {
	skipUnlessE2E(t)
	root := moduleRoot(t)
	bin := buildBinary(t, root)

	srv := testserver.New(t, testserver.Script{Responses: [][]string{
		{chatDelta("这是答案，但没有运行任何工具")},
	}})
	home := t.TempDir()
	writeConfig(t, home, srv.URL)
	work := t.TempDir()

	cmd := exec.Command(bin, "run", "--cwd", work, "只说一句话")
	cmd.Env = isolatedEnv(t, root)
	cmd.Env = append(cmd.Env, "LOCALAPPDATA="+home)
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("untested turn must exit non-zero:\n%s", out)
	}
	text := string(out)
	if !strings.Contains(text, "untested") && !strings.Contains(text, "未验证") {
		t.Fatalf("untested terminal not reported:\n%s", text)
	}
}
