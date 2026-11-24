package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/MC5lan/HonestBoy-CLI/internal/core"
	"github.com/MC5lan/HonestBoy-CLI/internal/tui/render"
)

// 右侧可观测面板：把“系统现在发生了什么”变成显性信息。
// 内容全部来自事件流，不猜、不改写。

type obsStats struct {
	toolsOK, toolsFail  int
	gateAllow, gateDeny int
	lastRule            string
	lastDenyReason      string
	promptTokens        int
	sessToolsOK         int
	sessToolsFail       int
	sessGateAllow       int
	sessGateDeny        int
}

func (m *Model) stats() obsStats {
	return obsStats{
		toolsOK: m.turnToolsOK, toolsFail: m.turnToolsFail,
		gateAllow: m.turnGateAllow, gateDeny: m.turnGateDeny,
		lastRule: m.obsRule, lastDenyReason: m.obsReason,
		promptTokens: m.promptTokens,
		sessToolsOK:  m.sessToolsOK, sessToolsFail: m.sessToolsFail,
		sessGateAllow: m.sessGateAllow, sessGateDeny: m.sessGateDeny,
	}
}

func (m *Model) renderPanel(width int) string {
	if width < 12 {
		return ""
	}
	s := m.stats()
	inner := width - 2
	line := func(label, value string) string {
		avail := inner - render.Width(label) - 1
		if avail < 1 {
			avail = 1
		}
		return m.th.DimStyle().Render(label) + " " + render.Truncate(value, avail)
	}
	head := func(text string) string { return m.th.AccentStyle().Render(text) }
	var lines []string
	lines = append(lines, head("── 状态 ──"))
	lines = append(lines, line("状态", m.stateLabelShort()))
	lines = append(lines, line("模型", firstNonEmpty(m.cfg.Model, "default")))
	lines = append(lines, line("上下文", fmt.Sprintf("~%d%%（估算）", m.contextPercent())))
	lines = append(lines, "")
	lines = append(lines, head("── 本轮 ──"))
	lines = append(lines, line("工具✓", fmt.Sprint(s.toolsOK)), line("工具✗", fmt.Sprint(s.toolsFail)))
	lines = append(lines, line("Gate✓", fmt.Sprint(s.gateAllow)), line("Gate✗", fmt.Sprint(s.gateDeny)))
	lines = append(lines, line("token", fmt.Sprintf("%d/%d", m.tokensIn, m.tokensOut)))
	lines = append(lines, "")
	lines = append(lines, head("── 会话 ──"))
	lines = append(lines, line("工具✓", fmt.Sprint(s.sessToolsOK)), line("工具✗", fmt.Sprint(s.sessToolsFail)))
	lines = append(lines, line("Gate✓", fmt.Sprint(s.sessGateAllow)), line("Gate✗", fmt.Sprint(s.sessGateDeny)))
	lines = append(lines, "")
	lines = append(lines, head("── Gate ──"))
	if s.gateDeny == 0 && s.gateAllow == 0 {
		lines = append(lines, m.th.DimStyle().Render("  本轮尚未判定"))
	} else if s.gateDeny > 0 {
		lines = append(lines, line("最近规则", s.lastRule))
		lines = append(lines, m.th.WarnStyle().Render("  "+render.Truncate(s.lastDenyReason, inner-2)))
	} else {
		lines = append(lines, m.th.SuccessStyle().Render("  证据齐备"))
	}
	lines = append(lines, "")
	lines = append(lines, head("── 回合 ──"))
	if m.term != "" {
		lines = append(lines, line("终态", core.TerminalLabel(m.term.String())))
	} else if !m.startedAt.IsZero() && m.busy() {
		lines = append(lines, line("已用时", fmt.Sprintf("%ds", int(time.Since(m.startedAt).Seconds()))))
	} else {
		lines = append(lines, m.th.DimStyle().Render("  空闲"))
	}
	lines = append(lines, line("块", fmt.Sprint(len(m.blocks))))
	lines = append(lines, "")
	lines = append(lines, m.th.DimStyle().Render("来源: 事件流"))
	return strings.Join(lines, "\n")
}

func (m *Model) stateLabelShort() string {
	switch m.state {
	case stStreaming:
		return "思考中"
	case stToolRunning:
		return "执行中"
	case stApproving:
		return "待批准"
	default:
		if m.term != "" {
			return core.TerminalLabel(m.term.String())
		}
		return "就绪"
	}
}
