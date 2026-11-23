package engine

import (
	"os"
	"strings"
	"testing"

	"github.com/MC5lan/HonestBoy-CLI/internal/core"
	"github.com/MC5lan/HonestBoy-CLI/internal/llm"
)

// 回放样本：脚本化模型 + 已落盘会话，断言终态与证据集合可复现（TA05）。
type replayCase struct {
	name    string
	prompt  string
	script  [][]llm.Chunk
	want    core.Terminal
	wantEvt core.EventType
}

var replayCases = []replayCase{
	{"done", "跑命令", [][]llm.Chunk{
		{call("s", "shell", `{"command":"echo hi"}`)}, {doneCall("d", "完成")},
	}, core.TermDone, core.EvtGateAllow},
	{"denied", "改 bug", [][]llm.Chunk{
		{call("w", "write_file", `{"path":"a.go","content":"package main\n"}`)},
		{doneCall("d1", "完成")}, {doneCall("d2", "完成")}, {doneCall("d3", "完成")},
	}, core.TermDenied, core.EvtGateDeny},
	{"untested", "解释", [][]llm.Chunk{{{Text: "已完成"}}}, core.TermUntest, core.EvtAbstain},
}

func TestReplayCases(t *testing.T) {
	for _, tc := range replayCases {
		t.Run(tc.name, func(t *testing.T) {
			eng := newTestEngine(t, &scriptedClient{responses: tc.script})
			rs := runEngine(t, eng, tc.prompt)
			if rs.terminal != tc.want {
				t.Fatalf("terminal = %s, want %s", rs.terminal, tc.want)
			}
			if countEvents(rs, tc.wantEvt) == 0 {
				t.Fatalf("missing %s event", tc.wantEvt)
			}
		})
	}
}

func TestReplayFromSessionFile(t *testing.T) {
	eng := newTestEngine(t, &scriptedClient{responses: replayCases[0].script})
	rs := runEngine(t, eng, "跑命令")
	if len(rs.events) == 0 {
		t.Fatal("no events")
	}
	// 事件已落盘，重新加载应能完整重建（seq、终态、Receipt）。
	ss := eng.store
	entries, err := os.ReadDir(ss.Paths.Sessions)
	if err != nil || len(entries) == 0 {
		t.Fatalf("no session file: %v", err)
	}
	id := strings.TrimSuffix(entries[0].Name(), ".jsonl")
	sess, err := ss.Load(id)
	if err != nil {
		t.Fatal(err)
	}
	if len(sess.Events) != len(rs.events) || sess.Seq() == 0 {
		t.Fatalf("replay mismatch: events=%d/%d seq=%d", len(sess.Events), len(rs.events), sess.Seq())
	}
	if len(sess.Receipts) != 1 {
		t.Fatalf("receipts = %d", len(sess.Receipts))
	}
}
