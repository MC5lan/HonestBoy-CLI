package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func initRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	for _, args := range [][]string{
		{"init", "-q"},
		{"config", "user.email", "test@local"},
		{"config", "user.name", "Test"},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v %s", args, err, out)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("hello\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("git", "add", "-A")
	cmd.Dir = dir
	_ = cmd.Run()
	cmd = exec.Command("git", "commit", "-qm", "init")
	cmd.Dir = dir
	_ = cmd.Run()
	return dir
}

func TestOpenAndDirty(t *testing.T) {
	dir := initRepo(t)
	repo, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	dirty, err := repo.IsDirty()
	if err != nil || dirty {
		t.Fatalf("dirty = %v, %v", dirty, err)
	}
	_ = os.WriteFile(filepath.Join(dir, "a.txt"), []byte("changed\n"), 0o600)
	dirty, _ = repo.IsDirty()
	if !dirty {
		t.Fatal("should be dirty")
	}
	files, err := repo.ChangedFiles()
	if err != nil || len(files) != 1 || files[0] != "a.txt" {
		t.Fatalf("files = %v, %v", files, err)
	}
}

func TestCommitDiffAndUndo(t *testing.T) {
	dir := initRepo(t)
	repo, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	_ = os.WriteFile(filepath.Join(dir, "a.txt"), []byte("changed\n"), 0o600)
	sha, err := repo.Commit("hb: update a")
	if err != nil || sha == "" {
		t.Fatalf("commit = %q, %v", sha, err)
	}
	stat, err := repo.DiffStat()
	if err != nil || stat != "" {
		t.Fatalf("stat after commit = %q, %v", stat, err)
	}
	last, err := repo.LastAutoCommit()
	if err != nil {
		t.Fatal(err)
	}
	// 当前仓库 user.name 是 Test（测试初始化），不是 Honest Boy —— 自动提交用 -c 覆盖。
	if last == "" {
		t.Fatal("auto commit not found")
	}
	undone, err := repo.UndoLast()
	if err != nil || undone == "" {
		t.Fatalf("undo = %q, %v", undone, err)
	}
	dirty, _ := repo.IsDirty()
	if !dirty {
		t.Fatal("undo should keep changes in worktree")
	}
}

func TestNotRepo(t *testing.T) {
	if _, err := Open(t.TempDir()); err != ErrNotRepo {
		t.Fatalf("err = %v", err)
	}
}

func TestCommitAuthorOverride(t *testing.T) {
	dir := initRepo(t)
	repo, _ := Open(dir)
	_ = os.WriteFile(filepath.Join(dir, "b.txt"), []byte("x"), 0o600)
	_, err := repo.Commit("hb: add b")
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("git", "log", "-1", "--format=%an")
	cmd.Dir = dir
	out, _ := cmd.Output()
	if !strings.Contains(string(out), "Honest Boy") {
		t.Fatalf("author = %q", out)
	}
}
