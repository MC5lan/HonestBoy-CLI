package core

import (
	"crypto/sha256"
	"testing"
	"time"
)

// TA09 性能基线；结果记录到 docs/perf-baseline.md。
func BenchmarkEmitEvents(b *testing.B) {
	s := &Session{ID: "bench"}
	for i := 0; i < b.N; i++ {
		s.Emit(EvtNotice, NoticeData{Text: "x"})
	}
}

func BenchmarkReceiptHash(b *testing.B) {
	r := NewReceipt("rc", "s", "t", TermDone, "claim", nil, ZeroHash, time.Now().UTC())
	for i := 0; i < b.N; i++ {
		r.ComputeHash()
	}
}

func BenchmarkGateEvaluate(b *testing.B) {
	g := NewGate(Policy{Strict: true})
	bag := EvidenceBag{}
	bag.Add(Evidence{ID: "e1", Kind: EvBuild, Target: "pkg", OK: true, Level: EvidenceLevelVerifier,
		Ran: true, Summary: "ok", Provenance: ProvObserved})
	claim := Claim{Req: Requirement{Kinds: []EvidenceKind{EvBuild}, Targets: []string{"pkg"}, MinCount: 1}, Targets: []string{"pkg"}}
	for i := 0; i < b.N; i++ {
		g.Evaluate(claim, bag)
	}
}

func BenchmarkSHA256(b *testing.B) {
	for i := 0; i < b.N; i++ {
		sha256.Sum256([]byte("evidence raw output"))
	}
}
