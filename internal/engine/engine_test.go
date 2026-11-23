package engine

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MC5lan/HonestBoy-CLI/internal/core"
	"github.com/MC5lan/HonestBoy-CLI/internal/llm"
	"github.com/MC5lan/HonestBoy-CLI/internal/store"
	"github.com/MC5lan/HonestBoy-CLI/internal/tools"
)

type scriptedClient struct {
	responses [][]llm.Chunk
	index     int
}

func (c *scriptedClient) Stream(ctx context.Context, req llm.Request) (<-chan llm.Chunk, <-chan error) {
	i := c.index
	c.index++
	chunks := make(chan llm.Chunk, 16)
	errs := make(chan error, 1)
	go func() {
		defer close(chunks)
		defer close(errs)
		if i >= len(c.responses) {
			chunks <- llm.Chunk{Done: true}
			return
		}
		for _, ch := range c.responses[i] {
			chunks <- ch
		}
		chunks <- llm.Chunk{Done: true}
	}()
	return chunks, errs
}

func call(id, name, args string) llm.Chunk {
	return llm.Chunk{Call: &llm.ToolCall{ID: id, Name: name, Args: args}}
}

func doneCall(id, summary string) llm.Chunk {
	return call(id, "request_done", `{"summary":"`+summary+`"}`)
}

func newTestEngine(t *testing.T, client llm.Client) *Engine {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module engtest\n\ngo 1.27\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\nfunc main() {}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	sb, err := tools.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { sb.Close() })
	paths := store.Paths{Root: dir, Sessions: filepath.Join(dir, "sessions"), Logs: filepath.Join(dir, "logs")}
	return New(Config{
		Workdir: dir, Model: "test", Client: client,
		Registry: tools.NewDefaultRegistry(sb, tools.NewReadGuard(), "cmd"),
		Store:    store.NewSessionStore(paths), MaxTurns: 10,
	})
}

type runResult struct {
	terminal core.Terminal
	rule     string
	events   []core.Event
}

func runEngine(t *testing.T, eng *Engine, prompt string) runResult {
	t.Helper()
	events, errs := eng.Run(context.Background(), prompt)
	var out runResult
	for ev := range events {
		out.events = append(out.events, ev)
		switch ev.Type {
		case core.EvtTurnEnd:
			if d, ok := ev.Data.(core.TurnEndData); ok {
				out.terminal = core.Terminal(d.Terminal)
			}
		case core.EvtGateDeny:
			if d, ok := ev.Data.(core.GateDenyData); ok {
				out.rule = d.Rule
			}
		}
	}
	if err := <-errs; err != nil {
		t.Fatalf("engine error: %v", err)
	}
	return out
}

func countEvents(rs runResult, t core.EventType) int {
	n := 0
	for _, ev := range rs.events {
		if ev.Type == t {
			n++
		}
	}
	return n
}

func TestEngineNoEvidenceDenied(t *testing.T) {
	client := &scriptedClient{responses: [][]llm.Chunk{
		{call("w1", "write_file", `{"path":"a.go","content":"package main\n"}`)},
		{doneCall("d1", "改了 a.go")},
		{doneCall("d2", "还是改了 a.go")},
		{doneCall("d3", "仍然改了 a.go")},
	}}
	rs := runEngine(t, newTestEngine(t, client), "改个 bug")
	if rs.terminal != core.TermDenied {
		t.Fatalf("terminal = %s, want denied", rs.terminal)
	}
	if rs.rule == "" {
		t.Fatal("deny must carry a rule")
	}
	if countEvents(rs, core.EvtGateDeny) < 1 || countEvents(rs, core.EvtTurnEnd) != 1 {
		t.Fatalf("events wrong: %+v", rs.events)
	}
}

func TestEngineEvidenceAllowsDone(t *testing.T) {
	client := &scriptedClient{responses: [][]llm.Chunk{
		{call("s1", "shell", `{"command":"echo hi"}`)},
		{doneCall("d1", "执行了命令")},
	}}
	rs := runEngine(t, newTestEngine(t, client), "跑个命令")
	if rs.terminal != core.TermDone {
		t.Fatalf("terminal = %s rule=%s, want done", rs.terminal, rs.rule)
	}
	if countEvents(rs, core.EvtGateAllow) != 1 {
		t.Fatal("missing gate_allow")
	}
	if n := countEvents(rs, core.EvtToolEnd); n != 1 {
		t.Fatalf("tool_end = %d", n)
	}
}

func TestEngineTextOnlyUntested(t *testing.T) {
	client := &scriptedClient{responses: [][]llm.Chunk{
		{{Text: "已经完成了"}},
	}}
	rs := runEngine(t, newTestEngine(t, client), "解释一下")
	if rs.terminal != core.TermUntest {
		t.Fatalf("terminal = %s, want untested", rs.terminal)
	}
	if countEvents(rs, core.EvtAbstain) != 1 {
		t.Fatal("missing abstain event")
	}
}

func TestEngineSelfAssuranceTightensGate(t *testing.T) {
	client := &scriptedClient{responses: [][]llm.Chunk{
		{{Text: "测试全部通过", Call: &llm.ToolCall{ID: "c1", Name: "request_done", Args: `{"summary":"测试通过了"}`}}},
		{doneCall("c2", "仍然测试通过")},
		{doneCall("c3", "坚持测试通过")},
	}}
	rs := runEngine(t, newTestEngine(t, client), "测试通过了吗")
	if rs.terminal != core.TermDenied {
		t.Fatalf("terminal = %s, want denied", rs.terminal)
	}
	warned := false
	for _, ev := range rs.events {
		if ev.Type == core.EvtNotice {
			if d, ok := ev.Data.(core.NoticeData); ok && strings.Contains(d.Text, "自我保证") {
				warned = true
			}
		}
	}
	if !warned {
		t.Fatal("missing self-assurance warning")
	}
}

func TestEngineDeclareUntested(t *testing.T) {
	client := &scriptedClient{responses: [][]llm.Chunk{
		{call("u1", "declare_untested", `{"reason":"无网络","done":["读文件"],"not_done":["跑测试"],"next":"联网后重试"}`)},
	}}
	rs := runEngine(t, newTestEngine(t, client), "跑测试")
	if rs.terminal != core.TermUntest {
		t.Fatalf("terminal = %s", rs.terminal)
	}
}

func TestEngineRemediation(t *testing.T) {
	client := &scriptedClient{responses: [][]llm.Chunk{
		{call("w1", "write_file", `{"path":"a.go","content":"package main\n"}`)},
		{doneCall("d1", "改了 a.go")},
		{call("b1", "verify_build", `{"target":"."}`), call("t1", "verify_test", `{"target":"."}`)},
		{doneCall("d2", "补了构建与测试证据")},
	}}
	rs := runEngine(t, newTestEngine(t, client), "改个 bug")
	if rs.terminal != core.TermDone {
		for _, ev := range rs.events {
			if ev.Type == core.EvtGateDeny {
				t.Logf("DENY %+v", ev.Data)
			}
		}
		t.Fatalf("terminal = %s rule=%s, want done after remediation", rs.terminal, rs.rule)
	}
	if countEvents(rs, core.EvtGateDeny) != 1 || countEvents(rs, core.EvtGateAllow) != 1 {
		t.Fatalf("expected one deny then one allow; got deny=%d allow=%d",
			countEvents(rs, core.EvtGateDeny), countEvents(rs, core.EvtGateAllow))
	}
}
