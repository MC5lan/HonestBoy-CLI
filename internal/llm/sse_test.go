package llm

import (
	"errors"
	"strings"
	"testing"
)

type sseEvent struct{ event, data string }

func collectSSE(t *testing.T, input string) ([]sseEvent, error) {
	t.Helper()
	var got []sseEvent
	err := ParseSSE(strings.NewReader(input), func(event, data []byte) error {
		got = append(got, sseEvent{string(event), string(data)})
		return nil
	})
	return got, err
}

func TestParseSSEWellFormed(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  []sseEvent
	}{
		{"basic", "data: {\"a\":1}\n\n", []sseEvent{{"", `{"a":1}`}}},
		{"no-final-blank", "data: x", []sseEvent{{"", "x"}}},
		{"comments-only", ": ping\n\n", nil},
		{"zero-space", "data:x\n\n", []sseEvent{{"", "x"}}},
		{"one-space", "data: x\n\n", []sseEvent{{"", "x"}}},
		{"two-spaces", "data:  x\n\n", []sseEvent{{"", " x"}}},
		{"crlf", "data: x\r\n\r\n", []sseEvent{{"", "x"}}},
		{"multiline-data", "data: a\ndata: b\n\n", []sseEvent{{"", "a\nb"}}},
		{"event-field", "event: msg\ndata: x\n\n", []sseEvent{{"msg", "x"}}},
		{"ignore-other-fields", "id: 1\nretry: 100\ndata: y\n\n", []sseEvent{{"", "y"}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := collectSSE(t, tc.input)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(got) != len(tc.want) {
				t.Fatalf("got %+v, want %+v", got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Fatalf("event %d = %+v, want %+v", i, got[i], tc.want[i])
				}
			}
		})
	}
}

func TestParseSSELongLine(t *testing.T) {
	input := "data: " + strings.Repeat("x", maxSSELine+1) + "\n\n"
	_, err := collectSSE(t, input)
	if !errors.Is(err, ErrSSELineTooLong) {
		t.Fatalf("long line err = %v", err)
	}
}

func TestParseSSECallbackError(t *testing.T) {
	want := errors.New("boom")
	err := ParseSSE(strings.NewReader("data: x\n\n"), func(_, _ []byte) error { return want })
	if !errors.Is(err, ErrSSECallback) || !errors.Is(err, want) {
		t.Fatalf("callback error not wrapped: %v", err)
	}
}
