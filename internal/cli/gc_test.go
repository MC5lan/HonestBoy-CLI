package cli

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/MC5lan/HonestBoy-CLI/internal/core"
	"github.com/MC5lan/HonestBoy-CLI/internal/store"
)

func TestGCAndExport(t *testing.T) {
	root := t.TempDir()
	t.Setenv("LOCALAPPDATA", root)
	paths := store.DefaultPaths()
	ss := store.NewSessionStore(paths)

	// 新会话
	if err := ss.Create(core.Session{ID: "s_new", Cwd: "x", CreatedAt: time.Now().UTC()}); err != nil {
		t.Fatal(err)
	}
	_ = ss.AppendEvent(core.Event{Seq: 1, SessionID: "s_new", Type: core.EvtTurnStart,
		Time: time.Now().UTC(), Data: core.TurnStartData{Input: "hi"}})
	ss.Unlock()

	// 旧会话（touch 到 100 天前）
	if err := ss.Create(core.Session{ID: "s_old", Cwd: "x", CreatedAt: time.Now().AddDate(0, 0, -100)}); err != nil {
		t.Fatal(err)
	}
	ss.Unlock()
	oldPath := filepath.Join(paths.Sessions, "s_old.jsonl")
	old := time.Now().AddDate(0, 0, -100)
	_ = os.Chtimes(oldPath, old, old)

	// dry-run 不删
	if err := RunGC(GlobalOptions{}, []string{"--dry-run"}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(oldPath); err != nil {
		t.Fatal("dry-run must not delete")
	}
	// --yes 删除旧会话，保留新会话
	if err := RunGC(GlobalOptions{Yes: true}, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(oldPath); err == nil {
		t.Fatal("old session should be deleted")
	}
	if _, err := os.Stat(filepath.Join(paths.Sessions, "s_new.jsonl")); err != nil {
		t.Fatal("new session should remain")
	}

	// export
	if err := RunExport(GlobalOptions{}, []string{"s_new"}); err != nil {
		t.Fatal(err)
	}
}

func TestGCNothingToClean(t *testing.T) {
	root := t.TempDir()
	t.Setenv("LOCALAPPDATA", root)
	if err := RunGC(GlobalOptions{}, []string{"--dry-run"}); err != nil {
		t.Fatal(err)
	}
}
