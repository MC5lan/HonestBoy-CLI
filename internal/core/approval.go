package core

import "context"

// ApprovalRequest 是一次审批请求。
type ApprovalRequest struct {
	CallID   string
	Tool     string
	Command  string
	Risk     string
	HighRisk bool
}

// ApprovalDecision 是审批结论；AllowAll 只在本回合生效，且不覆盖高危。
type ApprovalDecision struct {
	Allowed  bool
	Reason   string
	AllowAll bool
}

// ApprovalHandler 由宿主实现：非交互 CLI 按策略自动决定，TUI 等待用户。
type ApprovalHandler func(ctx context.Context, req ApprovalRequest) (ApprovalDecision, error)
