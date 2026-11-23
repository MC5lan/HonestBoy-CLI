package core

import (
	"strings"
	"testing"
)

func TestFormatEvidence(t *testing.T) {
	ok := FormatEvidence(Evidence{Kind: EvBuild, Target: "./internal/core", OK: true, Duration: 412, Summary: "ok  github.com/x/core 0.31s"})
	if !strings.HasPrefix(ok, "✓") || !strings.Contains(ok, "build") || !strings.Contains(ok, "412ms") {
		t.Fatalf("format = %q", ok)
	}
	fail := FormatEvidence(Evidence{Kind: EvTest, Target: "./internal/core", Duration: 1200, Summary: "FAIL: TestEdit"})
	if !strings.HasPrefix(fail, "✗") || !strings.Contains(fail, "1.2s") {
		t.Fatalf("format = %q", fail)
	}
}

func TestFormatEvidenceCompact(t *testing.T) {
	short := FormatEvidenceCompact(Evidence{Kind: EvBuild, Target: "./x", Summary: "ok"})
	if strings.Contains(short, "\n") {
		t.Fatalf("short evidence should stay one line: %q", short)
	}
	long := FormatEvidenceCompact(Evidence{Kind: EvBuild, Target: "./x", Summary: strings.Repeat("很长的原文摘要", 20)})
	if !strings.Contains(long, "\n") {
		t.Fatalf("long evidence should wrap: %q", long)
	}
}
