package tui

import (
	"context"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/MC5lan/HonestBoy-CLI/internal/core"
	tea "github.com/charmbracelet/bubbletea"
)

// 这些测试直接驱动 Model.Update（Bubble Tea 的纯函数核心），验证交互状态机。
// 覆盖了用户实测发现的问题：esc 中断、启动横幅常驻、忙碌行。

func key(s string) tea.KeyMsg {
	switch s {
	case "enter":
		return tea.KeyMsg{Type: tea.KeyEnter}
	case "esc":
		return tea.KeyMsg{Type: tea.KeyEsc}
	case "ctrl+c":
		return tea.KeyMsg{Type: tea.KeyCtrlC}
	}
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
}

func TestEscInterruptsBusyTurn(t *testing.T) {
	cancelled := false
	m := NewModel(Config{Cancel: func() { cancelled = true }})
	m.setSize(100, 30)
	m.state = stStreaming
	m.startedAt = time.Now()

	updated, _ := m.Update(key("esc"))
	mm := updated.(*Model)
	if !cancelled {
		t.Fatal("esc did not call Cancel while busy")
	}
	if mm.state != stIdle {
		t.Fatalf("state after esc = %d, want idle", mm.state)
	}
	if !strings.Contains(mm.hint, "已中断") {
		t.Fatalf("hint = %q", mm.hint)
	}
}

func TestEscIdleDoesNotQuit(t *testing.T) {
	m := NewModel(Config{})
	m.setSize(100, 30)
	m.state = stIdle
	_, cmd := m.Update(key("esc"))
	if cmd != nil {
		t.Fatal("esc while idle should not quit")
	}
}

func TestCtrlCIdleQuits(t *testing.T) {
	m := NewModel(Config{})
	m.setSize(100, 30)
	m.state = stIdle
	_, cmd := m.Update(key("ctrl+c"))
	if cmd == nil {
		t.Fatal("ctrl+c while idle should quit")
	}
}

func TestBannerAlwaysVisible(t *testing.T) {
	m := NewModel(Config{Workdir: "~/proj", Model: "m", Theme: "none"})
	m.setSize(80, 30)
	// 没有消息时，顶部标题栏包含产品名。
	emptyView := m.View()
	if !strings.Contains(emptyView, "Honest Boy") {
		t.Fatal("banner missing on empty screen")
	}
	// 有消息后，标题栏仍然常驻。
	m.applyEvent(core.Event{Seq: 1, Type: core.EvtTurnStart, Data: core.TurnStartData{Input: "任务"}})
	withBlocks := m.View()
	if !strings.Contains(withBlocks, "Honest Boy") {
		t.Fatal("banner disappeared after first message")
	}
}

func TestBusyLineVisible(t *testing.T) {
	m := NewModel(Config{Model: "m", Theme: "none"})
	m.setSize(80, 30)
	m.state = stStreaming
	m.startedAt = time.Now()
	m.spinnerIdx = 1
	view := m.View()
	if !strings.Contains(view, "Working") || !strings.Contains(view, "esc") {
		t.Fatalf("busy line missing: %q", plainView(view))
	}
}

func TestSubmitAndCancelFlow(t *testing.T) {
	cancelled := false
	m := NewModel(Config{Theme: "none", Cancel: func() { cancelled = true }})
	m.setSize(80, 30)
	m.input.SetValue("读文件")
	ctx, cancel := context.WithCancel(context.Background())
	m.cfg.Run = func(prompt, sessionID string) (<-chan core.Event, <-chan error) {
		events := make(chan core.Event, 4)
		errs := make(chan error, 1)
		go func() {
			events <- core.Event{Seq: 1, Type: core.EvtTurnStart, Data: core.TurnStartData{Input: prompt}}
			// 模拟一个卡住的请求：不发后续事件，等 ctx 取消。
			<-ctx.Done()
			close(events)
			close(errs)
		}()
		return events, errs
	}
	m.cfg.Cancel = func() { cancelled = true; cancel() }
	updated, cmd := m.Update(key("enter"))
	mm := updated.(*Model)
	if cmd == nil {
		t.Fatal("submit returned no cmd")
	}
	if mm.state != stStreaming {
		t.Fatalf("state = %d", mm.state)
	}
	// 中途 esc 中断（用返回的 model 继续驱动状态机）。
	_, _ = mm.Update(key("esc"))
	if !cancelled {
		t.Fatal("esc did not cancel running request")
	}
	if ctx.Err() != context.Canceled {
		t.Fatal("context not cancelled")
	}
}

var stripANSI = regexp.MustCompile(`\x1b\[[0-9;]*[A-Za-z]`)

func plainView(s string) string {
	return stripANSI.ReplaceAllString(s, "")
}
