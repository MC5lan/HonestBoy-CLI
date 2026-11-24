package tui

import (
	"strings"
	"testing"

	"github.com/MC5lan/HonestBoy-CLI/internal/tui/render"
)

func TestPitchPhases(t *testing.T) {
	const w = 50
	idle := render.StripANSI(renderPitch(pitchIdle, 0, w, true))
	for _, want := range []string{"▀", "⚽", "┌───┐", "└───┘"} {
		if !strings.Contains(idle, want) {
			t.Fatalf("idle pitch missing %q:\n%s", want, idle)
		}
	}
	kick0 := render.StripANSI(renderPitch(pitchKick, 0, w, true))
	kick3 := render.StripANSI(renderPitch(pitchKick, 3, w, true))
	if kick0 == kick3 {
		t.Fatal("kick animation should move the ball")
	}
	goal := render.StripANSI(renderPitch(pitchGoal, 0, w, true))
	if !strings.Contains(goal, "⚽") {
		t.Fatal("goal pitch missing ball")
	}
}

func TestPitchFitsWidth(t *testing.T) {
	for _, w := range []int{40, 60, 100} {
		for _, line := range strings.Split(renderPitch(pitchIdle, 0, w, true), "\n") {
			if n := render.DisplayWidth(line); n != w {
				t.Fatalf("pitch width=%d line width %d: %q", w, n, render.StripANSI(line))
			}
		}
	}
}

func TestPitchFrameProgression(t *testing.T) {
	m := NewModel(Config{})
	m.setSize(80, 34)
	if m.pitchFrame() != 0 {
		t.Fatal("initial pitch frame should be 0")
	}
	m.startKick()
	if m.kickFrame != 1 {
		t.Fatalf("kickFrame = %d", m.kickFrame)
	}
	if m.pitchFrame() != 1 {
		t.Fatal("pitchFrame should follow kickFrame")
	}
}
