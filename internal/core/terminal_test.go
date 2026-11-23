package core

import "testing"

func TestTerminalExitCode(t *testing.T) {
	cases := []struct {
		t    Terminal
		code int
		ok   bool
	}{
		{TermDone, 0, true}, {TermHalt, 1, false}, {TermFailed, 1, false},
		{TermUntest, 3, false}, {TermDenied, 4, false},
	}
	for _, tc := range cases {
		t.Run(tc.t.String(), func(t *testing.T) {
			if got := tc.t.ExitCode(); got != tc.code {
				t.Fatalf("ExitCode() = %d, want %d", got, tc.code)
			}
			if got := tc.t.IsSuccess(); got != tc.ok {
				t.Fatalf("IsSuccess() = %v, want %v", got, tc.ok)
			}
		})
	}
}
