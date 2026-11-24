package core

import (
	"strings"
	"testing"
)

func TestLabelsCoverage(t *testing.T) {
	if ToolLabel("read_file") != "读取文件" || ToolLabel("unknown") != "unknown" {
		t.Fatal("ToolLabel mismatch")
	}
	cases := map[string]string{
		"shell":            "执行命令 echo",
		"write_file":       "新增/修改文件 a.go",
		"apply_patch":      "应用补丁",
		"edit_file":        "编辑文件 a.go",
		"read_file":        "读取文件 a.go",
		"list_dir":         "查看目录 a.go",
		"search":           "搜索 foo",
		"request_done":     "申请完成",
		"declare_untested": "声明未验证",
	}
	args := `{"command":"echo","path":"a.go","pattern":"foo"}`
	for tool, want := range cases {
		if got := ToolAction(tool, args); got != want {
			t.Fatalf("ToolAction(%s) = %q, want %q", tool, got, want)
		}
	}
	if ToolAction("custom", "{}") != "custom" {
		t.Fatal("unknown tool action")
	}
	if firstArg("not json", "path") != "" || firstArg(`{"path":1}`, "path") != "" {
		t.Fatal("firstArg should reject non-string")
	}
	if TerminalLabel("done") != "完成" || TerminalLabel("x") != "x" {
		t.Fatal("TerminalLabel mismatch")
	}
	if GateRuleLabel("coverage") != "有改动没验证" || GateRuleLabel("x") != "x" {
		t.Fatal("GateRuleLabel mismatch")
	}
}

func TestLedgerSearchAndRepeat(t *testing.T) {
	l := &Ledger{}
	l.Record(LedgerEntry{TurnID: "t1", What: "build", Why: "undefined", Kind: "go", Tags: []string{"go", "build"}, Files: []string{"a.go"}})
	l.Record(LedgerEntry{TurnID: "t1", What: "build", Why: "undefined", Kind: "go", Tags: []string{"go", "build"}, Files: []string{"a.go"}})
	l.Record(LedgerEntry{TurnID: "t1", What: "build", Why: "other", Kind: "go"})
	if l.Len() != 3 {
		t.Fatalf("len = %d", l.Len())
	}
	if n := l.SameFailureRepeated("build", "undefined"); n != 0 {
		t.Fatalf("repeated = %d, want 0 (last entry differs)", n)
	}
	hits := l.Search(Query{Kind: "go", Tags: []string{"go"}, Files: []string{"a.go"}}, 2, 0.5)
	if len(hits) != 2 || hits[0].Why != "undefined" {
		t.Fatalf("hits = %+v", hits)
	}
	if Score(Query{Kind: "go", Tags: []string{"go"}}, l.Entries()[2]) <= 0 {
		t.Fatal("score should be positive")
	}
	ctx := RenderLedgerContext(hits)
	if !strings.Contains(ctx, "历史失败案例") || !strings.Contains(ctx, "undefined") {
		t.Fatalf("context = %q", ctx)
	}
	if RenderLedgerContext(nil) != "" {
		t.Fatal("empty ledger context")
	}
}

func TestEvidenceLevelAndSessionState(t *testing.T) {
	if EvidenceLevelVerifier.String() != "verifier" || EvidenceLevelRuntime.String() != "runtime" ||
		EvidenceLevelReview.String() != "review" || EvidenceLevelNone.String() != "none" {
		t.Fatal("evidence level strings")
	}
	if ParseEvidenceLevel("1") != EvidenceLevelVerifier || ParseEvidenceLevel("2") != EvidenceLevelRuntime ||
		ParseEvidenceLevel("3") != EvidenceLevelReview || ParseEvidenceLevel("x") != EvidenceLevelNone {
		t.Fatal("parse evidence level")
	}
	s := &Session{}
	s.SetSeq(42)
	if s.Seq() != 42 {
		t.Fatal("SetSeq")
	}
	s.Receipts = []Receipt{{Hash: "abc"}}
	if s.ChainTail() != "abc" {
		t.Fatal("ChainTail")
	}
}
