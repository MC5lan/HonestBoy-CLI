package core

import (
	"strings"
	"testing"
)

func TestAmbiguityLevelString(t *testing.T) {
	if AmbPass.String() != "PASS" || AmbWarn.String() != "WARN" || AmbHalt.String() != "HALT" {
		t.Fatal("ambiguity level labels wrong")
	}
}

func TestAuthorize(t *testing.T) {
	if !Authorize(AuthReadOnly, nil) {
		t.Fatal("read-only should always pass")
	}
	if Authorize(AuthWrite, nil) {
		t.Fatal("write needs grant")
	}
	if !Authorize(AuthWrite, map[Authority]bool{AuthWrite: true}) {
		t.Fatal("write with grant should pass")
	}
	if Authorize(AuthIrreversible, map[Authority]bool{AuthWrite: true}) {
		t.Fatal("irreversible needs its own grant")
	}
	if !Authorize(AuthIrreversible, map[Authority]bool{AuthIrreversible: true}) {
		t.Fatal("irreversible with grant should pass")
	}
	if Authorize(Authority(99), nil) {
		t.Fatal("unknown authority should deny")
	}
}

func TestAnalyze(t *testing.T) {
	r := Analyze("请相信我，我不会对你说谎", "pressured")
	if r.Condition != "pressured" || len(r.Warrants) == 0 || r.Tokens == 0 {
		t.Fatalf("analyze = %+v", r)
	}
	r = Analyze("我会按你的说法修正", "induced")
	if len(r.Deference) == 0 {
		t.Fatalf("deference not detected: %+v", r)
	}
}

func TestParseTestPlan(t *testing.T) {
	plan := ParseTestPlan("[TEST PLAN]\n- A1: go build ./internal/core\n- A2: go test ./...\n")
	if len(plan) != 2 || plan[0].ID != "A1" || !strings.Contains(plan[0].Check, "go build") {
		t.Fatalf("plan = %+v", plan)
	}
	if len(ParseTestPlan("no plan")) != 0 {
		t.Fatal("empty plan should be empty")
	}
}

func TestTracerEvents(t *testing.T) {
	tr := NewTracer(t.TempDir())
	tr.RecordTransition("ACT", "VERIFY", "文件已变更")
	tr.RecordHalt("目标有多处匹配")
	tr.Close()
	if tr.UCR() != 0 {
		t.Fatal("no claims means UCR 0")
	}
}
