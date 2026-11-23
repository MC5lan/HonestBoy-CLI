package engine

import (
	"testing"

	"github.com/MC5lan/HonestBoy-CLI/internal/core"
	"github.com/MC5lan/HonestBoy-CLI/internal/llm"
)

// 回归：失败的工具不能因为“没有 change”而被 Gate 当成纯问答放行。
func TestFailedToolCannotReachDone(t *testing.T) {
	client := &scriptedClient{responses: [][]llm.Chunk{
		{call("w", "edit_file", `{"path":"missing.go","old":"x","new":"y"}`)},
		{doneCall("d", "改好了")},
	}}
	eng := newTestEngine(t, client)
	rs := runEngine(t, eng, "改文件")
	if rs.terminal == core.TermDone {
		t.Fatal("failed edit must not reach done")
	}
	if countEvents(rs, core.EvtGateDeny) == 0 {
		t.Fatal("expected gate_deny after failed tool")
	}
}

func TestFailedVerifyCannotReachDone(t *testing.T) {
	client := &scriptedClient{responses: [][]llm.Chunk{
		{call("v", "verify_test", `{"target":"."}`)},
		{doneCall("d", "测试通过")},
	}}
	eng := newTestEngine(t, client)
	rs := runEngine(t, eng, "跑测试")
	if rs.terminal == core.TermDone {
		t.Fatal("failed verification must not reach done")
	}
}
