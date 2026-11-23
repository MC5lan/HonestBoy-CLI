package engine

import (
	"os"
	"path/filepath"
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
	setup   func(t *testing.T, eng *Engine)
}

// 12 个回放样本，覆盖全部终态与关键路径（TA05 要求 ≥10）。
var replayCases = []replayCase{
	{"done-shell", "跑命令", [][]llm.Chunk{
		{call("s", "shell", `{"command":"echo hi"}`)}, {doneCall("d", "完成")},
	}, core.TermDone, core.EvtGateAllow, nil},
	{"done-write-build-test", "改代码", [][]llm.Chunk{
		{call("w", "write_file", `{"path":"a.go","content":"package main\n"}`)},
		{call("b", "verify_build", `{"target":"."}`), call("t", "verify_test", `{"target":"."}`)},
		{doneCall("d", "完成")},
	}, core.TermDone, core.EvtGateAllow, nil},
	{"denied-no-evidence", "改 bug", [][]llm.Chunk{
		{call("w", "write_file", `{"path":"a.go","content":"package main\n"}`)},
		{doneCall("d1", "完成")}, {doneCall("d2", "完成")}, {doneCall("d3", "完成")},
	}, core.TermDenied, core.EvtGateDeny, nil},
	{"denied-failed-verify", "跑测试", [][]llm.Chunk{
		{call("v", "verify_test", `{"target":"."}`)},
		{doneCall("d1", "完成")}, {doneCall("d2", "完成")}, {doneCall("d3", "完成")},
	}, core.TermDenied, core.EvtGateDeny, func(t *testing.T, eng *Engine) {
		err := os.WriteFile(filepath.Join(eng.cfg.Workdir, "fail_test.go"),
			[]byte("package x\n\nimport \"testing\"\n\nfunc TestFail(t *testing.T) { t.Fatal(\"boom\") }\n"), 0o600)
		if err != nil {
			t.Fatal(err)
		}
	}},
	{"untested-text-only", "解释", [][]llm.Chunk{{{Text: "已完成"}}}, core.TermUntest, core.EvtAbstain, nil},
	{"untested-declare", "跑测试", [][]llm.Chunk{
		{call("u", "declare_untested", `{"reason":"无网络","done":["读文件"],"not_done":["跑测试"],"next":"联网后重试"}`)},
	}, core.TermUntest, core.EvtAbstain, nil},
	{"halt-ambiguous", "把那个文件改一下", [][]llm.Chunk{
		{call("w", "write_file", `{"path":"a.go","content":"package main\n"}`)},
	}, core.TermHalt, core.EvtNotice, func(t *testing.T, eng *Engine) {
		eng.cfg.AmbiguityHalt = true
	}},
	{"halt-self-unknown", "改文件", [][]llm.Chunk{
		{call("w", "write_file", `{"path":"a.go","content":"package main\n"}`)},
		{call("w2", "write_file", `{"path":"b.go","content":"package main\n"}`)},
	}, core.TermHalt, core.EvtNotice, func(t *testing.T, eng *Engine) {
		eng.cfg.SelfAssessment = true
	}},
	{"done-command", "看目录", [][]llm.Chunk{
		{call("s", "shell", `{"command":"echo a"}`), call("s2", "shell", `{"command":"echo b"}`)},
		{doneCall("d", "完成")},
	}, core.TermDone, core.EvtGateAllow, nil},
	{"untested-no-tool", "只回答", [][]llm.Chunk{{{Text: "这是答案"}}}, core.TermUntest, core.EvtAbstain, nil},
	{"denied-partial-coverage", "改两个文件", [][]llm.Chunk{
		{call("w", "write_file", `{"path":"a.go","content":"package main\n"}`),
			call("w2", "write_file", `{"path":"b.go","content":"package main\n"}`)},
		{call("b", "verify_build", `{"target":"a.go"}`)},
		{doneCall("d1", "完成")}, {doneCall("d2", "完成")}, {doneCall("d3", "完成")},
	}, core.TermDenied, core.EvtGateDeny, nil},
	{"done-read-only", "读 go.mod", [][]llm.Chunk{
		{call("r", "read_file", `{"path":"go.mod"}`)},
		{doneCall("d", "完成")},
	}, core.TermDone, core.EvtGateAllow, nil},
}

func TestReplayCases(t *testing.T) {
	for _, tc := range replayCases {
		t.Run(tc.name, func(t *testing.T) {
			eng := newTestEngine(t, &scriptedClient{responses: tc.script})
			if tc.setup != nil {
				tc.setup(t, eng)
			}
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
