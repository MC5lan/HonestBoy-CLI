package tui

import (
	"github.com/MC5lan/HonestBoy-CLI/internal/core"
	tea "github.com/charmbracelet/bubbletea"
)

// ApprovalAsk 是引擎发来的审批请求；Resp 用于回填用户决定。
type ApprovalAsk struct {
	Req  core.ApprovalRequest
	Resp chan core.ApprovalDecision
}

type approvalClosedMsg struct{}

// waitForApproval 持续等待审批请求；channel 关闭后发 approvalClosedMsg。
func waitForApproval(ch <-chan ApprovalAsk) tea.Cmd {
	if ch == nil {
		return nil
	}
	return func() tea.Msg {
		ask, ok := <-ch
		if !ok {
			return approvalClosedMsg{}
		}
		return ask
	}
}
