package diff

import (
	"strings"
	"testing"
)

func TestUnifiedBasic(t *testing.T) {
	out := Unified("a.go", "a\nb\nc\n", "a\nB\nc\n")
	for _, want := range []string{"--- a/a.go", "+++ b/a.go", "@@", "-b", "+B", " a", " c"} {
		if !strings.Contains(out, want) {
			t.Fatalf("diff missing %q:\n%s", want, out)
		}
	}
}

func TestUnifiedNoChange(t *testing.T) {
	if out := Unified("a.go", "same\r\n", "same\n"); out != "" {
		t.Fatalf("expected empty diff, got %q", out)
	}
}

func TestUnifiedAddAndDelete(t *testing.T) {
	add := Unified("new.go", "", "package x\n")
	if !strings.Contains(add, "+package x") {
		t.Fatalf("add diff = %q", add)
	}
	del := Unified("old.go", "package x\n", "")
	if !strings.Contains(del, "-package x") {
		t.Fatalf("delete diff = %q", del)
	}
}

func TestUnifiedLargeFallsBack(t *testing.T) {
	var a, b strings.Builder
	for i := 0; i < maxLCSLines+10; i++ {
		a.WriteString("same\n")
		b.WriteString("same\n")
	}
	a.WriteString("old\n")
	b.WriteString("new\n")
	out := Unified("big.go", a.String(), b.String())
	if !strings.Contains(out, "省略逐行比对") || !strings.Contains(out, "-old") || !strings.Contains(out, "+new") {
		t.Fatalf("large diff = %q", out)
	}
}
