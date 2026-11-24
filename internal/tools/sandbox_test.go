package tools

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func newTempSandbox(t *testing.T) (*Sandbox, string) {
	t.Helper()
	dir := t.TempDir()
	sb, err := Open(dir)
	if err != nil {
		t.Fatalf("open sandbox: %v", err)
	}
	t.Cleanup(func() { sb.Close() })
	return sb, dir
}

func TestSandboxEscapeCases(t *testing.T) {
	sb, dir := newTempSandbox(t)
	outside := filepath.Join(filepath.Dir(dir), "outside.txt")
	if err := os.WriteFile(outside, []byte("secret"), 0o600); err != nil {
		t.Skipf("cannot create outside file: %v", err)
	}
	cases := []struct {
		name string
		path string
	}{
		{"relative-parent", "../../etc/passwd"},
		{"absolute-outside", outside},
		{"url-encoded-slash", "..%2f..%2fetc"},
		{"url-encoded-backslash", "..%5c..%5cetc"},
		{"windows-short-name", "PROGRA~1"},
		{"dotdot-exact", ".."},
		{"null-byte", "a\x00b"},
	}
	if runtime.GOOS == "windows" {
		cases = append(cases,
			struct{ name, path string }{"drive-relative", `C:..\..\windows`},
			struct{ name, path string }{"windows-absolute", `C:\Windows\win.ini`},
			struct{ name, path string }{"windows-case-variant", `C:\WINDOWS\win.ini`},
			struct{ name, path string }{"unc-path", `\\localhost\C$\Windows\win.ini`},
			struct{ name, path string }{"long-path-prefix", `\\?\` + outside},
		)
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := sb.Rel(tc.path); err == nil {
				t.Fatalf("path %q should be rejected", tc.path)
			}
		})
	}
}

func TestSandboxMethodErrorBranches(t *testing.T) {
	sb, dir := newTempSandbox(t)
	if _, err := Open(filepath.Join(dir, "missing")); err == nil {
		t.Fatal("Open(missing) should fail")
	}
	if _, err := Open(filepath.Join(dir, "file.txt")); err == nil {
		_ = os.WriteFile(filepath.Join(dir, "file.txt"), []byte("x"), 0o600)
		if _, err2 := Open(filepath.Join(dir, "file.txt")); err2 == nil {
			t.Fatal("Open(file) should fail")
		}
	}
	escape := "../outside"
	if _, err := sb.OpenFile(escape); err == nil {
		t.Fatal("OpenFile escape")
	}
	if _, err := sb.Create(escape); err == nil {
		t.Fatal("Create escape")
	}
	if _, err := sb.OpenFileRW(escape); err == nil {
		t.Fatal("OpenFileRW escape")
	}
	if _, err := sb.Stat(escape); err == nil {
		t.Fatal("Stat escape")
	}
	if err := sb.Remove(escape); err == nil {
		t.Fatal("Remove escape")
	}
	if err := sb.Rename(escape, "x"); err == nil {
		t.Fatal("Rename old escape")
	}
	if err := sb.Rename("x", escape); err == nil {
		t.Fatal("Rename new escape")
	}
}

func TestSandboxNormalPaths(t *testing.T) {
	sb, dir := newTempSandbox(t)
	cases := map[string]string{
		"a.go":        "a.go",
		"dir/b.go":    filepath.Join("dir", "b.go"),
		"./a.go":      "a.go",
		"dir/../a.go": "a.go",
	}
	for in, want := range cases {
		got, err := sb.Rel(in)
		if err != nil || got != want {
			t.Fatalf("Rel(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
	abs := filepath.Join(dir, "x.go")
	if got, err := sb.Rel(abs); err != nil || got != "x.go" {
		t.Fatalf("Rel(abs) = %q, %v", got, err)
	}
}

func TestSandboxFileRoundTrip(t *testing.T) {
	sb, _ := newTempSandbox(t)
	if err := os.WriteFile(filepath.Join(sb.Workdir(), "a.txt"), []byte("hello"), 0o600); err != nil {
		t.Fatal(err)
	}
	data, err := sb.ReadFile("a.txt")
	if err != nil || string(data) != "hello" {
		t.Fatalf("data=%q err=%v", data, err)
	}
	f, err := sb.OpenFile("a.txt")
	if err != nil {
		t.Fatal(err)
	}
	f.Close()
	rw, err := sb.OpenFileRW("a.txt")
	if err != nil {
		t.Fatal(err)
	}
	rw.Close()
	created, err := sb.Create("b.txt")
	if err != nil {
		t.Fatal(err)
	}
	created.Close()
	if _, err := sb.Stat("a.txt"); err != nil {
		t.Fatal(err)
	}
	if err := sb.Rename("b.txt", "c.txt"); err != nil {
		t.Fatal(err)
	}
	if err := sb.Remove("c.txt"); err != nil {
		t.Fatal(err)
	}
	if err := sb.Remove("a.txt"); err != nil {
		t.Fatal(err)
	}
	if _, err := sb.OpenFile("a.txt"); err == nil {
		t.Fatal("removed file should not open")
	}
}

func TestSandboxSymlinkEscape(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation needs privileges on Windows")
	}
	sb, dir := newTempSandbox(t)
	outside := filepath.Join(filepath.Dir(dir), "secret.txt")
	if err := os.WriteFile(outside, []byte("secret"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(dir, "link")); err != nil {
		t.Skipf("symlink unsupported: %v", err)
	}
	if _, err := sb.OpenFile("link"); err == nil {
		t.Fatal("symlink escape must be rejected by os.Root")
	}
}
