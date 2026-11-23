package engine

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/MC5lan/HonestBoy-CLI/internal/core"
	"github.com/MC5lan/HonestBoy-CLI/internal/llm"
	"github.com/MC5lan/HonestBoy-CLI/internal/store"
	"github.com/MC5lan/HonestBoy-CLI/internal/tools"
)

func newTestEngineAt(t *testing.T, dir string, client llm.Client) *Engine {
	t.Helper()
	sb, err := tools.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { sb.Close() })
	// 会话数据放在工作区之外，避免被 git 当成用户改动提交。
	root := t.TempDir()
	paths := store.Paths{Root: root, Sessions: filepath.Join(root, "sessions"), Logs: filepath.Join(root, "logs")}
	eng := New(Config{
		Workdir: dir, Model: "test", Client: client,
		Registry: tools.NewDefaultRegistry(sb, tools.NewReadGuard(), "cmd"),
		Store:    store.NewSessionStore(paths), MaxTurns: 10,
	})
	t.Cleanup(func() {
		if eng.ledgerFile != nil {
			eng.ledgerFile.Close()
		}
	})
	return eng
}

func initGitWorkdir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	for _, args := range [][]string{
		{"init", "-q"}, {"config", "user.email", "t@x"}, {"config", "user.name", "T"},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		_ = cmd.Run()
	}
	_ = os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module x\n\ngo 1.27\n"), 0o600)
	_ = os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\nfunc main() {}\n"), 0o600)
	cmd := exec.Command("git", "add", "-A")
	cmd.Dir = dir
	_ = cmd.Run()
	cmd = exec.Command("git", "commit", "-qm", "init")
	cmd.Dir = dir
	_ = cmd.Run()
	return dir
}

func TestAutoCommitEmitsDiffReview(t *testing.T) {
	dir := initGitWorkdir(t)
	client := &scriptedClient{responses: [][]llm.Chunk{
		{call("w", "write_file", `{"path":"new.txt","content":"hello"}`)},
		{doneCall("d", "完成")},
	}}
	eng := newTestEngineAt(t, dir, client)
	eng.cfg.AutoCommit = true
	rs := runEngine(t, eng, "新增文件")
	if rs.terminal != core.TermDone {
		t.Fatalf("terminal = %s", rs.terminal)
	}
	if countEvents(rs, core.EvtDiffReview) != 1 {
		t.Fatal("missing diff_review event")
	}
	found := false
	for _, ev := range rs.events {
		if ev.Type == core.EvtDiffReview {
			d, ok := ev.Data.(core.DiffReviewData)
			if !ok || d.Commit == "" || len(d.Files) == 0 {
				t.Fatalf("bad diff review: %+v", ev.Data)
			}
			found = true
		}
	}
	if !found {
		t.Fatal("no diff review data")
	}
	if _, err := os.Stat(filepath.Join(dir, "new.txt")); err != nil {
		t.Fatal("file missing")
	}
}

func TestAutoCommitOffByDefault(t *testing.T) {
	dir := initGitWorkdir(t)
	client := &scriptedClient{responses: [][]llm.Chunk{
		{call("w", "write_file", `{"path":"x.txt","content":"y"}`)},
		{doneCall("d", "完成")},
	}}
	eng := newTestEngineAt(t, dir, client)
	rs := runEngine(t, eng, "任务")
	if countEvents(rs, core.EvtDiffReview) != 0 {
		t.Fatal("diff_review should not be emitted by default")
	}
}
