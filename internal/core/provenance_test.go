package core

import "testing"

func TestProvenanceRoundTrip(t *testing.T) {
	cases := []struct {
		name string
		p    Provenance
		want string
	}{
		{"unknown", ProvUnknown, "unknown"},
		{"assumed", ProvAssumed, "assumed"},
		{"inferred", ProvInferred, "inferred"},
		{"observed", ProvObserved, "observed"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.p.String(); got != tc.want {
				t.Fatalf("String() = %q, want %q", got, tc.want)
			}
			got, err := ParseProvenance(tc.want)
			if err != nil || got != tc.p {
				t.Fatalf("ParseProvenance(%q) = %v, %v", tc.want, got, err)
			}
		})
	}
}

func TestCanGroundOnlyObserved(t *testing.T) {
	for _, p := range []Provenance{ProvUnknown, ProvAssumed, ProvInferred} {
		if p.CanGround() {
			t.Fatalf("%s must not ground", p)
		}
	}
	if !ProvObserved.CanGround() {
		t.Fatal("observed must ground")
	}
}

func TestParseProvenanceRejectsExtraValue(t *testing.T) {
	if _, err := ParseProvenance("trusted"); err == nil {
		t.Fatal("expected error for fifth value")
	}
}
