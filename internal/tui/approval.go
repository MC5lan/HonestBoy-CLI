package tui

import (
	"strings"

	"github.com/MC5lan/HonestBoy-CLI/internal/core"
	"github.com/MC5lan/HonestBoy-CLI/internal/tui/component"
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

func (m *Model) approvalCard() *component.Approve {
	if m.pending == nil {
		return nil
	}
	req := m.pending.Req
	return &component.Approve{
		Tool: req.Tool, Command: req.Command, Risk: req.Risk, HighRisk: req.HighRisk,
	}
}

// handleApprovalKey 处理审批状态下的一次按键；返回 handled=false 时交给输入框。
func (m *Model) handleApprovalKey(msg tea.KeyMsg) (tea.Cmd, bool) {
	if m.pending == nil {
		return nil, false
	}
	req := m.pending.Req
	key := msg.String()
	if key == "ctrl+c" {
		return m.cancel(), true
	}
	if req.HighRisk {
		switch key {
		case "esc":
			m.denyPending("用户拒绝高危命令")
			return nil, true
		case "enter":
			if strings.EqualFold(strings.TrimSpace(m.input.Value()), "yes") {
				m.input.SetValue("")
				return m.respondApproval(true, false), true
			}
			m.denyPending("高危命令需输入 yes 确认")
			return nil, true
		}
		// 高危审批下 y/a/n 都是普通字符：输入 yes 才会执行。
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		return cmd, true
	}
	switch key {
	case "y":
		return m.respondApproval(true, false), true
	case "a":
		return m.respondApproval(true, true), true
	case "n", "esc":
		return m.respondApproval(false, false), true
	}
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return cmd, true
}

func (m *Model) denyPending(reason string) {
	if m.pending == nil {
		return
	}
	select {
	case m.pending.Resp <- core.ApprovalDecision{Allowed: false, Reason: reason}:
	default:
	}
	m.pending = nil
	m.input.SetValue("")
	if m.run != nil {
		m.state = stStreaming
	} else {
		m.state = stIdle
	}
	m.hint = reason
	m.refreshView()
}

func (m *Model) respondApproval(allowed, all bool) tea.Cmd {
	if m.pending == nil {
		return nil
	}
	d := core.ApprovalDecision{Allowed: allowed, AllowAll: all}
	if !allowed {
		d.Reason = "用户拒绝执行"
	}
	select {
	case m.pending.Resp <- d:
	default:
	}
	m.pending = nil
	m.input.SetValue("")
	if m.run != nil {
		m.state = stStreaming
	} else {
		m.state = stIdle
	}
	if allowed {
		m.hint = "已批准"
	} else {
		m.hint = "已拒绝"
	}
	m.refreshView()
	return nil
}
