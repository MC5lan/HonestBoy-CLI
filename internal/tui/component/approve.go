package component

import (
	"strings"

	"github.com/MC5lan/HonestBoy-CLI/internal/core"
	"github.com/charmbracelet/lipgloss"
)

// Approve 是审批卡片。高危命令必须输入 yes 后回车，普通命令 y/a 即可。
// HighRisk 由请求方显式传入，卡片不得自行从 Command 重新推断：
// outside_read 的 Command 是路径，重新推断会把高危卡片画成普通卡片。
type Approve struct {
	Tool     string
	Command  string
	Risk     string
	HighRisk bool
}

func (a *Approve) Render(width int, border lipgloss.Color) string {
	if a == nil {
		return ""
	}
	compact := width > 0 && width < 64
	title := "需要批准"
	keys := "[y] 允许   [a] 本回合都允许   [n]/[Esc] 拒绝"
	if a.HighRisk {
		title = "高危操作 · 输入 yes 后回车才会执行"
		keys = "输入 yes 再回车才会执行 · 其他内容回车拒绝 · Esc 拒绝"
		if compact {
			title = "高危操作 · 输入 yes 后回车"
			keys = "输入 yes 回车执行 · Esc 拒绝"
		}
	} else if compact {
		keys = "[y] 允许  [a] 都允许  [n] 拒绝"
	}
	borderStyle := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).
		BorderForeground(border).Padding(0, 1)
	if width > 0 {
		borderStyle = borderStyle.Width(width - 2)
	}
	tool := core.ToolLabel(a.Tool)
	if a.Tool == "" {
		tool = "shell"
	}
	risk := a.Risk
	if risk == "" {
		risk = "exec"
	}
	contentWidth := width - 6
	if contentWidth < 12 {
		contentWidth = 12
	}
	cmdLines := wrapText(a.Command, contentWidth, 5)
	body := []string{title, "工具: " + tool + " · 风险: " + risk, ""}
	for i, line := range cmdLines {
		if i == 0 {
			body = append(body, "> "+line)
		} else {
			body = append(body, "  "+line)
		}
	}
	body = append(body, "", keys)
	return borderStyle.Render(strings.Join(body, "\n"))
}

// Accept 处理普通审批按键；高危命令只接受 yes。
func (a *Approve) Accept(key string) bool {
	if a == nil {
		return false
	}
	if a.HighRisk {
		return strings.EqualFold(strings.TrimSpace(key), "yes")
	}
	return key == "y" || key == "a" || strings.EqualFold(strings.TrimSpace(key), "yes")
}

// wrapText 把命令按显示宽度折行，最多 maxLines 行；不丢弃中间内容。
func wrapText(s string, width, maxLines int) []string {
	s = strings.Join(strings.Fields(s), " ")
	r := []rune(s)
	if width < 4 {
		width = 4
	}
	var lines []string
	for len(r) > 0 && len(lines) < maxLines {
		cut := width
		if cut > len(r) {
			cut = len(r)
		} else {
			// 只有超宽时才回退到空格断行。
			for i := cut - 1; i > 0; i-- {
				if r[i] == ' ' {
					cut = i
					break
				}
			}
		}
		lines = append(lines, string(r[:cut]))
		r = r[cut:]
		for len(r) > 0 && r[0] == ' ' {
			r = r[1:]
		}
	}
	if len(r) > 0 && len(lines) > 0 {
		lines[len(lines)-1] += "…"
	}
	if len(lines) == 0 {
		lines = []string{""}
	}
	return lines
}
