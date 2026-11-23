package engine

import (
	"os"
	"path/filepath"
	"testing"
)

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
