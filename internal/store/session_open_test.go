package store

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/MC5lan/HonestBoy-CLI/internal/core"
)

func TestSessionRotatesAndLoadsAllParts(t *testing.T) {
	oldMax := sessionMaxBytes
	sessionMaxBytes = 240
	t.Cleanup(func() { sessionMaxBytes = oldMax })

	root := t.TempDir()
	paths := Paths{Root: root, Sessions: filepath.Join(root, "sessions"), Logs: filepath.Join(root, "logs")}
	ss := NewSessionStore(paths)
	sess := core.Session{ID: "s_rotate", Cwd: root, CreatedAt: time.Now().UTC()}
	if err := ss.Create(sess); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 12; i++ {
		ev := sess.Emit(core.EvtNotice, core.NoticeData{Level: "info", Text: strings.Repeat("x", 100)})
		if err := ss.AppendEvent(ev); err != nil {
			t.Fatal(err)
		}
	}
	if err := ss.AppendEvent(sess.Emit(core.EvtSessionEnd, core.SessionEndData{Terminal: "done"})); err != nil {
		t.Fatal(err)
	}
	if err := ss.Unlock(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(paths.Sessions, "s_rotate-2.jsonl")); err != nil {
		t.Fatalf("expected rotated part: %v", err)
	}
	loaded, err := NewSessionStore(paths).Load("s_rotate")
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded.Events) != 13 {
		t.Fatalf("events = %d, want 13", len(loaded.Events))
	}
	metas, err := NewSessionStore(paths).List()
	if err != nil {
		t.Fatal(err)
	}
	if len(metas) != 1 || !metas[0].Ended {
		t.Fatalf("metas = %+v", metas)
	}
	if err := NewSessionStore(paths).Delete("s_rotate"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(paths.Sessions, "s_rotate-2.jsonl")); !os.IsNotExist(err) {
		t.Fatalf("rotated part not deleted: %v", err)
	}
}

func TestListMarksSessionEnd(t *testing.T) {
	root := t.TempDir()
	paths := Paths{Root: root, Sessions: filepath.Join(root, "sessions"), Logs: filepath.Join(root, "logs")}
	ss := NewSessionStore(paths)
	sess := core.Session{ID: "s_end", Cwd: root, CreatedAt: time.Now().UTC()}
	if err := ss.Create(sess); err != nil {
		t.Fatal(err)
	}
	if err := ss.AppendEvent(sess.Emit(core.EvtTurnStart, core.TurnStartData{Input: "任务"})); err != nil {
		t.Fatal(err)
	}
	if err := ss.AppendEvent(sess.Emit(core.EvtSessionEnd, core.SessionEndData{Terminal: "done"})); err != nil {
		t.Fatal(err)
	}
	if err := ss.Unlock(); err != nil {
		t.Fatal(err)
	}
	metas, err := NewSessionStore(paths).List()
	if err != nil {
		t.Fatal(err)
	}
	if len(metas) != 1 || !metas[0].Ended {
		t.Fatalf("metas = %+v", metas)
	}
}

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
