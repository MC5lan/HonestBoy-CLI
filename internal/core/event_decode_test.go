package core

import (
	"encoding/json"
	"testing"
)

func TestUnmarshalEventRoundTrip(t *testing.T) {
	cases := []struct {
		name string
		t    EventType
		data any
	}{
		{"turn_start", EvtTurnStart, TurnStartData{Input: "hi", Model: "m"}},
		{"text_delta", EvtTextDelta, TextDeltaData{Text: "a", Block: 1}},
		{"tool_request", EvtToolRequest, ToolRequestData{CallID: "c", Tool: "t", Args: "{}"}},
		{"approval", EvtApprovalNeeded, ApprovalData{CallID: "c", Risk: "high"}},
		{"tool_start", EvtToolStart, ToolStartData{CallID: "c"}},
		{"tool_end", EvtToolEnd, ToolEndData{CallID: "c", OK: true, ExitCode: 0, Diff: "--- a\n+++ b\n-old\n+new\n"}},
		{"tool_denied", EvtToolDenied, ToolDeniedData{CallID: "c", Reason: "no"}},
		{"notice", EvtNotice, NoticeData{Level: "warn", Text: "x"}},
		{"gate_allow", EvtGateAllow, GateAllowData{ClaimID: "cl", EvidenceIDs: []string{"e1"}}},
		{"gate_deny", EvtGateDeny, GateDenyData{ClaimID: "cl", Rule: "coverage", Missing: []string{"test"}}},
		{"abstain", EvtAbstain, AbstainData{Reason: "r", NotDone: []string{"x"}}},
		{"turn_end", EvtTurnEnd, TurnEndData{Terminal: "done", TokensIn: 1}},
		{"usage", EvtUsage, UsageData{TokensIn: 1, TokensOut: 2, PromptTokens: 3, SystemTokens: 4, HBMTokens: 5, HistoryTokens: 6, ToolTokens: 7}},
		{"error", EvtError, ErrorData{Code: "E", Message: "m"}},
		{"session_end", EvtSessionEnd, SessionEndData{Terminal: "done"}},
		{"profile", EvtProfile, ProfileData{TotalMS: 10, ContextMS: 2, StreamMS: 3, FirstTokenMS: 1, ToolMS: 4, ToolCalls: 2}},
	}
	s := &Session{ID: "s"}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			original := s.Emit(tc.t, tc.data)
			raw, err := json.Marshal(original)
			if err != nil {
				t.Fatal(err)
			}
			got, err := UnmarshalEvent(raw)
			if err != nil {
				t.Fatal(err)
			}
			if got.Seq != original.Seq || got.Type != tc.t || got.SessionID != "s" {
				t.Fatalf("meta mismatch: %+v", got)
			}
			origJSON, _ := json.Marshal(original.Data)
			gotJSON, _ := json.Marshal(got.Data)
			if string(origJSON) != string(gotJSON) {
				t.Fatalf("data = %s, want %s", gotJSON, origJSON)
			}
		})
	}
}

func TestUnmarshalEventErrors(t *testing.T) {
	if _, err := UnmarshalEvent([]byte("not-json")); err == nil {
		t.Fatal("bad json should fail")
	}
	if _, err := UnmarshalEvent([]byte(`{"type":"unknown","data":{}}`)); err == nil {
		t.Fatal("unknown type should fail")
	}
	ev, err := UnmarshalEvent([]byte(`{"type":"turn_start"}`))
	if err != nil || ev.Data != nil {
		t.Fatalf("null data = %+v %v", ev, err)
	}
	if _, err := UnmarshalEvent([]byte(`{"type":"turn_start","data":123}`)); err == nil {
		t.Fatal("wrong data shape should fail")
	}
}
