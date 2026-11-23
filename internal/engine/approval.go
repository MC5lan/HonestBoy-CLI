package engine

import (
	"regexp"
	"strings"

	"github.com/MC5lan/HonestBoy-CLI/internal/core"
	"github.com/MC5lan/HonestBoy-CLI/internal/tools"
)

// ApprovalPolicy 判断工具调用是否需要批准。
// 高危命令永不自动批准（--yes 也不行）。
type ApprovalPolicy struct {
	Write string // auto | ask
	Exec  string // auto | ask
	Net   string // auto | ask
}

// readOnlyCommands 是只读命令白名单（正则精确匹配命令首词）。
var readOnlyCommands = []*regexp.Regexp{
	regexp.MustCompile(`(?i)^\s*ls\b`),
	regexp.MustCompile(`(?i)^\s*dir\b`),
	regexp.MustCompile(`(?i)^\s*(cat|type)\b`),
	regexp.MustCompile(`(?i)^\s*(head|tail)\b`),
	regexp.MustCompile(`(?i)^\s*rg\b`),
	regexp.MustCompile(`(?i)^\s*git\s+(status|diff|log)\b`),
	regexp.MustCompile(`(?i)^\s*go\s+(build|vet|test)\b`),
	regexp.MustCompile(`(?i)^\s*node\s+--version\b`),
	regexp.MustCompile(`(?i)^\s*pwd\b`),
}

// IsReadOnlyCommand 判断 shell 命令是否命中只读白名单。
func IsReadOnlyCommand(command string) bool {
	// 含 shell 连接符的命令不再是单条只读命令，不能走白名单。
	for _, op := range []string{"&&", "||", ";", "|", "`", ">", "$("} {
		if strings.Contains(command, op) {
			return false
		}
	}
	for _, re := range readOnlyCommands {
		if re.MatchString(command) {
			return true
		}
	}
	return false
}

// IsHighRisk 委托给 core，供 CLI 与 TUI 共用同一判据。
func IsHighRisk(command string) bool { return core.IsHighRiskCommand(command) }

// 审批类型定义在 core，供 engine 与 tui 共用而不产生反向依赖。
type ApprovalRequest = core.ApprovalRequest
type ApprovalDecision = core.ApprovalDecision
type ApprovalHandler = core.ApprovalHandler

// NeedApproval 判断是否需要走审批通道。
func (p ApprovalPolicy) NeedApproval(t tools.Tool, command string) bool {
	if core.IsHighRiskCommand(command) {
		return true
	}
	// 验证器是只读的确定性检查，默认放行；否则 Gate 永远拿不到证据。
	if strings.HasPrefix(t.Name(), "verify_") {
		return false
	}
	if t.Name() == "shell" && IsReadOnlyCommand(command) {
		return false
	}
	switch t.Risk() {
	case tools.RiskExec:
		return !strings.EqualFold(p.Exec, "auto")
	case tools.RiskWrite:
		return !strings.EqualFold(p.Write, "auto")
	case tools.RiskNet:
		return !strings.EqualFold(p.Net, "auto")
	default:
		return false
	}
}
