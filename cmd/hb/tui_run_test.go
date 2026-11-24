package main

import (
	"path/filepath"
	"testing"
)

func TestHistoryRoundTripPreservesNewlines(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history.json")
	want := []string{"第一行\n第二行", "另一个任务"}
	saveHistory(path, want)
	got := loadHistory(path)
	if len(got) != len(want) {
		t.Fatalf("history = %#v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("history[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}
