package tui

import (
	"strings"
	"testing"

	"github.com/MC5lan/HonestBoy-CLI/internal/core"
	tea "github.com/charmbracelet/bubbletea"
)

func TestComputeLayout(t *testing.T) {
	cases := []struct {
		w, h int
	}{
		{100, 40},
		{80, 24},
		{100, 6},
		{40, 3},
	}
	for _, tc := range cases {
		l := ComputeLayout(tc.w, tc.h)
		if l.ViewHeight < 1 || l.InputHeight < 1 || l.BannerHeight < 1 {
			t.Fatalf("layout(%d,%d) = %+v", tc.w, tc.h, l)
		}
	}
	if narrowMode(59) != true || narrowMode(60) != false {
		t.Fatal("narrow mode boundary wrong")
	}
}

func TestSlashCommand(t *testing.T) {
	name, args, ok := SlashCommand("/model gpt-5")
	if !ok || name != "/model" || len(args) != 1 || args[0] != "gpt-5" {
		t.Fatalf("slash = %q %v %v", name, args, ok)
	}
	if _, _, ok := SlashCommand("hello"); ok {
		t.Fatal("non-slash accepted")
	}
	s := suggestCommands("/stat")
	if len(s) != 2 || s[0] != "/stats" {
		t.Fatalf("suggestions = %v", s)
	}
}

func TestModelEventFlow(t *testing.T) {
	m := NewModel(Config{Workdir: "D:\\work", Model: "m", Theme: "none"})
	m.setSize(100, 40)
	if _, cmd := m.Update(tea.WindowSizeMsg{Width: 100, Height: 40}); cmd != nil {
		t.Fatal("unexpected cmd")
	}
	m.input.SetValue("改个 bug")
	returned := false
	m.cfg.Run = func(prompt string) (<-chan core.Event, <-chan error) {
		returned = true
		ch := make(chan core.Event, 1)
		errs := make(chan error, 1)
		ch <- core.Event{Seq: 1, Type: core.EvtTextDelta, Data: core.TextDeltaData{Text: "收到"}}
		close(ch)
		close(errs)
		return ch, errs
	}
	cmd := m.submit()
	if !returned || cmd == nil {
		t.Fatal("submit did not start run")
	}
	if m.state != stStreaming {
		t.Fatalf("state = %d", m.state)
	}
	ev := core.Event{Seq: 2, Type: core.EvtTextDelta, Data: core.TextDeltaData{Text: "回答文本"}}
	m.applyEvent(ev)
	if len(m.blocks) == 0 {
		t.Fatal("no blocks")
	}
	found := false
	for _, b := range m.blocks {
		if b.kind == blkAssistant && strings.Contains(b.content, "回答文本") {
			found = true
		}
	}
	if !found {
		t.Fatal("assistant block missing text")
	}
	m.applyEvent(core.Event{Seq: 3, Type: core.EvtTurnEnd, Data: core.TurnEndData{Terminal: "done"}})
	if m.term != core.TermDone || m.state != stIdle {
		t.Fatalf("term=%s state=%d", m.term, m.state)
	}
	view := m.View()
	if !strings.Contains(view, "就绪") && !strings.Contains(view, "完成") {
		t.Fatalf("view missing status: %s", view)
	}
}

func TestModelCancel(t *testing.T) {
	cancelled := false
	m := NewModel(Config{Cancel: func() { cancelled = true }})
	m.setSize(80, 24)
	m.state = stStreaming
	if cmd := m.cancel(); cmd != nil || !cancelled {
		t.Fatal("first cancel should call Cancel only")
	}
	if m.state != stIdle || !strings.Contains(m.hint, "已中断") {
		t.Fatalf("hint=%q state=%d", m.hint, m.state)
	}
}
