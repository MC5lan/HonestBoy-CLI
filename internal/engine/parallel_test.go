package engine

import (
	"context"
	"encoding/json"
	"sync/atomic"
	"testing"
	"time"

	"github.com/MC5lan/HonestBoy-CLI/internal/llm"
	"github.com/MC5lan/HonestBoy-CLI/internal/tools"
)

type parallelReadTool struct {
	active int32
	max    int32
}

func (p *parallelReadTool) Name() string            { return "parallel_read" }
func (p *parallelReadTool) Description() string     { return "test" }
func (p *parallelReadTool) Schema() json.RawMessage { return json.RawMessage(`{"type":"object"}`) }
func (p *parallelReadTool) Risk() tools.Risk        { return tools.RiskRead }
func (p *parallelReadTool) Run(context.Context, json.RawMessage) (tools.Result, error) {
	n := atomic.AddInt32(&p.active, 1)
	for {
		old := atomic.LoadInt32(&p.max)
		if n <= old || atomic.CompareAndSwapInt32(&p.max, old, n) {
			break
		}
	}
	time.Sleep(40 * time.Millisecond)
	atomic.AddInt32(&p.active, -1)
	return tools.Result{OK: true, Output: "ok"}, nil
}

func TestPrefetchReadOnlyRunsConcurrently(t *testing.T) {
	tool := &parallelReadTool{}
	reg := tools.NewRegistry()
	if err := reg.Register(tool); err != nil {
		t.Fatal(err)
	}
	e := &Engine{
		registry: reg,
		approval: ApprovalPolicy{Write: "auto", Exec: "auto", Net: "auto"},
	}
	calls := []llm.ToolCall{
		{ID: "1", Name: "parallel_read", Args: "{}"},
		{ID: "2", Name: "parallel_read", Args: "{}"},
	}
	out := e.prefetchReadOnly(context.Background(), calls)
	if len(out) != 2 || !out[0].done || !out[1].done {
		t.Fatalf("out = %+v", out)
	}
	if got := atomic.LoadInt32(&tool.max); got < 2 {
		t.Fatalf("max concurrency = %d, want >= 2", got)
	}
}
