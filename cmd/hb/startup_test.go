package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/MC5lan/HonestBoy-CLI/internal/core"
	"github.com/MC5lan/HonestBoy-CLI/internal/store"
)

func seedCrash(t *testing.T, paths store.Paths, id, wd string) {
	t.Helper()
	ss := store.NewSessionStore(paths)
	sess := core.Session{ID: id, Cwd: wd, CreatedAt: time.Now().UTC()}
	if err := ss.Create(sess); err != nil {
		t.Fatal(err)
	}
	if err := ss.AppendEvent(sess.Emit(core.EvtTurnStart, core.TurnStartData{Input: "修复 Add 函数"})); err != nil {
		t.Fatal(err)
	}
	if err := ss.Unlock(); err != nil {
		t.Fatal(err)
	}
}

func TestStartupResumeEnter(t *testing.T) {
	root := t.TempDir()
	wd := t.TempDir()
	paths := store.Paths{Root: root, Sessions: filepath.Join(root, "sessions"), Logs: filepath.Join(root, "logs")}
	seedCrash(t, paths, "s_crash", wd)
	var out bytes.Buffer
	id := ""
	resumeStartupSessionIO(store.NewSessionStore(paths), wd, &id, strings.NewReader("\n"), &out)
	if id != "s_crash" {
		t.Fatalf("resume id = %q, want s_crash\n%s", id, out.String())
	}
	if !strings.Contains(out.String(), "上次任务") || !strings.Contains(out.String(), "修复 Add 函数") {
		t.Fatalf("prompt missing details:\n%s", out.String())
	}
}

func TestStartupResumeNew(t *testing.T) {
	root := t.TempDir()
	wd := t.TempDir()
	paths := store.Paths{Root: root, Sessions: filepath.Join(root, "sessions"), Logs: filepath.Join(root, "logs")}
	seedCrash(t, paths, "s_crash", wd)
	var out bytes.Buffer
	id := ""
	resumeStartupSessionIO(store.NewSessionStore(paths), wd, &id, strings.NewReader("n\n"), &out)
	if id != "" {
		t.Fatalf("new session should not resume, got %q", id)
	}
}

func TestStartupResumeListThenResume(t *testing.T) {
	root := t.TempDir()
	wd := t.TempDir()
	paths := store.Paths{Root: root, Sessions: filepath.Join(root, "sessions"), Logs: filepath.Join(root, "logs")}
	seedCrash(t, paths, "s_crash", wd)
	var out bytes.Buffer
	id := ""
	resumeStartupSessionIO(store.NewSessionStore(paths), wd, &id, strings.NewReader("s\n\n"), &out)
	if !strings.Contains(out.String(), "最近会话") || !strings.Contains(out.String(), "s_crash") {
		t.Fatalf("session list missing:\n%s", out.String())
	}
	if id != "s_crash" {
		t.Fatalf("resume id = %q, want s_crash", id)
	}
}

func TestStartupResumeSkipsWhenNewestIsDone(t *testing.T) {
	root := t.TempDir()
	wd := t.TempDir()
	paths := store.Paths{Root: root, Sessions: filepath.Join(root, "sessions"), Logs: filepath.Join(root, "logs")}
	seedCrash(t, paths, "s_crash", wd)
	// 再写一条正常结束的会话，且时间更新。
	ss := store.NewSessionStore(paths)
	sess := core.Session{ID: "s_done", Cwd: wd, CreatedAt: time.Now().UTC()}
	if err := ss.Create(sess); err != nil {
		t.Fatal(err)
	}
	_ = ss.AppendEvent(sess.Emit(core.EvtTurnStart, core.TurnStartData{Input: "新任务"}))
	_ = ss.AppendEvent(sess.Emit(core.EvtTurnEnd, core.TurnEndData{Terminal: "done"}))
	_ = ss.AppendEvent(sess.Emit(core.EvtSessionEnd, core.SessionEndData{Terminal: "done"}))
	if err := ss.Unlock(); err != nil {
		t.Fatal(err)
	}
	future := time.Now().Add(2 * time.Second)
	if err := os.Chtimes(filepath.Join(paths.Sessions, "s_done.jsonl"), future, future); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	id := ""
	resumeStartupSessionIO(store.NewSessionStore(paths), wd, &id, strings.NewReader("n\n"), &out)
	if strings.Contains(out.String(), "上次任务") || id != "" {
		t.Fatalf("done session should suppress prompt:\n%s", out.String())
	}
}

func TestStartupResumeEOFDoesNotHang(t *testing.T) {
	root := t.TempDir()
	wd := t.TempDir()
	paths := store.Paths{Root: root, Sessions: filepath.Join(root, "sessions"), Logs: filepath.Join(root, "logs")}
	seedCrash(t, paths, "s_crash", wd)
	var out bytes.Buffer
	id := ""
	resumeStartupSessionIO(store.NewSessionStore(paths), wd, &id, strings.NewReader(""), &out)
	if id != "" {
		t.Fatalf("EOF should start a new session, got %q", id)
	}
}
