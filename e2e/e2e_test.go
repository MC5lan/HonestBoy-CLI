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
	"time"

	"github.com/MC5lan/HonestBoy-CLI/internal/core"
	"github.com/MC5lan/HonestBoy-CLI/internal/llm/testserver"
	"github.com/MC5lan/HonestBoy-CLI/internal/store"
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
			chatDelta("先执行 echo 看看。\n"),
			chatTool(0, "call_1", "shell", `{"command":"echo e2e-ok"}`),
		},
		{
			chatDelta("[SELF]\nconf: high\nmissing: none\nassumption: none\n"),
			chatDelta("echo 已经通过。\n"),
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
	t.Logf("CLI 输出:\n%s", text)
	if !strings.Contains(text, "e2e-ok") {
		t.Fatalf("tool output missing from CLI output:\n%s", text)
	}
	// 正常正文要可见；框架控制块不能污染输出；最终 summary 要显示。
	for _, want := range []string{"先执行 echo 看看。", "echo 已经通过。", "已执行 echo", "完成"} {
		if !strings.Contains(text, want) {
			t.Fatalf("human output missing %q:\n%s", want, text)
		}
	}
	if strings.Contains(text, "[SELF]") || strings.Contains(text, "conf: high") {
		t.Fatalf("control block leaked into CLI output:\n%s", text)
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

// seedCrashSession 在隔离的 LOCALAPPDATA 下写一条"进程被杀"的会话。
func seedCrashSession(t *testing.T, home, work, id string, resumed int) store.Paths {
	t.Helper()
	paths := store.Paths{
		Root:     filepath.Join(home, "HB"),
		Sessions: filepath.Join(home, "HB", "sessions"),
		Logs:     filepath.Join(home, "HB", "logs"),
	}
	ss := store.NewSessionStore(paths)
	sess := core.Session{ID: id, Cwd: work, CreatedAt: time.Now().UTC()}
	if err := ss.Create(sess); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < resumed; i++ {
		if err := ss.AppendEvent(sess.Emit(core.EvtTurnStart, core.TurnStartData{
			Input: "继续上次任务", Resumed: true, ResumeAttempt: i + 1,
		})); err != nil {
			t.Fatal(err)
		}
	}
	if err := ss.AppendEvent(sess.Emit(core.EvtTurnStart, core.TurnStartData{Input: "修复 Add 函数"})); err != nil {
		t.Fatal(err)
	}
	if err := ss.AppendEvent(sess.Emit(core.EvtToolRequest, core.ToolRequestData{
		CallID: "c1", Tool: "read_file", Args: `{"path":"a.go"}`,
	})); err != nil {
		t.Fatal(err)
	}
	if err := ss.AppendEvent(sess.Emit(core.EvtToolEnd, core.ToolEndData{
		CallID: "c1", OK: true, Output: "package main",
	})); err != nil {
		t.Fatal(err)
	}
	if err := ss.Unlock(); err != nil {
		t.Fatal(err)
	}
	return paths
}

// hb resume --last：检测强杀会话、注入原始任务与已完成工具，并记录续跑次数。
func TestBinaryResumeLast(t *testing.T) {
	skipUnlessE2E(t)
	root := moduleRoot(t)
	bin := buildBinary(t, root)
	srv := testserver.New(t, testserver.Script{Responses: [][]string{
		{chatDelta("继续完成")},
	}})
	home := t.TempDir()
	writeConfig(t, home, srv.URL)
	work := t.TempDir()
	paths := seedCrashSession(t, home, work, "s_crash", 0)

	cmd := exec.Command(bin, "resume", "--last", "--cwd", work)
	cmd.Env = append(isolatedEnv(t, root), "LOCALAPPDATA="+home)
	out, err := cmd.CombinedOutput()
	// 纯文本结果：untested（退出 3）是合法终态。
	if err != nil && exitCode(err) != 3 {
		t.Fatalf("resume failed: %v\n%s", err, out)
	}
	if n := srv.RequestCount(); n != 1 {
		t.Fatalf("model requests = %d, want 1", n)
	}
	req := srv.LastRequest()
	if req == nil {
		t.Fatal("no model request captured")
	}
	raw, _ := json.Marshal(req["messages"])
	text := string(raw)
	for _, want := range []string{
		"原始任务", "修复 Add 函数", "read_file", "已完成", "不要重复执行", "进程异常退出",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("resume request missing %q:\n%s", want, text)
		}
	}

	reloaded, err := store.NewSessionStore(paths).Load("s_crash")
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, ev := range reloaded.Events {
		if d, ok := ev.Data.(core.TurnStartData); ok && d.Resumed && d.ResumeAttempt == 1 {
			found = true
		}
	}
	if !found {
		t.Fatal("resume attempt was not recorded in the session")
	}
}

// 已续跑 2 次仍未完成时拒绝第 3 次，要求人工检查。
func TestBinaryResumeLastLimit(t *testing.T) {
	skipUnlessE2E(t)
	root := moduleRoot(t)
	bin := buildBinary(t, root)
	home := t.TempDir()
	writeConfig(t, home, "http://127.0.0.1:9")
	work := t.TempDir()
	seedCrashSession(t, home, work, "s_exhausted", 2)

	cmd := exec.Command(bin, "resume", "--last", "--cwd", work)
	cmd.Env = append(isolatedEnv(t, root), "LOCALAPPDATA="+home)
	out, err := cmd.CombinedOutput()
	if err == nil || exitCode(err) != 6 {
		t.Fatalf("exit = %v, want 6:\n%s", err, out)
	}
	if !strings.Contains(string(out), "已续跑 2 次") {
		t.Fatalf("missing resume limit message:\n%s", out)
	}
}

// hb run --auto-resume：没有新任务时自动接管最近一条未完成会话。
func TestBinaryAutoResume(t *testing.T) {
	skipUnlessE2E(t)
	root := moduleRoot(t)
	bin := buildBinary(t, root)
	srv := testserver.New(t, testserver.Script{Responses: [][]string{
		{chatDelta("自动续跑中")},
	}})
	home := t.TempDir()
	writeConfig(t, home, srv.URL)
	work := t.TempDir()
	paths := seedCrashSession(t, home, work, "s_auto", 0)

	cmd := exec.Command(bin, "run", "--auto-resume", "--cwd", work)
	cmd.Env = append(isolatedEnv(t, root), "LOCALAPPDATA="+home)
	out, err := cmd.CombinedOutput()
	if err != nil && exitCode(err) != 3 {
		t.Fatalf("auto resume failed: %v\n%s", err, out)
	}
	if n := srv.RequestCount(); n != 1 {
		t.Fatalf("model requests = %d, want 1", n)
	}
	raw, _ := json.Marshal(srv.LastRequest()["messages"])
	if !strings.Contains(string(raw), "修复 Add 函数") {
		t.Fatalf("auto resume did not carry the original task:\n%s", raw)
	}
	reloaded, err := store.NewSessionStore(paths).Load("s_auto")
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, ev := range reloaded.Events {
		if d, ok := ev.Data.(core.TurnStartData); ok && d.Resumed && d.ResumeAttempt == 1 {
			found = true
		}
	}
	if !found {
		t.Fatal("auto resume attempt was not recorded")
	}
}

// 没有未完成会话时，--auto-resume 必须给出明确提示，而不是假装成功。
func TestBinaryAutoResumeNone(t *testing.T) {
	skipUnlessE2E(t)
	root := moduleRoot(t)
	bin := buildBinary(t, root)
	home := t.TempDir()
	writeConfig(t, home, "http://127.0.0.1:9")
	work := t.TempDir()

	cmd := exec.Command(bin, "run", "--auto-resume", "--cwd", work)
	cmd.Env = append(isolatedEnv(t, root), "LOCALAPPDATA="+home)
	out, err := cmd.CombinedOutput()
	if err == nil || exitCode(err) != 6 {
		t.Fatalf("exit = %v, want 6:\n%s", err, out)
	}
	if !strings.Contains(string(out), "没有可恢复的会话") {
		t.Fatalf("missing no-resumable message:\n%s", out)
	}
}

// 断流后续写：第一段没有 [DONE]，引擎应注入续写提示并在同一回合拿到第二段。
func TestBinaryResumeAfterDisconnect(t *testing.T) {
	skipUnlessE2E(t)
	root := moduleRoot(t)
	bin := buildBinary(t, root)
	srv := testserver.New(t, testserver.Script{
		Responses: [][]string{
			{chatDelta("前半段"), chatDelta("")}, // 第二块用作断点位，不会被写出
			{chatDelta("后半段")},
		},
		DisconnectAt: 1, // 发完第一块就断开，不写 [DONE]
	})
	home := t.TempDir()
	writeConfig(t, home, srv.URL)
	work := t.TempDir()
	cmd := exec.Command(bin, "run", "--cwd", work, "续写")
	cmd.Env = append(isolatedEnv(t, root), "LOCALAPPDATA="+home)
	out, err := cmd.CombinedOutput()
	if err != nil && exitCode(err) != 3 {
		t.Fatalf("run failed: %v\n%s", err, out)
	}
	text := string(out)
	if !strings.Contains(text, "前半段") || !strings.Contains(text, "后半段") {
		t.Fatalf("resume did not stitch both halves:\n%s", text)
	}
	if !strings.Contains(text, "连接中断") {
		t.Fatalf("missing disconnect notice:\n%s", text)
	}
	if n := srv.RequestCount(); n != 2 {
		t.Fatalf("model requests = %d, want 2", n)
	}
}

// TD04 F20 真实测量：端点连不上时退出 6，且提示可执行（不是裸 dial 错误）。
func TestBinaryNetworkDownMessage(t *testing.T) {
	skipUnlessE2E(t)
	root := moduleRoot(t)
	bin := buildBinary(t, root)
	home := t.TempDir()
	writeConfig(t, home, "http://127.0.0.1:9")
	work := t.TempDir()
	cmd := exec.Command(bin, "run", "--cwd", work, "你好")
	cmd.Env = append(isolatedEnv(t, root), "LOCALAPPDATA="+home)
	out, err := cmd.CombinedOutput()
	if err == nil || exitCode(err) != 6 {
		t.Fatalf("exit = %v, want 6:\n%s", err, out)
	}
	text := string(out)
	if !strings.Contains(text, "base_url") || !strings.Contains(text, "hb doctor") {
		t.Fatalf("network error not actionable:\n%s", text)
	}
	if strings.Contains(text, "dial tcp") {
		t.Fatalf("raw dial error leaked:\n%s", text)
	}
}

// TD05 E1 真实测量：10MB 单行文件被 read_file 拒绝并给出可执行提示。
func TestBinaryHugeSingleLine(t *testing.T) {
	skipUnlessE2E(t)
	root := moduleRoot(t)
	bin := buildBinary(t, root)
	srv := testserver.New(t, testserver.Script{Responses: [][]string{
		{chatTool(0, "r1", "read_file", `{"path":"one-line.txt"}`)},
		{chatDelta("读不了")},
	}})
	home := t.TempDir()
	writeConfig(t, home, srv.URL)
	work := t.TempDir()
	big := strings.Repeat("x", 10<<20)
	if err := os.WriteFile(filepath.Join(work, "one-line.txt"), []byte(big), 0o600); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(bin, "run", "--cwd", work, "读文件")
	cmd.Env = append(isolatedEnv(t, root), "LOCALAPPDATA="+home)
	out, err := cmd.CombinedOutput()
	// untested（退出 3）是合法终态：工具失败后模型没有证据，不应假装完成。
	if err != nil && exitCode(err) != 3 {
		t.Fatalf("run failed: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "单行超过 1MB") {
		t.Fatalf("huge single line not rejected with hint:\n%s", out)
	}
}

// TD04 F07 真实测量：只读文件写入失败时给出提示，不崩溃。
func TestBinaryReadOnlyFile(t *testing.T) {
	skipUnlessE2E(t)
	root := moduleRoot(t)
	bin := buildBinary(t, root)
	srv := testserver.New(t, testserver.Script{Responses: [][]string{
		{chatTool(0, "r1", "read_file", `{"path":"locked.txt"}`)},
		{chatDelta("[SELF]\nconf: high\nmissing: none\nassumption: none\n"), chatTool(0, "w1", "write_file", `{"path":"locked.txt","content":"changed"}`)},
		{chatDelta("写不进去")},
	}})
	home := t.TempDir()
	writeConfig(t, home, srv.URL)
	work := t.TempDir()
	locked := filepath.Join(work, "locked.txt")
	if err := os.WriteFile(locked, []byte("original"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(locked, 0o400); err != nil {
		t.Skipf("cannot set read-only: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(locked, 0o600) })
	cmd := exec.Command(bin, "run", "--cwd", work, "改文件")
	cmd.Env = append(isolatedEnv(t, root), "LOCALAPPDATA="+home)
	out, err := cmd.CombinedOutput()
	if err != nil && exitCode(err) != 3 {
		t.Fatalf("run failed: %v\n%s", err, out)
	}
	text := string(out)
	t.Logf("只读文件实测输出:\n%s", text)
	if !strings.Contains(text, "失败") && !strings.Contains(text, "拒绝") && !strings.Contains(text, "权限") && !strings.Contains(text, "只读") {
		t.Fatalf("read-only failure not surfaced:\n%s", text)
	}
}
