package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestSlashSuggestions(t *testing.T) {
	m := NewModel(Config{Model: "m", Theme: "none"})
	m.setSize(100, 34)
	m.input.SetValue("/")
	sug := m.slashSuggestions()
	if len(sug) < 8 {
		t.Fatalf("slash should list all commands, got %d", len(sug))
	}
	m.input.SetValue("/mo")
	sug = m.slashSuggestions()
	if len(sug) != 1 || sug[0].name != "/model" {
		t.Fatalf("prefix filter = %+v", sug)
	}
	m.input.SetValue("/model gpt-5")
	if m.slashSuggestions() != nil {
		t.Fatal("with args should not show panel")
	}
	if m.slashSuggestions() != nil {
		t.Fatal("non-slash should not show panel")
	}
}

func TestTabCompletesCommand(t *testing.T) {
	m := NewModel(Config{Theme: "none"})
	m.setSize(100, 34)
	m.input.SetValue("/mo")
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	mm := updated.(*Model)
	if strings.TrimSpace(mm.input.Value()) != "/model" {
		t.Fatalf("tab complete = %q", mm.input.Value())
	}
}

func TestModelSwitch(t *testing.T) {
	var saved string
	m := NewModel(Config{Model: "deepseek-chat", Theme: "none", SetModel: func(n string) error {
		saved = n
		return nil
	}})
	m.setSize(100, 34)
	m.runSlash("/model", "/model deepseek-reasoner")
	if m.cfg.Model != "deepseek-reasoner" || saved != "deepseek-reasoner" {
		t.Fatalf("model=%q saved=%q", m.cfg.Model, saved)
	}
	if !strings.Contains(m.hint, "已切换") {
		t.Fatalf("hint = %q", m.hint)
	}
	// 无参数只提示当前模型
	m.runSlash("/model", "/model")
	if !strings.Contains(m.hint, "deepseek-reasoner") {
		t.Fatalf("hint = %q", m.hint)
	}
}

func TestConfigAndPathCommands(t *testing.T) {
	m := NewModel(Config{Model: "m", Theme: "none", Workdir: `C:\work\proj`, Protocol: "anthropic", BaseURL: "https://api.anthropic.com"})
	m.setSize(100, 34)
	m.runSlash("/config", "/config")
	if !strings.Contains(m.hint, "anthropic") || !strings.Contains(m.hint, "api.anthropic.com") {
		t.Fatalf("config hint = %q", m.hint)
	}
	m.runSlash("/path", "/path")
	if !strings.Contains(m.hint, `C:\work\proj`) {
		t.Fatalf("path hint = %q", m.hint)
	}
}

func TestCommandPanelRenders(t *testing.T) {
	m := NewModel(Config{Model: "m", Theme: "none"})
	m.setSize(100, 34)
	m.input.SetValue("/")
	view := stripANSI.ReplaceAllString(m.View(), "")
	if !strings.Contains(view, "切换模型") || !strings.Contains(view, "Tab 补全") {
		t.Fatalf("command panel missing:\n%s", view)
	}
}
