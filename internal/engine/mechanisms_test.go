package engine

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MC5lan/HonestBoy-CLI/internal/core"
	"github.com/MC5lan/HonestBoy-CLI/internal/llm"
	"github.com/MC5lan/HonestBoy-CLI/internal/store"
	"github.com/MC5lan/HonestBoy-CLI/internal/tools"
)

// 文档 §4.2：含悬空指代的输入必须暂停，不能瞎猜目标。
func TestAmbiguityHalt(t *testing.T) {
	client := &scriptedClient{responses: [][]llm.Chunk{
		{call("w", "write_file", `{"path":"a.go","content":"package main\n"}`)},
	}}
	eng := newTestEngine(t, client)
	eng.cfg.AmbiguityHalt = true
	rs := runEngine(t, eng, "把那个文件改一下")
	if rs.terminal != core.TermHalt {
		t.Fatalf("terminal = %s, want halt", rs.terminal)
	}
	if countEvents(rs, core.EvtToolStart) != 0 {
		t.Fatal("must not execute tools on ambiguous input")
	}
}

func TestAmbiguityDisabled(t *testing.T) {
	client := &scriptedClient{responses: [][]llm.Chunk{
		{call("w", "write_file", `{"path":"a.go","content":"package main\n"}`)},
		{doneCall("d", "完成")},
	}}
	eng := newTestEngine(t, client)
	eng.cfg.AmbiguityHalt = false
	rs := runEngine(t, eng, "把那个文件改一下")
	if rs.terminal == core.TermHalt {
		t.Fatal("disabled ambiguity should not halt")
	}
}

// 文档 §5：自我保证措辞只警告，不改判据。
func TestMorphWarnsButDoesNotBlock(t *testing.T) {
	client := &scriptedClient{responses: [][]llm.Chunk{
		{call("s", "shell", `{"command":"echo hi"}`)},
		{{Text: "请相信我，我保证没问题"}, doneCall("d", "完成")},
	}}
	eng := newTestEngine(t, client)
	eng.cfg.AmbiguityHalt = false
	rs := runEngine(t, eng, "跑命令")
	warned := false
	for _, ev := range rs.events {
		if ev.Type == core.EvtNotice {
			if d, ok := ev.Data.(core.NoticeData); ok && strings.Contains(d.Text, "自我保证") {
				warned = true
			}
		}
	}
	if !warned {
		t.Fatal("expected self-assurance warning")
	}
	// 自我保证会触发 Gate 加严（T410），因此不能直接 done。
	if rs.terminal == core.TermDone {
		t.Fatal("self-assurance must tighten the gate, not pass silently")
	}
}

// 文档 §3.5：历史失败案例按标签检索并注入系统提示。
func TestLedgerRecallInjects(t *testing.T) {
	client := &scriptedClient{responses: [][]llm.Chunk{
		{{Text: "好的"}},
	}}
	eng := newTestEngine(t, client)
	eng.cfg.LedgerRecall = true
	// 预置一条与输入标签匹配的历史失败
	eng.recordFailure(core.LedgerEntry{
		What: "go test", Why: "FAIL: TestEdit", Kind: "go test",
		Tags: extractTags("修复 edit 的测试失败"),
	})
	rs := runEngine(t, eng, "修复 edit 的测试失败")
	found := false
	for _, ev := range rs.events {
		if ev.Type == core.EvtNotice {
			if d, ok := ev.Data.(core.NoticeData); ok && strings.Contains(d.Text, "历史失败") {
				found = true
			}
		}
	}
	if !found {
		t.Fatal("expected ledger recall notice")
	}
}

// 文档 §3.5：账本落盘后能被新引擎加载。
func TestLedgerPersists(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "ledger.jsonl"),
		[]byte(`{"what":"build","why":"undefined: foo","kind":"build"}`+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module x\n\ngo 1.27\n"), 0o600)
	sb, err := tools.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { sb.Close() })
	eng := New(Config{
		Workdir: dir, Model: "t", Client: &scriptedClient{},
		Registry: tools.NewDefaultRegistry(sb, tools.NewReadGuard(), "cmd"),
		Store: store.NewSessionStore(store.Paths{
			Root: root, Sessions: filepath.Join(root, "sessions"), Logs: filepath.Join(root, "logs"),
		}),
	})
	t.Cleanup(func() {
		if eng.ledgerFile != nil {
			eng.ledgerFile.Close()
		}
	})
	if eng.ledger.Len() == 0 {
		t.Fatal("ledger not loaded from disk")
	}
	entries := eng.ledger.Entries()
	if entries[0].Why != "undefined: foo" {
		t.Fatalf("entry = %+v", entries[0])
	}
}

// 文档 §4.3：测试计划解析与三态校验。
func TestPlanParsedAndFinalized(t *testing.T) {
	client := &scriptedClient{responses: [][]llm.Chunk{
		{
			{Text: "[SELF]\nconf: high\nmissing: none\nassumption: none\n[TEST PLAN]\n- A1: go build ./...\n"},
			call("s", "shell", `{"command":"echo hi"}`),
		},
		{{Text: "[SELF]\nconf: high\nmissing: none\nassumption: none\n"}, doneCall("d", "完成")},
	}}
	eng := newTestEngine(t, client)
	eng.cfg.SelfAssessment = true
	eng.cfg.PlanFirst = true
	rs := runEngine(t, eng, "跑命令")
	// 计划被记录（notice）
	found := false
	for _, ev := range rs.events {
		if ev.Type == core.EvtNotice {
			if d, ok := ev.Data.(core.NoticeData); ok && strings.Contains(d.Text, "测试计划") {
				found = true
			}
		}
	}
	if !found {
		t.Fatal("expected test plan notice")
	}
}
