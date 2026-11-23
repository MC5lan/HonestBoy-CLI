package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
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

// 导出必须是合法 jsonl：Windows 路径的反斜杠要正确转义。
func TestExportValidJSONL(t *testing.T) {
	sess := &core.Session{
		ID:  "s1",
		Cwd: `C:\Users\张三\code`,
		Events: []core.Event{{
			Seq: 1, SessionID: "s1", Type: core.EvtTurnStart, Time: time.Now().UTC(),
			Data: core.TurnStartData{Input: "hi"},
		}},
	}
	var buf bytes.Buffer
	if err := writeExport(&buf, sess, false); err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("lines = %d", len(lines))
	}
	for _, line := range lines {
		if !json.Valid([]byte(line)) {
			t.Fatalf("invalid json line: %s", line)
		}
	}
	if !strings.Contains(lines[0], "C:\\\\Users") {
		t.Fatalf("windows path not escaped: %s", lines[0])
	}
}

// --redact 必须遮蔽密钥样式的内容，且仍是合法 JSON。
func TestExportRedacts(t *testing.T) {
	sess := &core.Session{
		ID: "s2", Cwd: "/tmp/x",
		Events: []core.Event{{
			Seq: 1, SessionID: "s2", Type: core.EvtNotice, Time: time.Now().UTC(),
			Data: core.NoticeData{Level: "info", Text: "key sk-abcdefghijklmnopqrstuvwxyz012345"},
		}},
	}
	var buf bytes.Buffer
	if err := writeExport(&buf, sess, true); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(buf.String(), "sk-abcdefghijklmnopqrstuvwxyz012345") {
		t.Fatal("secret leaked in redacted export")
	}
	for _, line := range strings.Split(strings.TrimSpace(buf.String()), "\n") {
		if !json.Valid([]byte(line)) {
			t.Fatalf("invalid json after redaction: %s", line)
		}
	}
}
