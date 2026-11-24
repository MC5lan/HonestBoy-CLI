package store

import (
	"fmt"
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

// TD05 E7：强杀留下的半行只出现在末片末尾，Load 应恢复到最后一个完整行。
func TestLoadRecoversTruncatedTail(t *testing.T) {
	root := t.TempDir()
	paths := Paths{Root: root, Sessions: filepath.Join(root, "sessions"), Logs: filepath.Join(root, "logs")}
	ss := NewSessionStore(paths)
	sess := core.Session{ID: "s_tail", Cwd: root, CreatedAt: time.Now().UTC()}
	if err := ss.Create(sess); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 3; i++ {
		if err := ss.AppendEvent(sess.Emit(core.EvtNotice, core.NoticeData{Level: "info", Text: fmt.Sprintf("e%d", i)})); err != nil {
			t.Fatal(err)
		}
	}
	if err := ss.Unlock(); err != nil {
		t.Fatal(err)
	}
	// 模拟写到一半崩了：最后一行只有前缀，没有闭合 JSON。
	f, err := os.OpenFile(filepath.Join(paths.Sessions, "s_tail.jsonl"), os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = f.WriteString(`{"v":1,"kind":"event","event":{"type":"notice",`)
	_ = f.Close()

	loader := NewSessionStore(paths)
	loaded, err := loader.Load("s_tail")
	if err != nil {
		t.Fatalf("truncated tail should be skipped: %v", err)
	}
	if len(loaded.Events) != 3 {
		t.Fatalf("events = %d, want 3 (last full line)", len(loaded.Events))
	}
	if !loader.RecoveredTail {
		t.Fatal("RecoveredTail should be set so callers can warn the user")
	}
}

// 截断恢复不能放宽到"任意坏行都跳过"：中间行损坏必须报错。
func TestLoadRejectsCorruptMiddleLine(t *testing.T) {
	root := t.TempDir()
	paths := Paths{Root: root, Sessions: filepath.Join(root, "sessions"), Logs: filepath.Join(root, "logs")}
	ss := NewSessionStore(paths)
	sess := core.Session{ID: "s_mid", Cwd: root, CreatedAt: time.Now().UTC()}
	if err := ss.Create(sess); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 3; i++ {
		if err := ss.AppendEvent(sess.Emit(core.EvtNotice, core.NoticeData{Level: "info", Text: fmt.Sprintf("e%d", i)})); err != nil {
			t.Fatal(err)
		}
	}
	_ = ss.Unlock()
	path := filepath.Join(paths.Sessions, "s_mid.jsonl")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(string(raw), "\n")
	lines[2] = `{"v":1,"kind":"event","event":` // 中间行损坏，后面还有完整行
	if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := NewSessionStore(paths).Load("s_mid"); err == nil {
		t.Fatal("corrupt middle line must not be silently skipped")
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
