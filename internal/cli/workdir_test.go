package cli

import (
	"path/filepath"
	"testing"
)

func TestResolveWorkdir(t *testing.T) {
	dir := t.TempDir()
	got, err := ResolveWorkdir("", dir)
	if err != nil || got != dir {
		t.Fatalf("config workdir = %q, %v", got, err)
	}
	flagDir := t.TempDir()
	got, err = ResolveWorkdir(flagDir, dir)
	if err != nil || got != flagDir {
		t.Fatalf("flag workdir = %q, %v", got, err)
	}
	if _, err := ResolveWorkdir("", filepath.Join(dir, "missing")); err == nil {
		t.Fatal("missing configured workdir should fail")
	}
	if _, err := ResolveWorkdir(filepath.Join(dir, "missing"), ""); err == nil {
		t.Fatal("missing --cwd should fail")
	}
}
