package core

import "testing"

func ev(id string, kind EvidenceKind, target string, ok bool) Evidence {
	return Evidence{ID: id, Kind: kind, Target: target, OK: ok, Summary: "verifier line",
		Level: EvidenceLevelVerifier, Ran: true, Provenance: ProvObserved}
}

func TestEvidenceBagRejectsEmptySummary(t *testing.T) {
	b := &EvidenceBag{}
	b.Add(Evidence{ID: "e1", Kind: EvBuild, Summary: ""})
	if len(b.All()) != 0 {
		t.Fatal("empty summary evidence must be rejected")
	}
}

func TestEvidenceBagDedupeByID(t *testing.T) {
	b := &EvidenceBag{}
	b.Add(ev("e1", EvBuild, "./pkg", true))
	b.Add(ev("e1", EvTest, "./other", true))
	if len(b.All()) != 1 || b.All()[0].Kind != EvBuild {
		t.Fatalf("ID dedupe failed: %+v", b.All())
	}
}

func TestEvidenceBagSupersedeSameKindTarget(t *testing.T) {
	b := &EvidenceBag{}
	b.Add(ev("e1", EvBuild, "./pkg", true))
	b.Add(ev("e2", EvBuild, "./pkg", true))
	all := b.All()
	if len(all) != 2 || !all[0].Superseded || all[1].Superseded {
		t.Fatalf("supersede failed: %+v", all)
	}
	if got := b.For("./pkg"); len(got) != 1 || got[0].ID != "e2" {
		t.Fatalf("For() = %+v", got)
	}
	if b.Count() != 1 {
		t.Fatalf("Count = %d", b.Count())
	}
}

func TestEvidenceBagCrossTarget(t *testing.T) {
	b := &EvidenceBag{}
	b.Add(ev("e1", EvBuild, "./a", true))
	b.Add(ev("e2", EvTest, "./b", true))
	if len(b.For("./a")) != 1 || len(b.For("./b")) != 1 || len(b.For("./c")) != 0 {
		t.Fatal("cross target lookup failed")
	}
	if !b.Satisfies(Requirement{Kinds: []EvidenceKind{EvBuild, EvTest}, MinCount: 2}) {
		t.Fatal("both kinds should satisfy")
	}
	if b.Satisfies(Requirement{Kinds: []EvidenceKind{EvBuild, EvTest, EvVet}}) {
		t.Fatal("missing vet should not satisfy")
	}
}

func TestEvidenceBagSatisfiesFlags(t *testing.T) {
	b := &EvidenceBag{}
	b.Add(ev("e1", EvBuild, "./a", true))
	if b.Satisfies(Requirement{Kinds: []EvidenceKind{EvBuild}, MinCount: 2}) {
		t.Fatal("MinCount not enforced")
	}
	if b.Satisfies(Requirement{Kinds: []EvidenceKind{EvBuild}, NoEvidence: false, MinCount: 1}) == false {
		t.Fatal("basic satisfy failed")
	}
	empty := &EvidenceBag{}
	if !empty.Satisfies(Requirement{NoEvidence: true}) {
		t.Fatal("NoEvidence requirement should always satisfy")
	}
	failed := &EvidenceBag{}
	failed.Add(ev("e2", EvBuild, "./a", false))
	if failed.Satisfies(Requirement{Kinds: []EvidenceKind{EvBuild}, MinCount: 1}) {
		t.Fatal("failed evidence must not satisfy")
	}
}
