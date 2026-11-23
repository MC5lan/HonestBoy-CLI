package cli

import (
	"errors"
	"testing"
)

func TestExitCodeOf(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want int
	}{
		{"nil", nil, 0},
		{"usage", ErrUsage, 2},
		{"environment", ErrEnvironment, 6},
		{"timeout", ErrTimeout, 7},
		{"interrupt", ErrInterrupted, 130},
		{"chain", ErrChainBroken, 5},
		{"unknown", errors.New("boom"), 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := ExitCodeOf(tc.err); got != tc.want {
				t.Fatalf("ExitCodeOf() = %d, want %d", got, tc.want)
			}
		})
	}
}
