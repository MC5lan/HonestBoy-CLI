package store

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/MC5lan/HonestBoy-CLI/internal/core"
)

func testPaths(root string) Paths {
	return Paths{Root: root, Sessions: filepath.Join(root, "sessions"), Logs: filepath.Join(root, "logs")}
}

// seedSession 写一条会话；resumed 控制续跑次数，end 控制是否写 session_end。
func seedSession(t *testing.T, paths Paths, id, cwd string, terminal core.Terminal, resumable bool, reason string, resumed int, end bool) {
	t.Helper()
	ss := NewSessionStore(paths)
	sess := core.Session{ID: id, Cwd: cwd, CreatedAt: time.Now().UTC()}
	if err := ss.Create(sess); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < resumed; i++ {
		if err := ss.AppendEvent(sess.Emit(core.EvtTurnStart, core.TurnStartData{
			Input: "继续", Resumed: true, ResumeAttempt: i + 1,
		})); err != nil {
			t.Fatal(err)
		}
	}
	if err := ss.AppendEvent(sess.Emit(core.EvtTurnStart, core.TurnStartData{Input: "任务 " + id})); err != nil {
		t.Fatal(err)
	}
	// terminal 为空表示进程在 turn_end 之前被强杀。
	if terminal != "" {
		if err := ss.AppendEvent(sess.Emit(core.EvtTurnEnd, core.TurnEndData{
			Terminal: string(terminal), Resumable: resumable, ResumeReason: reason,
		})); err != nil {
			t.Fatal(err)
		}
	}
	if end {
		if err := ss.AppendEvent(sess.Emit(core.EvtSessionEnd, core.SessionEndData{Terminal: string(terminal)})); err != nil {
			t.Fatal(err)
		}
	}
	if err := ss.Unlock(); err != nil {
		t.Fatal(err)
	}
}

func TestListResumableStates(t *testing.T) {
	root := t.TempDir()
	paths := testPaths(root)
	seedSession(t, paths, "s_done", root, core.TermDone, false, "", 0, true)
	seedSession(t, paths, "s_crash", root, "", false, "", 0, false)
	seedSession(t, paths, "s_timeout", root, core.TermHalt, true, "timeout", 0, true)
	seedSession(t, paths, "s_abort", root, core.TermHalt, false, "user_abort", 0, true)
	seedSession(t, paths, "s_failed", root, core.TermFailed, true, "failed", 0, true)
	// 即使数据里误标可恢复，正常终态也必须收敛为不可恢复。
	seedSession(t, paths, "s_untested", root, core.TermUntest, true, "failed", 0, true)

	// denied 回合正常结束，随后新回合被强杀：仍应可恢复。
	ss := NewSessionStore(paths)
	sess := core.Session{ID: "s_after_denied", Cwd: root, CreatedAt: time.Now().UTC()}
	if err := ss.Create(sess); err != nil {
		t.Fatal(err)
	}
	_ = ss.AppendEvent(sess.Emit(core.EvtTurnStart, core.TurnStartData{Input: "第一轮"}))
	_ = ss.AppendEvent(sess.Emit(core.EvtTurnEnd, core.TurnEndData{Terminal: string(core.TermDenied)}))
	_ = ss.AppendEvent(sess.Emit(core.EvtTurnStart, core.TurnStartData{Input: "第二轮"}))
	if err := ss.Unlock(); err != nil {
		t.Fatal(err)
	}

	metas, err := NewSessionStore(paths).List()
	if err != nil {
		t.Fatal(err)
	}
	byID := map[string]SessionMeta{}
	for _, m := range metas {
		byID[m.ID] = m
	}
	cases := []struct {
		id        string
		resumable bool
		reason    string
	}{
		{"s_done", false, ""},
		{"s_crash", true, "crash"},
		{"s_timeout", true, "timeout"},
		{"s_abort", false, "user_abort"},
		{"s_failed", true, "failed"},
		{"s_untested", false, ""},
		{"s_after_denied", true, "crash"},
	}
	for _, tc := range cases {
		m, ok := byID[tc.id]
		if !ok {
			t.Fatalf("session %s missing", tc.id)
		}
		if m.Resumable != tc.resumable || m.ResumeReason != tc.reason {
			t.Fatalf("%s resumable=%v reason=%q, want %v/%q",
				tc.id, m.Resumable, m.ResumeReason, tc.resumable, tc.reason)
		}
	}
}

func TestLatestResumableFiltersWorkdirAndCountsAttempts(t *testing.T) {
	root := t.TempDir()
	paths := testPaths(root)
	other := filepath.Join(root, "other")
	seedSession(t, paths, "s_old", root, core.TermFailed, true, "failed", 0, true)
	seedSession(t, paths, "s_other", other, core.TermFailed, true, "failed", 0, true)
	seedSession(t, paths, "s_attempts", root, core.TermFailed, true, "failed", 2, true)

	// 让 s_attempts 成为当前工作区最新的一条。
	latest := filepath.Join(paths.Sessions, "s_attempts.jsonl")
	future := time.Now().Add(2 * time.Second)
	if err := os.Chtimes(latest, future, future); err != nil {
		t.Fatal(err)
	}
	m, err := NewSessionStore(paths).LatestResumable(root)
	if err != nil {
		t.Fatal(err)
	}
	if m.ID != "s_attempts" {
		t.Fatalf("latest = %s, want s_attempts", m.ID)
	}
	if m.ResumeAttempts != 2 {
		t.Fatalf("resume attempts = %d, want 2", m.ResumeAttempts)
	}

	// 其它工作区只返回自己的可恢复会话。
	m, err = NewSessionStore(paths).LatestResumable(other)
	if err != nil || m.ID != "s_other" {
		t.Fatalf("other workdir = %+v, %v", m, err)
	}
}

// 启动提示只看“最近一条会话”；如果最新一条已经正常结束，旧的崩溃会话不反复打扰。
func TestLatestForWorkdirPrefersNewest(t *testing.T) {
	root := t.TempDir()
	paths := testPaths(root)
	seedSession(t, paths, "s_crash", root, "", false, "", 0, false)
	seedSession(t, paths, "s_done", root, core.TermDone, false, "", 0, true)
	future := time.Now().Add(2 * time.Second)
	if err := os.Chtimes(filepath.Join(paths.Sessions, "s_done.jsonl"), future, future); err != nil {
		t.Fatal(err)
	}
	m, ok := NewSessionStore(paths).LatestForWorkdir(root)
	if !ok || m.ID != "s_done" {
		t.Fatalf("latest = %+v, %v; want s_done", m, ok)
	}
	// 显式 resume --last 仍能找到旧的崩溃会话。
	r, err := NewSessionStore(paths).LatestResumable(root)
	if err != nil || r.ID != "s_crash" {
		t.Fatalf("resumable = %+v, %v; want s_crash", r, err)
	}
}

func TestLatestResumableNone(t *testing.T) {
	root := t.TempDir()
	paths := testPaths(root)
	seedSession(t, paths, "s_done", root, core.TermDone, false, "", 0, true)
	if _, err := NewSessionStore(paths).LatestResumable(root); !errors.Is(err, ErrNoResumable) {
		t.Fatalf("err = %v, want ErrNoResumable", err)
	}
}
