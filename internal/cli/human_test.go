package cli

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/MC5lan/HonestBoy-CLI/internal/core"
	"github.com/MC5lan/HonestBoy-CLI/internal/llm"
)

func TestHumanEventsFriendly(t *testing.T) {
	tests := []struct {
		ev   core.Event
		want string
	}{
		{core.Event{Type: core.EvtToolRequest, Data: core.ToolRequestData{Tool: "read_file"}}, "读取文件"},
		{core.Event{Type: core.EvtGateAllow}, "检查通过"},
		{core.Event{Type: core.EvtTurnEnd, Data: core.TurnEndData{Terminal: "untested"}}, "结果: 未验证"},
	}
	for _, tc := range tests {
		var buf bytes.Buffer
		if err := WriteHumanEvent(&buf, tc.ev); err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(buf.String(), tc.want) {
			t.Fatalf("got %q, want %q", buf.String(), tc.want)
		}
	}
}

func TestFriendlyErrorAuth(t *testing.T) {
	raw := `环境错误: {"error":{"message":"Authentication Fails","type":"invalid_request_error"}}`
	got := friendlyError(raw)
	if !strings.Contains(got, "密钥不对") || strings.Contains(got, `"error"`) {
		t.Fatalf("friendly auth error = %q", got)
	}
}

func TestHumanWriterFiltersControlBlocksAndShowsSummary(t *testing.T) {
	var buf bytes.Buffer
	h := NewHumanWriter(&buf)
	_ = h.WriteEvent(core.Event{Type: core.EvtTurnStart, Data: core.TurnStartData{Input: "改 bug"}})
	_ = h.WriteEvent(core.Event{Type: core.EvtTextDelta, Data: core.TextDeltaData{
		Text: "[SELF]\nconf: high\nmissing: none\nassumption: none\n\n",
	}})
	_ = h.WriteEvent(core.Event{Type: core.EvtTextDelta, Data: core.TextDeltaData{Text: "我先读一下文件。"}})
	_ = h.WriteEvent(core.Event{Type: core.EvtToolRequest, Data: core.ToolRequestData{CallID: "r1", Tool: "read_file"}})
	_ = h.WriteEvent(core.Event{Type: core.EvtToolEnd, Data: core.ToolEndData{CallID: "r1", OK: true, Output: "a.go"}})
	// 协议出口工具不单独展示，由 GateAllow/Abstain 汇总。
	_ = h.WriteEvent(core.Event{Type: core.EvtToolRequest, Data: core.ToolRequestData{CallID: "d1", Tool: "request_done"}})
	_ = h.WriteEvent(core.Event{Type: core.EvtToolEnd, Data: core.ToolEndData{CallID: "d1", OK: true}})
	_ = h.WriteEvent(core.Event{Type: core.EvtGateAllow, Data: core.GateAllowData{Claim: "修复 Add 并跑通测试"}})
	out := buf.String()
	if strings.Contains(out, "[SELF]") || strings.Contains(out, "conf: high") {
		t.Fatalf("control block leaked:\n%s", out)
	}
	if !strings.Contains(out, "我先读一下文件。\n") {
		t.Fatalf("assistant text not separated from tool line:\n%s", out)
	}
	if !strings.Contains(out, "修复 Add 并跑通测试") {
		t.Fatalf("final summary missing:\n%s", out)
	}
	if strings.Contains(out, "申请完成") {
		t.Fatalf("control tool leaked into output:\n%s", out)
	}
}

func TestFriendlyErrorNetwork(t *testing.T) {
	err := fmt.Errorf("%w: %w", ErrEnvironment, &llm.RetryableError{Status: 0, Message: "dial tcp: connection refused"})
	got := FriendlyError(err)
	if !strings.Contains(got, "base_url") || !strings.Contains(got, "hb doctor") {
		t.Fatalf("network error should be actionable: %q", got)
	}
	if strings.Contains(got, "dial tcp") {
		t.Fatalf("raw dial error leaked: %q", got)
	}
}

func TestFriendlyErrorDialTCP(t *testing.T) {
	got := friendlyError(`Post "http://127.0.0.1:9/chat/completions": dial tcp 127.0.0.1:9: connectex: connection refused`)
	if !strings.Contains(got, "base_url") || !strings.Contains(got, "hb doctor") {
		t.Fatalf("dial error should be actionable: %q", got)
	}
}

func TestFriendlyErrorTerminal(t *testing.T) {
	got := FriendlyError(&TerminalError{Terminal: core.TermDenied})
	if !strings.Contains(got, "缺验证证据") {
		t.Fatalf("terminal friendly = %q", got)
	}
}
