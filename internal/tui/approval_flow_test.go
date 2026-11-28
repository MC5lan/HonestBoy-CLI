package tui

import (
	"strings"
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
	if mm.pending != nil || mm.state != stIdle {
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
		t.Fatal("single y should not decide; it is the first letter of yes")
	default:
	}
	if !strings.Contains(m.input.Value(), "y") {
		t.Fatalf("y should be typed into the composer, got %q", m.input.Value())
	}
	_, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	select {
	case d := <-ask.Resp:
		if d.Allowed {
			t.Fatal("esc must deny high risk")
		}
	default:
		t.Fatal("esc should produce a deny decision")
	}
}

// 高危审批必须真的能输入 yes 并通过：此前 y 会直接拒绝，导致永远无法批准。
func TestApprovalHighRiskYesApproves(t *testing.T) {
	ch := make(chan ApprovalAsk, 1)
	m := NewModel(Config{Approvals: ch})
	m.setSize(100, 40)
	ask := ApprovalAsk{
		Req:  core.ApprovalRequest{CallID: "c1", Tool: "shell", Command: "rm -rf /", HighRisk: true},
		Resp: make(chan core.ApprovalDecision, 1),
	}
	m.Update(ask)
	for _, r := range "yes" {
		m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	select {
	case d := <-ask.Resp:
		if !d.Allowed {
			t.Fatal("typing yes + enter should approve high risk")
		}
	default:
		t.Fatal("no decision sent")
	}
}

// outside_read 的 Command 是路径，但 HighRisk=true；卡片必须按显式标记显示高危。
func TestApprovalOutsideReadUsesExplicitHighRisk(t *testing.T) {
	ch := make(chan ApprovalAsk, 1)
	m := NewModel(Config{Approvals: ch})
	m.setSize(100, 40)
	m.Update(ApprovalAsk{
		Req:  core.ApprovalRequest{CallID: "c1", Tool: "outside_read", Command: `C:\Users\dev\notes.txt`, HighRisk: true},
		Resp: make(chan core.ApprovalDecision, 1),
	})
	view := stripANSI.ReplaceAllString(m.View(), "")
	if !strings.Contains(view, "高危操作") || !strings.Contains(view, "输入 yes") {
		t.Fatalf("outside_read card not marked high risk:\n%s", view)
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
