package component

import (
	"strings"

	"github.com/MC5lan/HonestBoy-CLI/internal/core"
	"github.com/charmbracelet/lipgloss"
)

// Approve 是审批卡片。高危命令需输入 yes，普通命令 y 即可。
type Approve struct {
	Command  string
	Risk     string
	Decision string
}

func (a *Approve) Render(width int) string {
	if a == nil {
		return ""
	}
	title := "需要批准"
	if core.IsHighRiskCommand(a.Command) {
		title = "高危操作 · 需输入 yes 确认"
	}
	border := lipgloss.RoundedBorder()
	style := lipgloss.NewStyle().Border(border).BorderForeground(lipgloss.Color("214")).Padding(0, 1)
	if width > 0 {
		style = style.Width(width - 2)
	}
	body := strings.Join([]string{
		title,
		"",
		"> " + a.Command,
		"",
		"[y] 允许  [n] 拒绝  [a] 本回合都允许",
	}, "\n")
	return style.Render(body)
}

// Accept 处理审批按键；高危命令只接受 "yes"。
func (a *Approve) Accept(key string) bool {
	if a == nil {
		return false
	}
	if core.IsHighRiskCommand(a.Command) {
		return strings.TrimSpace(key) == "yes"
	}
	return key == "y" || key == "a" || strings.TrimSpace(key) == "yes"
}
