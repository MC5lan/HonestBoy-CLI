package engine

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/MC5lan/HonestBoy-CLI/internal/core"
	"github.com/MC5lan/HonestBoy-CLI/internal/llm"
	"github.com/MC5lan/HonestBoy-CLI/internal/store"
	"github.com/MC5lan/HonestBoy-CLI/internal/tools"
)

func TestBuildResumePrompt(t *testing.T) {
	s := &core.Session{Events: []core.Event{
		{Type: core.EvtTurnStart, Data: core.TurnStartData{Input: "修 bug"}},
		{Type: core.EvtToolRequest, Data: core.ToolRequestData{CallID: "c1", Tool: "read_file", Args: `{"path":"a.go"}`}},
		{Type: core.EvtToolEnd, Data: core.ToolEndData{CallID: "c1", OK: true, Output: "read ok"}},
		{Type: core.EvtTextDelta, Data: core.TextDeltaData{Text: "我先看看"}},
		{Type: core.EvtToolRequest, Data: core.ToolRequestData{CallID: "c2", Tool: "shell", Args: `{"command":"go test"}`}},
		{Type: core.EvtToolEnd, Data: core.ToolEndData{CallID: "c2", OK: false, Output: "exit 1"}},
		// 只有 request、没有 end：进程在工具执行中被杀。
		{Type: core.EvtToolRequest, Data: core.ToolRequestData{CallID: "c3", Tool: "write_file", Args: `{"path":"b.go"}`}},
		{Type: core.EvtTurnStart, Data: core.TurnStartData{Input: "继续上次", Resumed: true, ResumeAttempt: 1}},
		{Type: core.EvtTextDelta, Data: core.TextDeltaData{Text: "继续中"}},
	}}
	p := BuildResumePrompt(s, "crash")
	for _, want := range []string{
		"原始任务", "修 bug", "read_file", "已完成", "shell", "失败",
		"不要重复执行", "进程异常退出", "继续中", "状态未知", "write_file",
	} {
		if !strings.Contains(p, want) {
			t.Fatalf("resume prompt missing %q:\n%s", want, p)
		}
	}
	if strings.Contains(p, "原始任务：\n继续上次") {
		t.Fatal("resume prompt used the resume turn as the original task")
	}
}

func TestResumeOutcome(t *testing.T) {
	cancelCtx, cancel := context.WithCancel(context.Background())
	cancel()
	past, pastCancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer pastCancel()
	<-past.Done()

	cases := []struct {
		name     string
		terminal core.Terminal
		err      error
		ctx      context.Context
		want     bool
		reason   string
	}{
		{"done", core.TermDone, nil, context.Background(), false, ""},
		{"untested", core.TermUntest, nil, context.Background(), false, ""},
		{"denied", core.TermDenied, nil, context.Background(), false, ""},
		{"user_abort", core.TermHalt, context.Canceled, cancelCtx, false, "user_abort"},
		{"timeout", core.TermHalt, context.DeadlineExceeded, past, true, "timeout"},
		{"plan_halt", core.TermHalt, nil, context.Background(), false, ""},
		{"failed", core.TermFailed, errors.New("boom"), context.Background(), true, "failed"},
		{"network", core.TermFailed, &llm.RetryableError{Status: 0, Message: "dial"}, context.Background(), true, "network"},
		{"permanent", core.TermFailed, &llm.RetryableError{Status: 401, Retryable: false}, context.Background(), false, "permanent"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, reason := resumeOutcome(tc.terminal, tc.err, tc.ctx)
			if got != tc.want || reason != tc.reason {
				t.Fatalf("got %v/%q, want %v/%q", got, reason, tc.want, tc.reason)
			}
		})
	}
}

// errorClient 只返回错误，用于验证 failed 回合的可恢复标记。
type errorClient struct{ err error }

func (c errorClient) Stream(ctx context.Context, _ llm.Request) (<-chan llm.Chunk, <-chan error) {
	chunks := make(chan llm.Chunk)
	errs := make(chan error, 1)
	go func() {
		defer close(errs)
		errs <- c.err
		time.Sleep(20 * time.Millisecond)
		close(chunks)
	}()
	return chunks, errs
}

func drainTurn(t *testing.T, eng *Engine, ctx context.Context, prompt string) (core.TurnStartData, core.TurnEndData, error) {
	t.Helper()
	events, errs := eng.Run(ctx, prompt)
	var start core.TurnStartData
	var end core.TurnEndData
	for ev := range events {
		switch ev.Type {
		case core.EvtTurnStart:
			start, _ = ev.Data.(core.TurnStartData)
		case core.EvtTurnEnd:
			end, _ = ev.Data.(core.TurnEndData)
		}
	}
	return start, end, <-errs
}

func TestEngineTurnEndResumableOnNetworkFailure(t *testing.T) {
	eng := newTestEngine(t, errorClient{err: &llm.RetryableError{Status: 0, Retryable: true, Message: "dial"}})
	_, end, err := drainTurn(t, eng, context.Background(), "修 bug")
	if err == nil {
		t.Fatal("expected engine error")
	}
	if !end.Resumable || end.ResumeReason != "network" {
		t.Fatalf("turn_end resumable=%v reason=%q", end.Resumable, end.ResumeReason)
	}
}

