package core

import "testing"

func TestTurnAllTransitions(t *testing.T) {
	states := []TurnState{StIdle, StThinking, StToolPending, StToolRunning, StJudging, StClosed}
	legal := map[[2]TurnState]bool{
		{StIdle, StThinking}:           true,
		{StThinking, StToolPending}:    true,
		{StThinking, StJudging}:        true,
		{StThinking, StClosed}:         true,
		{StToolPending, StToolRunning}: true,
		{StToolPending, StThinking}:    true,
		{StToolRunning, StThinking}:    true,
		{StToolRunning, StClosed}:      true,
		{StJudging, StClosed}:          true,
	}
	for _, from := range states {
		for _, to := range states {
			t.Run(from.String()+"_to_"+to.String(), func(t *testing.T) {
				turn := &Turn{State: from}
				want := legal[[2]TurnState{from, to}]
				if got := turn.CanTransition(to); got != want {
					t.Fatalf("CanTransition(%s->%s) = %v, want %v", from, to, got, want)
				}
				err := turn.Transition(to)
				if want && err != nil {
					t.Fatalf("legal transition failed: %v", err)
				}
				if !want && err != ErrIllegalTransition {
					t.Fatalf("illegal transition err = %v", err)
				}
				if want && turn.State != to {
					t.Fatalf("state = %s, want %s", turn.State, to)
				}
			})
		}
	}
}

func TestTurnRejectsDirectDone(t *testing.T) {
	turn := &Turn{State: StThinking, Terminal: TermDone}
	if err := turn.Transition(StClosed); err != ErrIllegalTransition {
		t.Fatalf("Thinking -> Closed(done) must be rejected, got %v", err)
	}
	if turn.State != StThinking {
		t.Fatal("state changed on rejected transition")
	}
	// 非 done 的中断允许直接 Closed。
	turn = &Turn{State: StThinking, Terminal: TermFailed}
	if err := turn.Transition(StClosed); err != nil {
		t.Fatalf("Thinking -> Closed(failed) should be allowed: %v", err)
	}
}

func TestTurnStateString(t *testing.T) {
	cases := []struct {
		s    TurnState
		want string
	}{
		{StIdle, "idle"}, {StThinking, "thinking"}, {StToolPending, "tool_pending"},
		{StToolRunning, "tool_running"}, {StJudging, "judging"}, {StClosed, "closed"},
		{TurnState(99), "idle"},
	}
	for _, tc := range cases {
		if got := tc.s.String(); got != tc.want {
			t.Fatalf("%d.String() = %q, want %q", tc.s, got, tc.want)
		}
	}
}
