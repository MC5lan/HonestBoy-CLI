package core

import "testing"

func TestLedgerSameFailureRepeated(t *testing.T) {
	l := &Ledger{}
	for i := 0; i < 3; i++ {
		l.Record(LedgerEntry{What: "build ./internal/tools", Why: "syntax error at line 12"})
	}
	if got := l.SameFailureRepeated("build ./internal/tools", "syntax error at line 12"); got != 3 {
		t.Fatalf("count = %d, want 3", got)
	}
	l.Record(LedgerEntry{What: "test ./internal/tools", Why: "FAIL: TestX"})
	if got := l.SameFailureRepeated("build ./internal/tools", "syntax error at line 12"); got != 0 {
		t.Fatalf("different failure must not merge: %d", got)
	}
	if l.Len() != 4 {
		t.Fatalf("Len = %d", l.Len())
	}
	if len(l.Entries()) != 4 || l.Entries()[0].Attempt != 1 {
		t.Fatalf("entries wrong: %+v", l.Entries())
	}
}