func TestEngineTurnEndNotResumableOnCancel(t *testing.T) {
	c := &pauseClient{sent: make(chan struct{}), release: make(chan struct{})}
	defer close(c.release)
	eng := newTestEngine(t, c)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	events, errs := eng.Run(ctx, "乱码")
	select {
	case <-c.sent:
	case <-time.After(5 * time.Second):
		t.Fatal("engine never consumed the chunk")
	}
	cancel()
	var end core.TurnEndData
	for ev := range events {
		if ev.Type == core.EvtTurnEnd {
			end, _ = ev.Data.(core.TurnEndData)
		}
	}
	<-errs
	if end.Resumable || end.ResumeReason != "user_abort" {
		t.Fatalf("cancel turn_end resumable=%v reason=%q", end.Resumable, end.ResumeReason)
	}
}

func TestEngineTurnEndResumableOnTimeout(t *testing.T) {
	c := &pauseClient{sent: make(chan struct{}), release: make(chan struct{})}
	defer close(c.release)
	eng := newTestEngine(t, c)
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	events, errs := eng.Run(ctx, "乱码")
	select {
	case <-c.sent:
	case <-time.After(5 * time.Second):
		t.Fatal("engine never consumed the chunk")
	}
	var end core.TurnEndData
	for ev := range events {
		if ev.Type == core.EvtTurnEnd {
			end, _ = ev.Data.(core.TurnEndData)
		}
	}
	<-errs
	if !end.Resumable || end.ResumeReason != "timeout" {
		t.Fatalf("timeout turn_end resumable=%v reason=%q", end.Resumable, end.ResumeReason)
	}
}

func TestEngineStampsResumeAttempt(t *testing.T) {
	client := &scriptedClient{responses: [][]llm.Chunk{{{Text: "继续完成"}}}}
	eng := newTestEngine(t, client)
	if err := eng.store.Create(core.Session{ID: "s_resume", Cwd: eng.cfg.Workdir, CreatedAt: time.Now().UTC()}); err != nil {
		t.Fatal(err)
	}
	_ = eng.store.Unlock()
	eng.cfg.ResumeID = "s_resume"
	eng.cfg.ResumeAttempt = 2

	start, _, err := drainTurn(t, eng, context.Background(), "继续")
	if err != nil {
		t.Fatal(err)
	}
	if !start.Resumed || start.ResumeAttempt != 2 {
		t.Fatalf("turn_start resumed=%v attempt=%d", start.Resumed, start.ResumeAttempt)
	}
}

func abstainCall(id, reason string) llm.Chunk {
	return call(id, "declare_untested", `{"reason":"`+reason+`","not_done":["测试"]}`)
}

// 续接同一会话时，第二回合的系统提示必须包含第一回合的历史。
func TestResumeCarriesPreviousTurnIntoContext(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module resume\n\ngo 1.27\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\nfunc main() {}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	paths := store.Paths{Root: root, Sessions: filepath.Join(root, "sessions"), Logs: filepath.Join(root, "logs")}

	client1 := &scriptedClient{responses: [][]llm.Chunk{{abstainCall("u1", "第一轮没有验证")}}}
	sb1, err := tools.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	eng1 := New(Config{
		Workdir: dir, Model: "test", Client: client1,
		Registry: tools.NewDefaultRegistry(sb1, tools.NewReadGuard(), "cmd"),
		Store:    store.NewSessionStore(paths), MaxTurns: 5,
	})
	res1 := runEngine(t, eng1, "第一轮任务：请只回答，不改文件")
	sb1.Close()
	sessionID := ""
	for _, ev := range res1.events {
		if ev.Type == core.EvtTurnStart {
			sessionID = ev.SessionID
		}
	}
	if sessionID == "" {
		t.Fatal("first run did not emit session id")
	}

	client2 := &scriptedClient{responses: [][]llm.Chunk{{abstainCall("u2", "第二轮没有验证")}}}
	sb2, err := tools.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer sb2.Close()
	eng2 := New(Config{
		Workdir: dir, Model: "test", Client: client2, ResumeID: sessionID,
		Registry: tools.NewDefaultRegistry(sb2, tools.NewReadGuard(), "cmd"),
		Store:    store.NewSessionStore(paths), MaxTurns: 5,
	})
	runEngine(t, eng2, "第二轮任务：接着上一轮")
	if len(client2.requests) == 0 || len(client2.requests[0].Messages) == 0 {
		t.Fatal("second run made no request")
	}
	system := client2.requests[0].Messages[0].Content
	if !strings.Contains(system, "之前回合") {
		t.Fatalf("second run missing history section:\n%s", system)
	}
	if !strings.Contains(system, "第一轮任务") {
		t.Fatalf("second run missing previous user input:\n%s", system)
	}
	// 续接后的证据链必须包含两个回合，且仍然完整。
	sess, err := store.NewSessionStore(paths).Load(sessionID)
	if err != nil {
		t.Fatal(err)
	}
	if len(sess.Receipts) != 2 {
		t.Fatalf("receipts = %d, want 2", len(sess.Receipts))
	}
	if at, err := core.VerifyChain(sess.Receipts); err != nil {
		t.Fatalf("receipt chain broken at %d: %v", at, err)
	}
}
