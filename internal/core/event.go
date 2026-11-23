package core

import "time"

// EventType 是事件的稳定标识。
type EventType string

const (
	EvtTurnStart      EventType = "turn_start"
	EvtTextDelta      EventType = "text_delta"
	EvtToolRequest    EventType = "tool_request"
	EvtApprovalNeeded EventType = "approval_required"
	EvtToolStart      EventType = "tool_start"
	EvtToolEnd        EventType = "tool_end"
	EvtToolDenied     EventType = "tool_denied"
	EvtNotice         EventType = "notice"
	EvtGateAllow      EventType = "gate_allow"
	EvtGateDeny       EventType = "gate_deny"
	EvtAbstain        EventType = "abstain"
	EvtTurnEnd        EventType = "turn_end"
	EvtDiffReview     EventType = "diff_review"
	EvtUsage          EventType = "usage"
	EvtError          EventType = "error"
)

// Event 是系统唯一真相；一经产出不得修改。
type Event struct {
	Seq       uint64    `json:"seq"`
	SessionID string    `json:"session_id"`
	TurnID    string    `json:"turn_id,omitempty"`
	Type      EventType `json:"type"`
	Time      time.Time `json:"time"`
	Data      any       `json:"data"`
}

type TurnStartData struct {
	Input string
	Model string
}
type TextDeltaData struct {
	Text  string
	Block int
}
type ToolRequestData struct{ CallID, Tool, Args string }
type ApprovalData struct{ CallID, Tool, Args, Risk string }
type ToolStartData struct{ CallID string }
type ToolEndData struct {
	CallID     string
	OK         bool
	ExitCode   int
	DurationMS int64
	Output     string
	Truncated  bool
}
type ToolDeniedData struct{ CallID, Reason string }
type NoticeData struct{ Level, Text string }
type GateAllowData struct {
	ClaimID     string
	EvidenceIDs []string
}
type GateDenyData struct {
	ClaimID string
	Missing []string
	Rule    string
	Reason  string
}
type AbstainData struct {
	Reason  string
	Done    []string
	NotDone []string
}
type TurnEndData struct {
	Terminal            string
	DurationMS          int64
	TokensIn, TokensOut int
	ReceiptID           string
}

// DiffReviewData 是一次自动提交的 diff 审阅信息。
type DiffReviewData struct {
	Commit string
	Files  []string
	Stat   string
}

type UsageData struct{ TokensIn, TokensOut, CostMicros int64 }
type ErrorData struct {
	Code, Message string
	Retryable     bool
}
