package core

import (
	"reflect"
	"testing"
)

func TestRequiredForFiveScenarios(t *testing.T) {
	cases := []struct {
		name     string
		changes  []Change
		wantKind []EvidenceKind
		wantEcho bool
		wantTest bool
		wantNone bool
		wantTgt  []string
	}{
		{"go-file", []Change{{Path: "internal/core/gate.go", Kind: ChangeGo}},
			[]EvidenceKind{EvBuild, EvTest, EvDiff}, false, false, false, []string{"internal/core/gate.go"}},
		{"doc-file", []Change{{Path: "docs/readme.md", Kind: ChangeDoc}},
			[]EvidenceKind{EvFile, EvDiff}, true, false, false, []string{"docs/readme.md"}},
		{"command", []Change{{Kind: ChangeCommand}},
			[]EvidenceKind{EvCommand}, false, false, false, nil},
		{"answer-only", []Change{{Kind: ChangeAnswer}},
			nil, false, false, true, nil},
		{"bugfix", []Change{{Path: "internal/tools/edit.go", Kind: ChangeBugfix}},
			[]EvidenceKind{EvTest, EvBuild, EvDiff}, false, true, false, []string{"internal/tools/edit.go"}},
		{"no-changes", nil, nil, false, false, true, nil},
		{"yaml-config", []Change{{Path: "config.yaml"}},
			[]EvidenceKind{EvFile, EvDiff}, true, false, false, []string{"config.yaml"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := RequiredFor(tc.changes)
			if !reflect.DeepEqual(got.Kinds, tc.wantKind) {
				t.Fatalf("Kinds = %v, want %v", got.Kinds, tc.wantKind)
			}
			if got.NeedReadEcho != tc.wantEcho || got.NeedTestName != tc.wantTest || got.NoEvidence != tc.wantNone {
				t.Fatalf("flags = %+v", got)
			}
			if !reflect.DeepEqual(got.Targets, tc.wantTgt) {
				t.Fatalf("Targets = %v, want %v", got.Targets, tc.wantTgt)
			}
			if got.MinCount != len(got.Kinds) {
				t.Fatalf("MinCount = %d, want %d", got.MinCount, len(got.Kinds))
			}
		})
	}
}

func TestRequiredForDataDriven(t *testing.T) {
	if len(changeRules) < 5 {
		t.Fatal("policy rules must be a data table with 5+ entries")
	}
}
