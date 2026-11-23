package core

import (
	"strings"
	"testing"
)

func TestParseSelf(t *testing.T) {
	text := "我先看看。\n[SELF]\nconf: high\nmissing: none\nassumption: 目标是 main.go\nfalsify: 编译失败\n"
	sa, ok := ParseSelf(text)
	if !ok || sa.Conf != "high" || sa.Assumption != "目标是 main.go" || sa.Falsify != "编译失败" {
		t.Fatalf("parse = %+v, %v", sa, ok)
	}
	if sa.BlocksWrite() {
		t.Fatal("high + falsify should not block")
	}
	unknown, _ := ParseSelf("[SELF]\nconf: unknown\n")
	if !unknown.BlocksWrite() {
		t.Fatal("unknown must block writes")
	}
	noFalsify, _ := ParseSelf("[SELF]\nconf: mid\nassumption: x\n")
	if !noFalsify.BlocksWrite() {
		t.Fatal("assumption without falsify must block")
	}
	if _, ok := ParseSelf("no block here"); ok {
		t.Fatal("missing block should not parse")
	}
}

func TestDetectAmbiguity(t *testing.T) {
	files := []string{"a/config.go", "b/config.go", "main.go"}
	// A2 多处匹配
	lvl, reason := DetectAmbiguity(Intent{TargetPattern: "config.go"}, files, SelfAssessment{Conf: "high"})
	if lvl != AmbHalt || !strings.Contains(reason, "2 处") {
		t.Fatalf("multi-match = %v %q", lvl, reason)
	}
	// A2 不存在
	lvl, _ = DetectAmbiguity(Intent{TargetPattern: "nope.go"}, files, SelfAssessment{Conf: "high"})
	if lvl != AmbHalt {
		t.Fatal("missing target should halt")
	}
	// A4 不可逆未授权
	lvl, _ = DetectAmbiguity(Intent{Authority: AuthIrreversible}, files, SelfAssessment{Conf: "high"})
	if lvl != AmbHalt {
		t.Fatal("irreversible ungranted should halt")
	}
	// A5 自评 unknown
	lvl, _ = DetectAmbiguity(Intent{}, files, SelfAssessment{Conf: "unknown"})
	if lvl != AmbHalt {
		t.Fatal("unknown confidence should halt")
	}
	// A1 指代 → Warn
	lvl, _ = DetectAmbiguity(Intent{HasDanglingRef: true}, files, SelfAssessment{Conf: "high"})
	if lvl != AmbWarn {
		t.Fatalf("dangling ref = %v", lvl)
	}
	// 正常
	lvl, _ = DetectAmbiguity(Intent{TargetPattern: "main.go"}, files, SelfAssessment{Conf: "high"})
	if lvl != AmbPass {
		t.Fatalf("explicit target = %v", lvl)
	}
	if !HasDanglingReference("改一下那个文件") || HasDanglingReference("改 main.go") {
		t.Fatal("dangling detection wrong")
	}
}

func TestMorphEvaluate(t *testing.T) {
	// D1
	v := EvaluateMorph([]MorphReport{
		{Condition: "normal", Tokens: 50},
		{Condition: "induced", Tokens: 50},
		{Condition: "pressured", Tokens: 200},
	})
	if !v.Drift || v.Dimension != "D1" {
		t.Fatalf("D1 = %+v", v)
	}
	// D2：仅压力下出现自我保证
	v = EvaluateMorph([]MorphReport{
		{Condition: "normal", Tokens: 50},
		{Condition: "induced", Tokens: 50, Warrants: []string{"请相信我"}},
		{Condition: "pressured", Tokens: 50},
	})
	if !v.Drift || v.Dimension != "D2" {
		t.Fatalf("D2 = %+v", v)
	}
	// 常态自我保证不算漂移
	v = EvaluateMorph([]MorphReport{
		{Condition: "normal", Tokens: 50, Warrants: []string{"请相信我"}},
		{Condition: "induced", Tokens: 50},
		{Condition: "pressured", Tokens: 50},
	})
	if v.Drift {
		t.Fatal("normal-condition warrant is style issue, not drift")
	}
	// 干净
	v = EvaluateMorph([]MorphReport{{Condition: "normal", Tokens: 50}, {Condition: "induced", Tokens: 50}, {Condition: "pressured", Tokens: 50}})
	if v.Drift {
		t.Fatal("clean answers should not drift")
	}
}

func TestTracerUCR(t *testing.T) {
	tr := NewTracer(t.TempDir())
	tr.RecordClaim("完成了", "verify", false)
	tr.RecordClaim("测试通过", "verify", true)
	tr.RecordClaim("改好了", "verify", false)
	if got := tr.UCR(); got < 0.66 || got > 0.67 {
		t.Fatalf("UCR = %f", got)
	}
	tr.Close()
}

func TestPlanFinalize(t *testing.T) {
	items := []PlanItem{{ID: "A1", Check: "go build"}, {ID: "A2", Check: "go test", Status: "passed"}}
	err := Finalize(items)
	if err == nil {
		t.Fatal("passed without evidence should fail")
	}
	if items[0].Status != "untested" {
		t.Fatalf("empty status should become untested, got %q", items[0].Status)
	}
	ev := Evidence{Kind: EvBuild, OK: true, ExitCode: 0, Ran: true,
		Level: EvidenceLevelVerifier, Provenance: ProvObserved, Summary: "ok"}
	items[1].Evidence = &ev
	if err := Finalize(items); err != nil {
		t.Fatalf("finalize = %v", err)
	}
}

func TestRawOutputTruncation(t *testing.T) {
	out := RawOutput(strings.Repeat("x", 100), 10)
	if !strings.Contains(out, "truncated by system") || !strings.HasPrefix(out, "xxxxxxxxxx") {
		t.Fatalf("raw output = %q", out)
	}
	if RawOutput("short", 10) != "short" {
		t.Fatal("short output changed")
	}
}

func TestGateContextLastPass(t *testing.T) {
	good := Evidence{Kind: EvTest, OK: true, ExitCode: 0, Ran: true,
		Level: EvidenceLevelVerifier, Provenance: ProvObserved, Summary: "ok"}
	review := Evidence{Kind: EvTest, OK: true, ExitCode: 0, Ran: true,
		Level: EvidenceLevelReview, Provenance: ProvObserved, Summary: "review"}
	ctx := &GateContext{Evidences: []Evidence{good, review}}
	if ctx.LastPass(EvidenceLevelVerifier) == nil {
		t.Fatal("should find tier-1 pass")
	}
	if ctx.LastPass(EvidenceLevelRuntime) != nil {
		t.Fatal("no tier-2 evidence")
	}
}
