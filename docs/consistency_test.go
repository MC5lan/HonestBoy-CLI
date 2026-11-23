package docs

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MC5lan/HonestBoy-CLI/internal/cli"
	"github.com/MC5lan/HonestBoy-CLI/internal/errs"
	"github.com/MC5lan/HonestBoy-CLI/internal/tui"
)

// TC07：错误码表与代码登记一致；改代码不改文档则测试红。
func TestErrorCodesDocumented(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("errors.md"))
	if err != nil {
		t.Fatal(err)
	}
	doc := string(raw)
	for code := range errs.Errors {
		if !strings.Contains(doc, string(code)) {
			t.Fatalf("error code %s missing from docs/errors.md", code)
		}
	}
}

// TC07：退出码表与实现一致。
func TestExitCodesDocumented(t *testing.T) {
	raw, err := os.ReadFile("exit-codes.md")
	if err != nil {
		t.Fatal(err)
	}
	doc := string(raw)
	for _, code := range []string{"0", "1", "2", "3", "4", "5", "6", "7", "130"} {
		if !strings.Contains(doc, "| "+code+" |") {
			t.Fatalf("exit code %s missing from docs/exit-codes.md", code)
		}
	}
}

// TC07：hb --help 的命令集合与 docs/commands.md 一致（双向）。
func TestCommandTableMatchesHelp(t *testing.T) {
	var buf bytes.Buffer
	cli.PrintHelp(&buf)
	helpSet := map[string]bool{}
	for _, line := range strings.Split(buf.String(), "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "hb ") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) >= 2 && !strings.HasPrefix(fields[1], "-") && !strings.HasPrefix(fields[1], "[") {
			helpSet[fields[1]] = true
		}
	}
	if len(helpSet) < 10 {
		t.Fatalf("parsed too few commands from help: %v", helpSet)
	}

	raw, err := os.ReadFile("commands.md")
	if err != nil {
		t.Fatal(err)
	}
	docSet := map[string]bool{}
	for _, line := range strings.Split(string(raw), "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "| `hb") {
			continue
		}
		start := strings.Index(line, "`")
		end := strings.Index(line[start+1:], "`")
		if start < 0 || end < 0 {
			continue
		}
		cmdText := line[start+1 : start+1+end]
		fields := strings.Fields(cmdText)
		if len(fields) >= 2 {
			docSet[fields[1]] = true
		}
	}

	for cmd := range helpSet {
		if !docSet[cmd] {
			t.Fatalf("command %q in --help but missing from docs/commands.md", cmd)
		}
	}
	for cmd := range docSet {
		if !helpSet[cmd] {
			t.Fatalf("command %q documented but missing from --help", cmd)
		}
	}
}

// TC07：TUI 快捷键表与 docs/commands.md 一致。
func TestKeyMapDocumented(t *testing.T) {
	raw, err := os.ReadFile("commands.md")
	if err != nil {
		t.Fatal(err)
	}
	doc := string(raw)
	for _, key := range tui.KeyBindings() {
		if !strings.Contains(doc, "`"+key+"`") {
			t.Fatalf("key %q missing from docs/commands.md", key)
		}
	}
}
