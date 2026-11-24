package component

import (
	"strings"
	"testing"

	"github.com/MC5lan/HonestBoy-CLI/internal/core"
)

func TestApproveCard(t *testing.T) {
	normal := &Approve{Command: "go test ./...", Risk: "exec"}
	if !normal.Accept("y") || !normal.Accept("a") {
		t.Fatal("normal command should accept y/a")
	}
	card := normal.Render(60, "")
	if !strings.Contains(card, "go test") || !strings.Contains(card, "[y]") {
		t.Fatalf("card = %q", card)
	}

	high := &Approve{Command: "rm -rf /", Risk: "exec", HighRisk: true}
	if high.Accept("y") || high.Accept("a") {
		t.Fatal("high risk must not accept y/a")
	}
	if !high.Accept("yes") {
		t.Fatal("high risk should accept explicit yes")
	}
	if !strings.Contains(high.Render(60, ""), "高危") {
		t.Fatal("high risk card missing warning")
	}
}

func TestWrapTextKeepsShortLine(t *testing.T) {
	if got := wrapText("go test ./...", 72, 5); len(got) != 1 || got[0] != "go test ./..." {
		t.Fatalf("short command should stay on one line: %q", got)
	}
	got := wrapText("a b c d e f g h i j k l m n o p", 12, 5)
	if len(got) < 2 {
		t.Fatalf("long command should wrap: %q", got)
	}
}

func TestHighRiskShared(t *testing.T) {
	for _, cmd := range []string{"rm -rf /", "git reset --hard", "del /f /s /q c:\\x", "format c:"} {
		if !core.IsHighRiskCommand(cmd) {
			t.Fatalf("%q should be high risk", cmd)
		}
	}
	if core.IsHighRiskCommand("go test ./...") {
		t.Fatal("go test must not be high risk")
	}
}
