package core

import "testing"

func TestLoopGuard(t *testing.T) {
	g := &LoopGuard{}
	what, why := "go test ./...", "FAIL: TestX"

	none := &Ledger{}
	if halt, reason := g.Check(none, 1); halt || reason != "" {
		t.Fatalf("no failures should pass: %v %q", halt, reason)
	}

	twice := &Ledger{}
	twice.Record(LedgerEntry{What: what, Why: why})
	twice.Record(LedgerEntry{What: what, Why: why})
	if halt, reason := g.Check(twice, 2); halt || reason != warnRepeatedTwice {
		t.Fatalf("twice = %v %q", halt, reason)
	}

	thrice := &Ledger{}
	for i := 0; i < 3; i++ {
		thrice.Record(LedgerEntry{What: what, Why: why})
	}
	if halt, reason := g.Check(thrice, 3); !halt || reason != haltRepeated3x {
		t.Fatalf("thrice = %v %q", halt, reason)
	}

	if halt, reason := g.Check(&Ledger{}, 41); !halt || reason != haltTooManyCalls {
		t.Fatalf("41 calls = %v %q", halt, reason)
	}
	if halt, reason := g.Check(nil, 1); halt || reason != "" {
		t.Fatalf("nil ledger = %v %q", halt, reason)
	}
}
