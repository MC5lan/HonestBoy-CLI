package tui

import (
	"strings"
	"testing"

	"github.com/MC5lan/HonestBoy-CLI/internal/core"
)

func TestPanelShowsObservables(t *testing.T) {
	m := NewModel(Config{Workdir: "~/p", Model: "m", Theme: "none"})
	m.setSize(110, 34)
	m.applyEvent(core.Event{Seq: 1, Type: core.EvtToolEnd, Data: core.ToolEndData{CallID: "c", OK: true}})
	m.applyEvent(core.Event{Seq: 2, Type: core.EvtGateDeny, Data: core.GateDenyData{Rule: "coverage", Reason: "缺少 test"}})
	panel := stripANSI.ReplaceAllString(m.renderPanel(26), "")
	for _, want := range []string{"状态", "Gate", "coverage", "本轮", "会话"} {
		if !strings.Contains(panel, want) {
			t.Fatalf("panel missing %q:\n%s", want, panel)
		}
	}
}

func TestPanelCounts(t *testing.T) {
	m := NewModel(Config{Theme: "none"})
	m.setSize(110, 34)
	m.applyEvent(core.Event{Seq: 1, Type: core.EvtToolEnd, Data: core.ToolEndData{CallID: "a", OK: true}})
	m.applyEvent(core.Event{Seq: 2, Type: core.EvtToolEnd, Data: core.ToolEndData{CallID: "b", OK: false}})
	m.applyEvent(core.Event{Seq: 3, Type: core.EvtGateAllow, Data: core.GateAllowData{}})
	s := m.stats()
	if s.toolsOK != 1 || s.toolsFail != 1 || s.gateAllow != 1 {
		t.Fatalf("stats = %+v", s)
	}
}
