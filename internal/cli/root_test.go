package cli

import (
	"errors"
	"testing"
)

func TestExecuteGlobalFlags(t *testing.T) {
	cases := []struct {
		name string
		args []string
		want error
	}{
		{"unknown-protocol", []string{"--protocol", "bogus", "run", "x"}, ErrUsage},
		{"valid-anthropic-protocol", []string{"--protocol", "anthropic", "run", "x"}, ErrEnvironment},
		{"valid-responses-protocol", []string{"--protocol", "responses", "run", "x"}, ErrEnvironment},
		{"missing-flag-value", []string{"--model"}, ErrUsage},
		{"max-turns-invalid", []string{"--max-turns", "abc", "run", "x"}, ErrUsage},
		{"json-and-quiet", []string{"--json", "--quiet", "run", "x"}, ErrUsage},
		{"unknown-command", []string{"nope"}, ErrUsage},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := Execute(tc.args)
			if !errors.Is(err, tc.want) {
				t.Fatalf("Execute(%v) = %v, want %v", tc.args, err, tc.want)
			}
		})
	}
}
