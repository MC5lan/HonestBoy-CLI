package engine

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MC5lan/HonestBoy-CLI/internal/core"
	"github.com/MC5lan/HonestBoy-CLI/internal/store"
)

// 直接验证 recoverCrash 写崩溃日志；它只依赖 Engine 的 workdir 与 session。
func TestCrashRecovery(t *testing.T) {
	dir := t.TempDir()
	eng := &Engine{
		cfg:     Config{Workdir: dir},
		session: &core.Session{ID: "s_crash"},
		store:   store.NewSessionStore(store.Paths{Root: dir, Sessions: filepath.Join(dir, "sessions"), Logs: filepath.Join(dir, "logs")}),
	}
	func() {
		defer recoverCrash(eng)
		panic("injected boom")
	}()
	crashDir := filepath.Join(dir, ".hb-crash")
	entries, err := os.ReadDir(crashDir)
	if err != nil || len(entries) == 0 {
		t.Fatalf("no crash log: %v", err)
	}
	data, _ := os.ReadFile(filepath.Join(crashDir, entries[0].Name()))
	if !strings.Contains(string(data), "injected boom") {
		t.Fatalf("crash log wrong: %s", data)
	}
	if len(eng.session.Events) == 0 || eng.session.Events[len(eng.session.Events)-1].Type != core.EvtError {
		t.Fatal("crash should emit error event")
	}
}
