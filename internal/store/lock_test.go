package store

import (
	"path/filepath"
	"testing"

	"github.com/MC5lan/HonestBoy-CLI/internal/core"
)

func TestSessionLockContention(t *testing.T) {
	root := t.TempDir()
	paths := Paths{Root: root, Sessions: filepath.Join(root, "sessions"), Logs: filepath.Join(root, "logs")}
	a := NewSessionStore(paths)
	if err := a.Create(core.Session{ID: "s_lock"}); err != nil {
		t.Fatal(err)
	}
	defer a.Unlock()

	b := NewSessionStore(paths)
	if err := b.Lock("s_lock"); err != ErrSessionLocked {
		t.Fatalf("second lock = %v, want ErrSessionLocked", err)
	}
	if err := a.Unlock(); err != nil {
		t.Fatal(err)
	}
	if err := b.Lock("s_lock"); err != nil {
		t.Fatalf("lock after release: %v", err)
	}
	b.Unlock()
}
