package core

import (
	"strings"
	"testing"
)

func TestPlaceholderWarnings(t *testing.T) {
	pos := []string{
		"// TODO: implement this",
		"FIXME: broken",
		"panic(\"not implemented\")",
		"throw new Error(\"not implemented\")",
		"const x = PLACEHOLDER",
	}
	for _, s := range pos {
		if len(PlaceholderWarnings(s)) == 0 {
			t.Fatalf("%q should warn", s)
		}
	}
	for _, s := range []string{
		"完整实现，没有占位符",
		"func Add(a, b int) int { return a + b }",
		"// 这是正常注释",
	} {
		if got := PlaceholderWarnings(s); len(got) != 0 {
			t.Fatalf("%q unexpected warnings: %v", s, got)
		}
	}
}

func TestRigorPromptFragment(t *testing.T) {
	for _, want := range []string{"完整实现", "declare_untested", "测试", "最小 diff", "来源"} {
		if !strings.Contains(RigorPromptFragment, want) {
			t.Fatalf("rigor prompt missing %q", want)
		}
	}
	if RigorEnabled("normal") || !RigorEnabled("STRICT") {
		t.Fatal("RigorEnabled mapping wrong")
	}
}
