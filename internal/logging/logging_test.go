package logging

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRedact(t *testing.T) {
	samples := []string{
		"Authorization: Bearer sk-abcdefghijklmnopqrstuvwxyz",
		"key=sk-1234567890abcdefghij",
		`"api_key": "supersecret123"`,
		"normal text without secret",
	}
	joined := strings.Join(samples, " | ")
	out := Redact(joined)
	for _, bad := range []string{"sk-abcdefghijklmnopqrstuvwxyz", "sk-1234567890abcdefghij", "supersecret123"} {
		if strings.Contains(out, bad) {
			t.Fatalf("secret leaked: %s", out)
		}
	}
	if !strings.Contains(out, "sk-***") || !strings.Contains(out, "normal text") {
		t.Fatalf("redact output wrong: %s", out)
	}
}

func TestRedactWriter(t *testing.T) {
	var buf strings.Builder
	w := &redactWriter{inner: &buf}
	n, err := w.Write([]byte("token sk-abcdefghijklmnop trailing"))
	if err != nil || n != len("token sk-abcdefghijklmnop trailing") {
		t.Fatalf("write = %d %v", n, err)
	}
	if strings.Contains(buf.String(), "sk-abcdefghijklmnop") {
		t.Fatalf("writer leaked: %s", buf.String())
	}
}

func TestInitWritesFile(t *testing.T) {
	dir := t.TempDir()
	if err := Init(true, dir); err != nil {
		t.Fatal(err)
	}
	L().Info("test", "session_id", "s1")
	if err := Close(); err != nil {
		t.Fatal(err)
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) == 0 {
		t.Fatal("no log file")
	}
	data, _ := os.ReadFile(filepath.Join(dir, entries[0].Name()))
	if !strings.Contains(string(data), "test") || !strings.Contains(string(data), "session_id") {
		t.Fatalf("log content = %s", data)
	}
}
