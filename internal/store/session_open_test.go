package store

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/MC5lan/HonestBoy-CLI/internal/core"
)

// Open 必须锁定并加载已有会话，续写时 seq 继续递增、不重写 meta。
func TestOpenAppendsToExistingSession(t *testing.T) {
	root := t.TempDir()
	paths := Paths{Root: root, Sessions: filepath.Join(root, "sessions"), Logs: filepath.Join(root, "logs")}
	ss := NewSessionStore(paths)
	sess := core.Session{ID: "s_test", Cwd: root, CreatedAt: time.Now().UTC()}
	if err := ss.Create(sess); err != nil {
		t.Fatal(err)
	}
	if err := ss.AppendEvent(sess.Emit(core.EvtTurnStart, core.TurnStartData{Input: "第一轮"})); err != nil {
		t.Fatal(err)
	}
	if err := ss.Unlock(); err != nil {
		t.Fatal(err)
	}

	ss2 := NewSessionStore(paths)
	loaded, err := ss2.Open("s_test")
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded.Events) != 1 || loaded.Seq() != 1 {
		t.Fatalf("loaded events=%d seq=%d", len(loaded.Events), loaded.Seq())
	}
	ev := loaded.Emit(core.EvtTurnStart, core.TurnStartData{Input: "第二轮"})
	if ev.Seq != 2 {
		t.Fatalf("seq = %d, want 2", ev.Seq)
	}
	if err := ss2.AppendEvent(ev); err != nil {
		t.Fatal(err)
	}
	if err := ss2.Unlock(); err != nil {
		t.Fatal(err)
	}

	reloaded, err := NewSessionStore(paths).Load("s_test")
	if err != nil {
		t.Fatal(err)
	}
	if len(reloaded.Events) != 2 || reloaded.Events[1].Seq != 2 {
		t.Fatalf("reloaded events=%d", len(reloaded.Events))
	}
}
