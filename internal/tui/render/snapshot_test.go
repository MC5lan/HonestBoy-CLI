package render

import "testing"

func TestWidthASCII(t *testing.T) {
	if got := Width("hello"); got != 5 {
		t.Fatalf("Width = %d, want 5", got)
	}
	if got := Truncate("hello", 3); got != "hel" {
		t.Fatalf("Truncate = %q", got)
	}
}
