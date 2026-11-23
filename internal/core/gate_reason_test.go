package core

import (
	"strings"
	"testing"
)

func TestReasonFor(t *testing.T) {
	cases := []struct {
		name     string
		d        Decision
		modelKey string
		userKey  string
	}{
		{"coverage", Decision{Rule: "coverage", Missing: []string{"./pkg"}}, "./pkg", "1 个改动"},
		{"evidence_failed", Decision{Rule: "evidence_failed", Summary: "FAIL: TestX"}, "FAIL: TestX", "验证失败"},
		{"provenance", Decision{Rule: "provenance", Provenance: "inferred"}, "inferred", "证据来源不足"},
		{"evidence_kind", Decision{Rule: "evidence_kind", Needed: []string{"build"}, Got: "command"}, "build", "证据类型不符"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := ReasonFor(tc.d, true)
			u := ReasonFor(tc.d, false)
			if !strings.Contains(m, tc.modelKey) || !strings.Contains(u, tc.userKey) {
				t.Fatalf("model=%q user=%q", m, u)
			}
		})
	}
	if got := ReasonFor(Decision{Reason: "自定义"}, true); got != "自定义" {
		t.Fatalf("explicit reason should be kept: %q", got)
	}
}

func TestClaimOverridesSelfReported(t *testing.T) {
	c := NewClaim("完成", []Change{{Path: "a.go", Kind: ChangeGo}}).
		WithTargets([]string{"a.go"}).
		WithSelfReported(Requirement{NoEvidence: true})
	if len(c.Req.Kinds) != 3 || c.Req.NoEvidence {
		t.Fatalf("default policy not applied: %+v", c.Req)
	}
	if !c.SelfReported.NoEvidence {
		t.Fatal("self reported value should be kept for audit only")
	}
	if len(c.Targets) != 1 || c.Targets[0] != "a.go" {
		t.Fatalf("targets = %v", c.Targets)
	}
}
