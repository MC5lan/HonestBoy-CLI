package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/MC5lan/HonestBoy-CLI/internal/core"
	"github.com/MC5lan/HonestBoy-CLI/internal/tui/render"
)

// 右侧可观测面板：把"系统现在发生了什么"变成显性信息。
// 内容全部来自事件流，不猜、不改写。

type obsStats struct {
	toolsOK, toolsFail  int
	gateAllow, gateDeny int
	receipts            int
	lastRule            string
	lastDenyReason      string
	startedAt           time.Time
}

func (m *Model) stats() obsStats {
	return obsStats{
		toolsOK: m.obsToolsOK, toolsFail: m.obsToolsFail,
		gateAllow: m.obsAllow, gateDeny: m.obsDeny,
		lastRule: m.obsRule, lastDenyReason: m.obsReason,
		startedAt: m.startedAt,
	}
}

// renderPanel 渲染右侧面板，宽度固定，内容超出时截断。
func (m *Model) renderPanel(width int) string {
	if width < 12 {
		return ""
	}
	s := m.stats()
	inner := width - 4
	line := func(label, value string) string {
		// label 带 ANSI，不能和 value 拼在一起再截断，否则转义序列会被算进宽度。
		avail := inner - render.Width(label) - 1
		if avail < 1 {
			avail = 1
		}
		return m.th.DimStyle().Render(label) + " " + render.Truncate(value, avail)
	}
	head := func(text string) string {
		return m.th.AccentStyle().Render(text)
	}
	var lines []string
	lines = append(lines, head("── 可观测 ──"))
	lines = append(lines, line("状态", m.stateLabelShort()))
	lines = append(lines, line("模型", m.cfg.Model))
	lines = append(lines, line("上下文", fmt.Sprintf("剩余 %d%%", m.contextPercent())))
	lines = append(lines, "")
	lines = append(lines, head("── 证据 ──"))
	lines = append(lines, line("通过", fmt.Sprintf("%d", s.gateAllow)))
	lines = append(lines, line("拒绝", fmt.Sprintf("%d", s.gateDeny)))
	lines = append(lines, line("工具✓", fmt.Sprintf("%d", s.toolsOK)))
	lines = append(lines, line("工具✗", fmt.Sprintf("%d", s.toolsFail)))
	lines = append(lines, "")
	lines = append(lines, head("── Gate ──"))
	if s.gateDeny == 0 && s.gateAllow == 0 {
		lines = append(lines, m.th.DimStyle().Render("  尚未判定"))
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
	} else if !m.startedAt.IsZero() {
		lines = append(lines, line("已用时", fmt.Sprintf("%ds", int(time.Since(m.startedAt).Seconds()))))
	} else {
		lines = append(lines, m.th.DimStyle().Render("  空闲"))
	}
	lines = append(lines, line("事件", fmt.Sprintf("%d", len(m.blocks))))

	// 关键：每条显性标注来源（P1 要求 UI 可区分）。
	lines = append(lines, "")
	lines = append(lines, m.th.DimStyle().Render("来源: 事件流"))

	// 定高：不足则补空行，超出截断。
	body := strings.Join(lines, "\n")
	return body
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
