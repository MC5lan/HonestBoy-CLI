package tui

import (
	"testing"

	"github.com/MC5lan/HonestBoy-CLI/internal/core"
	tea "github.com/charmbracelet/bubbletea"
)

func TestApprovalFlow(t *testing.T) {
	ch := make(chan ApprovalAsk, 1)
	m := NewModel(Config{Approvals: ch})
	m.setSize(100, 40)
	ask := ApprovalAsk{
		Req:  core.ApprovalRequest{CallID: "c1", Tool: "shell", Command: "echo hi", Risk: "exec"},
		Resp: make(chan core.ApprovalDecision, 1),
	}
	ch <- ask
	updated, _ := m.Update(ask)
	mm := updated.(*Model)
	if mm.state != stApproving || mm.pending == nil {
		t.Fatal("approval not pending")
	}
	if view := mm.View(); len(view) < 10 {
		t.Fatal("approval card missing")
	}
	_, _ = mm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	select {
	case d := <-ask.Resp:
		if !d.Allowed {
			t.Fatal("y should approve")
		}
	default:
		t.Fatal("no decision sent")
	}
	if mm.pending != nil || mm.state != stStreaming {
		t.Fatal("approval not cleared")
	}
}

func TestApprovalHighRiskNeedsYes(t *testing.T) {
	ch := make(chan ApprovalAsk, 1)
	m := NewModel(Config{Approvals: ch})
	m.setSize(100, 40)
	ask := ApprovalAsk{
		Req:  core.ApprovalRequest{CallID: "c1", Tool: "shell", Command: "rm -rf /", HighRisk: true},
		Resp: make(chan core.ApprovalDecision, 1),
	}
	_, _ = m.Update(ask)
	_, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	select {
	case d := <-ask.Resp:
		if d.Allowed {
			t.Fatal("y must not approve high risk")
		}
	default:
		t.Fatal("y should still produce a deny decision")
	}
}

func TestReplayRestoresInterruptedHint(t *testing.T) {
	events := []core.Event{
		{Seq: 1, Type: core.EvtTurnStart, Data: core.TurnStartData{Input: "任务"}},
	}
	m := NewModel(Config{Replay: events})
	if len(m.blocks) == 0 || m.hint == "" {
		t.Fatalf("replay failed: blocks=%d hint=%q", len(m.blocks), m.hint)
	}
}
