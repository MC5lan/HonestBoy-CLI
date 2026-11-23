package core

import (
	"strings"
	"testing"
)

// TD05 极端输入演练：判定逻辑对畸形输入不 panic、不误判。
func TestExtremeInputs(t *testing.T) {
	cases := []struct {
		name string
		text string
	}{
		{"emoji only", "👨‍👩‍👧🎉🎉"},
		{"very long line", strings.Repeat("x", 100000)},
		{"invalid utf8 marker", "\xff\xfe"},
		{"empty", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_ = TokenEstimate(tc.text)
			_ = DetectSelfAssurance(tc.text)
			_ = DetectSurrender(tc.text)
			_ = IsHighRiskCommand(tc.text)
		})
	}
	// 单行截断与宽度估算不 panic。
	s := &Session{ID: "s"}
	for i := 0; i < 1000; i++ {
		s.Emit(EvtTextDelta, TextDeltaData{Text: strings.Repeat("中", 100)})
	}
	if s.Seq() != 1000 {
		t.Fatal("seq drift")
	}
}

func TestEmptyAndMalformedPolicy(t *testing.T) {
	r := RequiredFor([]Change{{Path: "", Kind: ""}})
	if !r.NoEvidence {
		t.Fatalf("empty change should need no evidence: %+v", r)
	}
	if RequiredFor(nil).MinCount != 0 {
		t.Fatal("nil changes MinCount")
	}
}
