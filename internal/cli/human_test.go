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

func TestHumanWriterReceiptDone(t *testing.T) {
	var buf bytes.Buffer
	h := NewHumanWriter(&buf)
	write := func(e core.Event) { _ = h.WriteEvent(e) }
	write(core.Event{SessionID: "s1", Type: core.EvtTurnStart, Data: core.TurnStartData{Input: "改 a.go"}})
	write(core.Event{Type: core.EvtToolRequest, Data: core.ToolRequestData{CallID: "w1", Tool: "write_file", Args: `{"path":"a.go","content":"x"}`}})
	write(core.Event{Type: core.EvtToolEnd, Data: core.ToolEndData{CallID: "w1", OK: true, Output: "ok"}})
	write(core.Event{Type: core.EvtToolRequest, Data: core.ToolRequestData{CallID: "v1", Tool: "verify_test", Args: `{}`}})
	write(core.Event{Type: core.EvtToolEnd, Data: core.ToolEndData{CallID: "v1", OK: true, Output: "PASS"}})
	write(core.Event{Type: core.EvtGateAllow, Data: core.GateAllowData{Claim: "改完并测试"}})
	write(core.Event{Type: core.EvtTurnEnd, Data: core.TurnEndData{
		Terminal: "done", Summary: "改完并测试", DurationMS: 1234, TokensIn: 10, TokensOut: 2,
	}})
	out := buf.String()
	for _, want := range []string{
		"■ 完成", "摘要: 改完并测试", "改动: a.go", "验证: test ✓", "未验证: —",
		"下一步:", "hb verify", "hb diff",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("receipt missing %q:\n%s", want, out)
		}
	}
}

func TestHumanWriterReceiptUntested(t *testing.T) {
	var buf bytes.Buffer
	h := NewHumanWriter(&buf)
	write := func(e core.Event) { _ = h.WriteEvent(e) }
	write(core.Event{SessionID: "s_bad", Type: core.EvtTurnStart, Data: core.TurnStartData{Input: "随便改"}})
	write(core.Event{Type: core.EvtAbstain, Data: core.AbstainData{Reason: "没验证", NotDone: []string{"测试", "构建"}}})
	write(core.Event{Type: core.EvtTurnEnd, Data: core.TurnEndData{
		Terminal: "untested", Summary: "没验证", Resumable: true, ResumeReason: "failed",
	}})
	out := buf.String()
	for _, want := range []string{
		"■ 未验证", "未验证: 测试, 构建", "hb resume --last", "hb sessions show s_bad",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("receipt missing %q:\n%s", want, out)
		}
	}
}

func TestApplyPatchPaths(t *testing.T) {
	args := `{"patch":"*** Begin Patch\n*** Add File: a.go\n+x\n*** Update File: b.go\n@@\n*** Delete File: c.go\n*** End Patch"}`
	got := applyPatchPaths(args)
	want := []string{"a.go", "b.go", "c.go"}
	if len(got) != len(want) {
		t.Fatalf("paths = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("paths = %v, want %v", got, want)
		}
	}
}

func TestFriendlyErrorStatuses(t *testing.T) {
	cases := []struct {
		status int
		want   string
	}{
		{401, "密钥"},
		{403, "密钥"},
		{404, "模型"},
		{400, "请求"},
		{429, "频繁"},
		{500, "服务端"},
	}
	for _, tc := range cases {
		err := fmt.Errorf("%w: %w", ErrEnvironment, &llm.RetryableError{Status: tc.status, Message: "x"})
		if got := FriendlyError(err); !strings.Contains(got, tc.want) {
			t.Fatalf("status %d: %q missing %q", tc.status, got, tc.want)
		}
	}
}

func TestFriendlyErrorPlainStatus(t *testing.T) {
	if got := friendlyError("HTTP 401 Unauthorized"); !strings.Contains(got, "密钥") {
		t.Fatalf("plain 401 = %q", got)
	}
	if got := friendlyError("HTTP 404 model not found"); !strings.Contains(got, "模型") {
		t.Fatalf("plain 404 = %q", got)
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
