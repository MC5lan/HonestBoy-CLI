package engine

import (
	"context"
	"testing"

	"github.com/MC5lan/HonestBoy-CLI/internal/core"
	"github.com/MC5lan/HonestBoy-CLI/internal/llm"
)

func TestReadOnlyCommandWhitelist(t *testing.T) {
	for _, cmd := range []string{"ls -la", "dir", "git status", "git diff --stat", "go test ./...", "pwd", "cat a.go"} {
		if !IsReadOnlyCommand(cmd) {
			t.Fatalf("%q should be read-only", cmd)
		}
	}
	for _, cmd := range []string{"rm -rf /", "go build ./... && rm -rf /", "curl http://x"} {
		if IsReadOnlyCommand(cmd) {
			t.Fatalf("%q must not be read-only", cmd)
		}
	}
}

func TestApprovalHandlerRoundTrip(t *testing.T) {
	client := &scriptedClient{responses: [][]llm.Chunk{
		{call("s1", "shell", `{"command":"echo hi"}`)},
		{doneCall("d1", "跑过命令")},
	}}
	eng := newTestEngine(t, client)
	eng.approval.Exec = "ask"
	asked := 0
	eng.approvalHandler = func(ctx context.Context, req core.ApprovalRequest) (core.ApprovalDecision, error) {
		asked++
		if req.Tool != "shell" || req.Command != "echo hi" {
			t.Fatalf("bad request: %+v", req)
		}
		return core.ApprovalDecision{Allowed: true, Reason: "用户批准"}, nil
	}
	rs := runEngine(t, eng, "跑命令")
	if asked != 1 {
		t.Fatalf("handler called %d times", asked)
	}
	if countEvents(rs, core.EvtApprovalNeeded) != 1 {
		t.Fatal("missing approval_required event")
	}
	if countEvents(rs, core.EvtToolDenied) != 0 || rs.terminal != core.TermDone {
		t.Fatalf("approved tool should run: terminal=%s denied=%d", rs.terminal, countEvents(rs, core.EvtToolDenied))
	}
}

func TestApprovalHandlerDeny(t *testing.T) {
	client := &scriptedClient{responses: [][]llm.Chunk{
		{call("s1", "shell", `{"command":"echo hi"}`)},
		{doneCall("d1", "跑过命令")},
	}}
	eng := newTestEngine(t, client)
	eng.approval.Exec = "ask"
	eng.approvalHandler = func(ctx context.Context, req core.ApprovalRequest) (core.ApprovalDecision, error) {
		return core.ApprovalDecision{Allowed: false, Reason: "用户拒绝"}, nil
	}
	rs := runEngine(t, eng, "跑命令")
	if countEvents(rs, core.EvtToolDenied) < 1 {
		t.Fatal("missing tool_denied")
	}
	found := false
	for _, ev := range rs.events {
		if ev.Type == core.EvtToolEnd {
			if d, ok := ev.Data.(core.ToolEndData); ok && !d.OK {
				found = true
			}
		}
	}
	if !found {
		t.Fatal("denied tool should emit failed tool_end")
	}
}

func TestApprovalAllowAll(t *testing.T) {
	client := &scriptedClient{responses: [][]llm.Chunk{
		{call("s1", "shell", `{"command":"echo a"}`), call("s2", "shell", `{"command":"echo b"}`)},
		{doneCall("d1", "两条命令")},
	}}
	eng := newTestEngine(t, client)
	eng.approval.Exec = "ask"
	asked := 0
	eng.approvalHandler = func(ctx context.Context, req core.ApprovalRequest) (core.ApprovalDecision, error) {
		asked++
		return core.ApprovalDecision{Allowed: true, AllowAll: true}, nil
	}
	_ = runEngine(t, eng, "跑命令")
	if asked != 1 {
		t.Fatalf("AllowAll should ask once, got %d", asked)
	}
}
