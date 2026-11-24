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
	EvtSessionEnd     EventType = "session_end"
	EvtProfile        EventType = "profile"
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
	// Diff 是写操作的统一格式行级差异（只增字段，不影响旧会话回放）。
	Diff string
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

type UsageData struct {
	TokensIn, TokensOut, CostMicros int64
	// PromptTokens 是最近一步实际发送的提示词 tokens（本轮累计 In 会重复计算历史，
	// 不能用来表示上下文占用）。旧会话没有该字段时为 0。
	PromptTokens int64
	// 上下文明细是近似值，供 TUI 展示；不是计费依据。
	SystemTokens  int64
	HBMTokens     int64
	HistoryTokens int64
	ToolTokens    int64
}
type ErrorData struct {
	Code, Message string
	Retryable     bool
}

// SessionEndData 标记会话正常收尾；缺失它说明进程被强杀或崩溃。
type SessionEndData struct {
	Terminal string
}

// ProfileData 是一次回合的耗时汇总（毫秒），供 hb run --profile 使用。
type ProfileData struct {
	TotalMS      int64
	ContextMS    int64
	StreamMS     int64
	FirstTokenMS int64
	ToolMS       int64
	ToolCalls    int
}
