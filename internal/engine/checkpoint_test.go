package engine

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCheckpointSkipsHeavyDirs(t *testing.T) {
	dir := t.TempDir()
	for _, sub := range []string{"AppData/Local/x", "node_modules/pkg", ".ssh", "src"} {
		if err := os.MkdirAll(filepath.Join(dir, filepath.FromSlash(sub)), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	for _, f := range []string{"a.txt", "AppData/Local/x/a.txt", "node_modules/pkg/a.js", ".ssh/id_rsa", "src/main.go"} {
		if err := os.WriteFile(filepath.Join(dir, filepath.FromSlash(f)), []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	sessionDir := filepath.Join(t.TempDir(), "s1")
	cp, err := CreateCheckpoint(dir, sessionDir, "t1", "test")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"a.txt", "src/main.go"} {
		if _, ok := cp.Files[want]; !ok {
			t.Fatalf("%s missing from checkpoint", want)
		}
	}
	for _, bad := range []string{"AppData/Local/x/a.txt", "node_modules/pkg/a.js", ".ssh/id_rsa"} {
		if _, ok := cp.Files[bad]; ok {
			t.Fatalf("%s should be ignored", bad)
		}
	}
}

func TestCheckpointTruncates(t *testing.T) {
	dir := t.TempDir()
	for i := 0; i <= maxCheckpointFiles; i++ {
		name := filepath.Join(dir, "f"+itoaTest(i)+".txt")
		if err := os.WriteFile(name, []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	cp, err := CreateCheckpoint(dir, filepath.Join(t.TempDir(), "s1"), "t1", "test")
	if err != nil {
		t.Fatal(err)
	}
	if !cp.Truncated || len(cp.Files) > maxCheckpointFiles {
		t.Fatalf("truncated=%v files=%d", cp.Truncated, len(cp.Files))
	}
}

func itoaTest(n int) string {
	if n == 0 {
		return "0"
	}
	var b [12]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}

func TestCheckpointRoundTrip(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("original"), 0o600); err != nil {
		t.Fatal(err)
	}
	sessionDir := filepath.Join(dir, "sessions", "s1")
	cp, err := CreateCheckpoint(dir, sessionDir, "t1", "改文件")
	if err != nil {
		t.Fatal(err)
	}
	if cp.ID == "" || len(cp.Files) == 0 {
		t.Fatalf("empty checkpoint: %+v", cp)
	}
	// 模拟回合：修改原文件 + 新增文件。
	_ = os.WriteFile(filepath.Join(dir, "a.txt"), []byte("broken"), 0o600)
	_ = os.WriteFile(filepath.Join(dir, "new.txt"), []byte("new"), 0o600)

	listed, err := ListCheckpoints(sessionDir)
	if err != nil || len(listed) != 1 || listed[0].ID != cp.ID {
		t.Fatalf("list = %+v, %v", listed, err)
	}
	n, err := RestoreCheckpoint(dir, cp)
	if err != nil || n == 0 {
		t.Fatalf("restore: %d %v", n, err)
	}
	data, _ := os.ReadFile(filepath.Join(dir, "a.txt"))
	if string(data) != "original" {
		t.Fatalf("restored = %q", data)
	}
	if _, err := os.Stat(filepath.Join(dir, "new.txt")); err == nil {
		t.Fatal("new file should be removed on restore")
	}
}
