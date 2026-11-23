package engine

import (
	"testing"

	"github.com/MC5lan/HonestBoy-CLI/internal/core"
	"github.com/MC5lan/HonestBoy-CLI/internal/llm"
)

// 自评 conf=unknown 时必须阻断写操作并转 HALT（文档 §4.1.3）。
func TestSelfAssessmentBlocksWrite(t *testing.T) {
	client := &scriptedClient{responses: [][]llm.Chunk{
		{
			{Text: "[SELF]\nconf: unknown\nmissing: 不知道要改哪个文件\n"},
			call("w", "write_file", `{"path":"a.go","content":"package main\n"}`),
		},
	}}
	eng := newTestEngine(t, client)
	eng.cfg.SelfAssessment = true
	rs := runEngine(t, eng, "改个文件")
	if rs.terminal != core.TermHalt {
		t.Fatalf("terminal = %s, want halt", rs.terminal)
	}
	if countEvents(rs, core.EvtToolStart) != 0 {
		t.Fatal("write must not execute when conf=unknown")
	}
}

// conf=high 且有证伪条件时放行。
func TestSelfAssessmentAllowsWrite(t *testing.T) {
	client := &scriptedClient{responses: [][]llm.Chunk{
		{
			{Text: "[SELF]\nconf: high\nmissing: none\nassumption: none\n"},
			call("w", "write_file", `{"path":"ok.txt","content":"hello"}`),
		},
		{{Text: "[SELF]\nconf: high\nmissing: none\nassumption: none\n"}, doneCall("d", "完成")},
	}}
	eng := newTestEngine(t, client)
	eng.cfg.SelfAssessment = true
	rs := runEngine(t, eng, "跑命令")
	if rs.terminal != core.TermDone {
		t.Fatalf("terminal = %s", rs.terminal)
	}
	if countEvents(rs, core.EvtToolStart) == 0 {
		t.Fatal("write should execute with high confidence")
	}
}

// 未输出自评块按 unknown 处理。
func TestMissingSelfAssessmentBlocksWrite(t *testing.T) {
	client := &scriptedClient{responses: [][]llm.Chunk{
		{call("w", "write_file", `{"path":"b.go","content":"package main\n"}`)},
	}}
	eng := newTestEngine(t, client)
	eng.cfg.SelfAssessment = true
	rs := runEngine(t, eng, "改文件")
	if rs.terminal != core.TermHalt {
		t.Fatalf("terminal = %s, want halt", rs.terminal)
	}
}
