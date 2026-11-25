package engine

import (
	"testing"

	"github.com/MC5lan/HonestBoy-CLI/internal/core"
	"github.com/MC5lan/HonestBoy-CLI/internal/llm"
)

// request_done.summary 必须进入 GateAllow 和 TurnEnd，供 CLI/TUI 展示。
func TestFinalSummaryInEvents(t *testing.T) {
	const summary = "执行 echo 并确认退出码 0"
	client := &scriptedClient{responses: [][]llm.Chunk{
		{
			{Text: "[SELF]\nconf: high\nmissing: none\nassumption: none\n先执行 echo。"},
			call("s1", "shell", `{"command":"echo hi"}`),
		},
		{
			{Text: "echo 已经通过。"},
			doneCall("d", summary),
		},
	}}
	eng := newTestEngine(t, client)
	eng.cfg.Approval.Exec = "auto"
	rs := runEngine(t, eng, "跑 echo")
	if rs.terminal != core.TermDone {
		t.Fatalf("terminal = %s", rs.terminal)
	}
	gotClaim, gotSummary := false, false
	for _, ev := range rs.events {
		switch ev.Type {
		case core.EvtGateAllow:
			if d, ok := ev.Data.(core.GateAllowData); ok && d.Claim == summary {
				gotClaim = true
			}
		case core.EvtTurnEnd:
			if d, ok := ev.Data.(core.TurnEndData); ok && d.Summary == summary {
				gotSummary = true
			}
		}
	}
	if !gotClaim || !gotSummary {
		t.Fatalf("summary missing from events: gate=%v turn_end=%v", gotClaim, gotSummary)
	}
}
