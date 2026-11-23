package tui

import (
	"errors"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// TestHelpToggle 验证 ? 打开帮助、Esc 关闭且按键不穿透输入框。
func TestHelpToggle(t *testing.T) {
	m := NewModel(Config{Theme: "none"})
	m.setSize(100, 34)
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("?")})
	if !m.showHelp {
		t.Fatal("? should open help page")
	}
	if got := m.input.Value(); got != "" {
		t.Fatalf("? leaked into input: %q", got)
	}
	if !strings.Contains(m.View(), "快捷键") {
		t.Fatal("help page not rendered")
	}
	m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if m.showHelp {
		t.Fatal("esc should close help page")
	}
}

func TestHelpViaSlash(t *testing.T) {
	m := NewModel(Config{Theme: "none"})
	m.setSize(100, 34)
	m.runSlash("/help", "/help")
	if !m.showHelp {
		t.Fatal("/help should open help page")
	}
	if !strings.Contains(m.View(), "Ctrl+C") {
		t.Fatal("keymap missing from help page")
	}
}

func TestOnboardHint(t *testing.T) {
	m := NewModel(Config{Theme: "none", Onboard: true})
	m.setSize(100, 34)
	if !strings.Contains(m.hint, "首次使用") {
		t.Fatalf("onboard hint missing: %q", m.hint)
	}
	if !strings.Contains(m.View(), "/ 命令面板") {
		t.Fatal("empty-state quickstart not rendered")
	}
}

func TestExecCommandInTUI(t *testing.T) {
	var called string
	m := NewModel(Config{Theme: "none", Exec: func(cmd string, args []string) (string, error) {
		called = cmd
		return "链完整", nil
	}})
	m.setSize(100, 34)
	m.runSlash("/verify", "/verify")
	if called != "/verify" {
		t.Fatalf("exec not called: %q", called)
	}
	found := false
	for _, b := range m.blocks {
		if strings.Contains(b.content, "链完整") {
			found = true
		}
	}
	if !found {
		t.Fatal("command output not shown in transcript")
	}
}

func TestExecCommandError(t *testing.T) {
	m := NewModel(Config{Theme: "none", Exec: func(string, []string) (string, error) {
		return "", errors.New("boom")
	}})
	m.setSize(100, 34)
	m.runSlash("/stats", "/stats")
	if !strings.Contains(m.hint, "执行失败") {
		t.Fatalf("hint = %q", m.hint)
	}
}

func TestContextAndCompactCommands(t *testing.T) {
	m := NewModel(Config{Theme: "none", ContextWindow: 1000})
	m.setSize(100, 34)
	m.tokensIn, m.tokensOut = 100, 50
	m.runSlash("/context", "/context")
	if !strings.Contains(m.hint, "150") || !strings.Contains(m.hint, "1000") {
		t.Fatalf("context hint = %q", m.hint)
	}
	m.runSlash("/compact", "/compact")
	if !strings.Contains(m.hint, "压缩") {
		t.Fatalf("compact hint = %q", m.hint)
	}
}
