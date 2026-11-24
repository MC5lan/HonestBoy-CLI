package core

import "testing"

// TestGateCounterexampleMatrix 补足 Gate 反例矩阵；与 TestGateSixCases 合计 ≥20。
func TestGateCounterexampleMatrix(t *testing.T) {
	base := gateEvidence("e1", EvBuild, "pkg", true, ProvObserved)
	claim := Claim{
		Req:     Requirement{Kinds: []EvidenceKind{EvBuild}, Targets: []string{"pkg"}, MinCount: 1},
		Targets: []string{"pkg"},
	}
	with := func(mut func(*Evidence)) Evidence {
		e := base
		mut(&e)
		return e
	}
	cases := []struct {
		name string
		bag  EvidenceBag
		rule string
	}{
		{"ran-false", bagOf(with(func(e *Evidence) { e.Ran = false })), "coverage"},
		{"exit-negative", bagOf(with(func(e *Evidence) { e.ExitCode = -1 })), "provenance"},
		{"runtime-level", bagOf(with(func(e *Evidence) { e.Level = EvidenceLevelRuntime })), "provenance"},
		{"assumed", bagOf(with(func(e *Evidence) { e.Provenance = ProvAssumed })), "provenance"},
		{"unknown", bagOf(with(func(e *Evidence) { e.Provenance = ProvUnknown })), "provenance"},
		{"raw-hash-mismatch", bagOf(with(func(e *Evidence) { e.Raw = "x"; e.RawHash = "deadbeef" })), "provenance"},
		{"min-count", bagOf(base), "coverage"},
		{"target-mismatch", bagOf(with(func(e *Evidence) { e.Target = "other" })), "coverage"},
		{"kind-mismatch", bagOf(with(func(e *Evidence) { e.Kind = EvCommand })), "evidence_kind"},
		{"empty-kind", bagOf(with(func(e *Evidence) { e.Kind = "" })), "evidence_kind"},
		{"failed", bagOf(with(func(e *Evidence) { e.OK = false })), "evidence_failed"},
	}
	gate := NewGate(Policy{Strict: true})
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := claim
			if tc.name == "min-count" {
				req.Req.Kinds = []EvidenceKind{EvBuild, EvTest}
				req.Req.MinCount = 2
			}
			d := gate.Evaluate(req, tc.bag)
			if d.Allow {
				t.Fatalf("unexpected allow: %+v", d)
			}
			if d.Rule != tc.rule {
				t.Fatalf("rule = %q, want %q (%s)", d.Rule, tc.rule, d.Reason)
			}
		})
	}
}
