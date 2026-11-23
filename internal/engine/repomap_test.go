package engine

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRepoMapGoSymbols(t *testing.T) {
	dir := t.TempDir()
	code := `package demo

type Engine struct{}

func NewEngine() *Engine { return &Engine{} }

func (e *Engine) Run(prompt string) error { return nil }
`
	if err := os.WriteFile(filepath.Join(dir, "a.go"), []byte(code), 0o600); err != nil {
		t.Fatal(err)
	}
	m := BuildRepoMap(dir, 4000)
	if m == nil || len(m.Symbols) == 0 {
		t.Fatal("no symbols")
	}
	text := m.Render()
	for _, want := range []string{"NewEngine", "Engine.Run", "type Engine", "refs="} {
		if !strings.Contains(text, want) {
			t.Fatalf("missing %q in:\n%s", want, text)
		}
	}
	// 引用计数：Run 在源码中出现 2 次（定义 + 未调用，这里只算定义 1 次，至少 >=1）
	for _, s := range m.Symbols {
		if s.Name == "Engine" && s.Refs < 1 {
			t.Fatalf("Engine refs = %d", s.Refs)
		}
	}
}

func TestRepoMapNonGoAndBudget(t *testing.T) {
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "a.py"), []byte("def hello(x):\n    return x\n\nclass World:\n    pass\n"), 0o600)
	_ = os.WriteFile(filepath.Join(dir, "b.js"), []byte("function greet() {}\nclass Foo {}\n"), 0o600)
	_ = os.MkdirAll(filepath.Join(dir, ".git"), 0o700)
	_ = os.WriteFile(filepath.Join(dir, ".git", "config.go"), []byte("func hidden() {}\n"), 0o600)

	m := BuildRepoMap(dir, 30)
	if m == nil || len(m.Symbols) == 0 {
		t.Fatal("no symbols")
	}
	text := m.Render()
	if strings.Contains(text, "hidden") {
		t.Fatal("ignored dir included")
	}
	if m.Tokens > 30 {
		t.Fatalf("budget exceeded: %d", m.Tokens)
	}
	if RepoMapContext(dir, 30) == "" {
		t.Fatal("RepoMapContext empty")
	}
}
