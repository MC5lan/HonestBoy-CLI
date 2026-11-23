package core

import (
	"testing"
	"time"
)

// 论文 §3 定义 3 / §6.3：接地谓词必须对所有非验证器来源、未运行、
// 负退出码、非一级情形返回否，并可静态验证。
func TestEvidenceGroundingPredicate(t *testing.T) {
	base := func() Evidence {
		e := Evidence{ID: "e1", Kind: EvTest, Target: "pkg", OK: true, ExitCode: 0,
			Summary: "ok", Raw: "ok", Level: EvidenceLevelVerifier, Ran: true,
			Provenance: ProvObserved}
		e.RawHash = e.ComputeRawHash()
		return e
	}
	if !base().CanGround() {
		t.Fatal("valid verifier evidence must ground")
	}
	cases := []struct {
		name string
		mut  func(*Evidence)
	}{
		{"来源 inferred", func(e *Evidence) { e.Provenance = ProvInferred }},
		{"来源 assumed", func(e *Evidence) { e.Provenance = ProvAssumed }},
		{"未运行", func(e *Evidence) { e.Ran = false }},
		{"负退出码", func(e *Evidence) { e.ExitCode = -1 }},
		{"未通过", func(e *Evidence) { e.OK = false }},
		{"二级", func(e *Evidence) { e.Level = EvidenceLevelRuntime }},
		{"三级评审", func(e *Evidence) { e.Level = EvidenceLevelReview }},
		{"未标注级别", func(e *Evidence) { e.Level = EvidenceLevelNone }},
		{"原文被改写", func(e *Evidence) { e.Raw = "PASS (rewritten)" }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := base()
			tc.mut(&e)
			if e.CanGround() {
				t.Fatal("evidence must not ground")
			}
		})
	}
	if ParseEvidenceLevel("bogus") != EvidenceLevelNone || ParseEvidenceLevel("3") != EvidenceLevelReview {
		t.Fatal("ParseEvidenceLevel wrong")
	}
}

func TestGateRejectsReviewEvidence(t *testing.T) {
	review := Evidence{ID: "e1", Kind: EvTest, Target: "pkg", OK: true, Summary: "评审认可",
		Level: EvidenceLevelReview, Ran: true, Provenance: ProvObserved}
	review.RawHash = review.ComputeRawHash()
	bag := EvidenceBag{}
	bag.Add(review)
	claim := Claim{Req: Requirement{Kinds: []EvidenceKind{EvTest}, Targets: []string{"pkg"}, MinCount: 1},
		Targets: []string{"pkg"}}
	d := NewGate(Policy{Strict: true}).Evaluate(claim, bag)
	if d.Allow || d.Rule != "provenance" {
		t.Fatalf("review must not ground: %+v", d)
	}
}

func TestGateTreatsNotRunAsMissing(t *testing.T) {
	notRun := Evidence{ID: "e1", Kind: EvTest, Target: "pkg", OK: false, Summary: "验证器未运行",
		Level: EvidenceLevelVerifier, Ran: false, Provenance: ProvObserved}
	bag := EvidenceBag{}
	bag.Add(notRun)
	claim := Claim{Req: Requirement{Kinds: []EvidenceKind{EvTest}, Targets: []string{"pkg"}, MinCount: 1},
		Targets: []string{"pkg"}}
	d := NewGate(Policy{Strict: true}).Evaluate(claim, bag)
	if d.Allow || d.Rule != "coverage" {
		t.Fatalf("not-run evidence should be missing, not failed: %+v", d)
	}
}

func TestVerifyChainDetectsEvidenceTamper(t *testing.T) {
	at := time.Now().UTC()
	ev := Evidence{ID: "e1", Kind: EvTest, Summary: "ok", Raw: "ok",
		Level: EvidenceLevelVerifier, Ran: true, Provenance: ProvObserved}
	ev.RawHash = ev.ComputeRawHash()
	r := NewReceipt("rc1", "s", "t", TermDone, "claim", []Evidence{ev}, ZeroHash, at)
	if at2, err := VerifyChain([]Receipt{r}); at2 != -1 || err != nil {
		t.Fatalf("chain should be valid: %d %v", at2, err)
	}
	tampered := r
	tampered.Evidence = []Evidence{ev}
	tampered.Evidence[0].Raw = "rewritten"
	if at2, err := VerifyChain([]Receipt{tampered}); at2 != 0 || err != ErrChainBroken {
		t.Fatalf("evidence tamper not detected: %d %v", at2, err)
	}
}
