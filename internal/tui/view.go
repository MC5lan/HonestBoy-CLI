package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/MC5lan/HonestBoy-CLI/internal/core"
	"github.com/MC5lan/HonestBoy-CLI/internal/tui/component"
	"github.com/MC5lan/HonestBoy-CLI/internal/tui/render"
	"github.com/charmbracelet/lipgloss"
)

// banner 是顶部常驻的足球场面板：小人颠球，成功则射门 + GOAL!。
// 框线用亮色使其在终端里显眼（对标你说的"显眼颜色"）。
func (m *Model) banner() string {
	model := m.cfg.Model
	if model == "" {
		model = "default"
	}
	// 顶部框与下方对话框同宽（铺满终端宽度）。
	boxW := m.width
	if boxW < 46 {
		boxW = 46
	}
	inner := boxW - 2

	phase := pitchIdle
	if m.term == core.TermDone {
		phase = pitchGoal
	} else if m.kickFrame > 0 {
		phase = pitchKick
	} else if m.state == stStreaming || m.state == stToolRunning {
		phase = pitchWorking
	}

	const brightCyan = "\x1b[96m"
	const brightGreen = "\x1b[92;1m"
	const reset = "\x1b[0m"

	line := func(text string) string {
		// 左框(1) + 内容(inner) + 右框(1) = boxW
		pad := inner - render.DisplayWidth(text)
		if pad < 0 {
			pad = 0
		}
		return brightCyan + "│" + reset + text + strings.Repeat(" ", pad) + brightCyan + "│" + reset
	}
	// 标题只显示项目目录名，避免完整路径被截断；完整路径见 /path 与状态栏。
	title := fmt.Sprintf(" ⚽ Honest Boy FC · %s · %s", render.Truncate(model, 18), dirName(m.cfg.Workdir))

	var lines []string
	lines = append(lines, brightCyan+"╭"+strings.Repeat("─", inner)+"╮"+reset)
	lines = append(lines, line(render.Truncate(title, inner)))

	// 球场铺满内框：小人靠左，球门靠右。
	for _, pl := range strings.Split(renderPitch(phase, m.pitchFrame(), inner-2), "\n") {
		lines = append(lines, line(" "+pl))
	}
	if phase == pitchGoal {
		for _, gl := range goalArt() {
			lines = append(lines, line(" "+brightGreen+gl+reset))
		}
	}

	lines = append(lines, brightCyan+"╰"+strings.Repeat("─", inner)+"╯"+reset)
	return strings.Join(lines, "\n")
}

// dirName 返回路径的最后一段（项目名）。
func dirName(p string) string {
	p = strings.TrimRight(p, `\/`)
	if i := strings.LastIndexAny(p, `\/`); i >= 0 && i+1 < len(p) {
		return p[i+1:]
	}
	return p
}

func shortCwd(p string) string {
	r := []rune(p)
	if len(r) > 20 {
		return "~/" + string(r[len(r)-18:])
	}
	return p
}

func (m *Model) View() string {
	if !m.ready {
		return "正在初始化…"
	}
	l := ComputeLayout(m.width, m.height)

	// 顶部品牌框：永远常驻（吉祥物不会因为对话而消失）。
	var parts []string
	parts = append(parts, m.banner())

	// GOAL 艺术字会多占 4 行；命令面板也会占几行。对话区相应让位。
	goalExtra := 0
	if m.term == core.TermDone {
		goalExtra = len(goalArt())
	}
	panelExtra := 0
	if sug := m.slashSuggestions(); len(sug) > 0 {
		n := len(sug)
		if n > 6 {
			n = 6
		}
		panelExtra = n + 1 // + 底部操作提示
	}
	reserve := goalExtra + panelExtra

	// 对话区 + 右侧可观测面板（宽屏时分栏，窄屏隐藏面板）。
	body := m.view.View()
	if len(m.blocks) == 0 {
		body = "  " + m.th.DimStyle().Render("输入任务开始，过程会显示在这个框里。")
	}
	body = clipLines(body, maxInt(3, m.view.Height-reserve))
	// banner 与「对话区+面板」必须同宽。lipgloss 的 Width 不含 padding/border（各 2），
	// 所以 dialogW + panelW + 8 == m.width。
	panelW := 26
	dialogW := m.width - 2
	if !narrowMode(m.width) && m.width >= 76 {
		dialogW = m.width - panelW - 8
	}
	dialogStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("240")).
		Padding(0, 1).
		Width(dialogW)
	dialog := dialogStyle.Render(body)

	if !narrowMode(m.width) && m.width >= 76 {
		// 面板内容必须裁剪到可视高度，否则会撑破布局导致顶部被裁。
		panelBody := clipLines(m.renderPanel(panelW), maxInt(3, m.view.Height-reserve))
		panelStyle := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("39")).
			Padding(0, 1).
			Width(panelW)
		parts = append(parts, lipgloss.JoinHorizontal(lipgloss.Top, dialog, panelStyle.Render(panelBody)))
	} else {
		parts = append(parts, dialog)
	}

	// 提示/审批区。
	if m.pending != nil {
		card := &component.Approve{Command: m.pending.Req.Command, Risk: m.pending.Req.Risk}
		parts = append(parts, card.Render(m.width-2))
	} else if m.hint != "" || m.err != nil {
		parts = append(parts, m.hintLine())
	}

	// 忙碌行：spinner + 计时 + 中断提示（Codex: "• Working (1s • esc to interrupt)"）。
	if m.state == stStreaming || m.state == stToolRunning {
		secs := 0
		if !m.startedAt.IsZero() {
			secs = int(time.Since(m.startedAt).Seconds())
		}
		working := fmt.Sprintf("%s Working (%ds · esc 中断)", m.spinnerFrame(), secs)
		parts = append(parts, m.th.AccentStyle().Render("  "+working))
	}

	// 命令面板：输入 / 时显示，上下键选择，Tab 补全。
	if sug := m.slashSuggestions(); len(sug) > 0 {
		parts = append(parts, m.renderCommandPanel(sug))
	}

	// 输入区：整块深色背景（Codex 用 rgb(57,57,57)），› 加粗提示符。
	prompt := m.input.View()
	if narrowMode(m.width) {
		parts = append(parts, prompt)
	} else {
		parts = append(parts, m.th.ComposerStyle().Width(m.width-2).Render(prompt))
	}

	// 底部状态栏：常驻单行、暗色（对标 Codex 的 <model> · <dir> · Context ...）。
	if l.StatusHeight > 0 {
		parts = append(parts, m.statusBar())
		// 快捷键提示行（Codex: "← for agents · ? for shortcuts"）。
		parts = append(parts, m.th.DimStyle().Render("  ? 快捷键 · / 命令 · ctrl+c 中断"))
	}
	out := lipgloss.JoinVertical(lipgloss.Left, parts...)
	// 最后一道保险：输出行数绝不超过终端高度，否则顶部会被滚出屏幕。
	return clipLines(out, m.height)
}

// clipLines 把字符串裁剪到最多 n 行（保留前面的行）。
func clipLines(s string, n int) string {
	if n <= 0 {
		return ""
	}
	lines := strings.Split(s, "\n")
	if len(lines) <= n {
		return s
	}
	return strings.Join(lines[:n], "\n")
}

// renderCommandPanel 渲染斜杠命令面板。
func (m *Model) renderCommandPanel(sug []slashCmd) string {
	max := 6
	if len(sug) < max {
		max = len(sug)
	}
	var lines []string
	for i := 0; i < max; i++ {
		c := sug[i]
		text := render.PadDisplay(c.name, 12) + " " + c.desc
		if i == m.cmdIdx {
			lines = append(lines, m.th.AccentStyle().Render(" ▸ "+text))
		} else {
			lines = append(lines, m.th.DimStyle().Render("   "+text))
		}
	}
	if len(sug) > max {
		lines = append(lines, m.th.DimStyle().Render(fmt.Sprintf("   … %d 条", len(sug))))
	}
	lines = append(lines, m.th.DimStyle().Render("   ↑↓ 选择 · Tab 补全 · Enter 执行"))
	return strings.Join(lines, "\n")
}

// statusBar 是底部单行状态栏。
func (m *Model) statusBar() string {
	model := m.cfg.Model
	if model == "" {
		model = "default"
	}
	left := fmt.Sprintf("  %s · %s", model, shortCwd(m.cfg.Workdir))
	if !narrowMode(m.width) {
		pct := m.contextPercent()
		left += fmt.Sprintf(" · 上下文剩余 %d%%", pct)
		if m.term != "" {
			left += fmt.Sprintf(" · %s", core.TerminalLabel(m.term.String()))
		}
	}
	right := m.stateLabel()
	left = render.Truncate(left, maxInt(1, m.width-lipgloss.Width(right)-1))
	gap := m.width - lipgloss.Width(left) - lipgloss.Width(right)
	if gap < 1 {
		gap = 1
	}
	return m.th.DimStyle().Render(left+strings.Repeat(" ", gap)) + m.stateLabelStyle().Render(right)
}

func (m *Model) contextPercent() int {
	used := m.tokensIn + m.tokensOut
	window := m.cfg.ContextWindow
	if window <= 0 {
		window = 128000
	}
	pct := 100 - used*100/window
	if pct < 0 {
		return 0
	}
	return pct
}

func (m *Model) evidenceCount() int {
	n := 0
	for _, b := range m.blocks {
		if b.kind == blkEvidence || b.kind == blkGateAllow {
			n++
		}
	}
	return n
}

func (m *Model) stateLabel() string {
	switch m.state {
	case stStreaming:
		if m.startedAt.IsZero() {
			return "• 思考中"
		}
		secs := int(time.Since(m.startedAt).Seconds())
		if secs > 30 {
			return fmt.Sprintf("• 思考中 %ds（esc 中断）", secs)
		}
		return fmt.Sprintf("• 思考中 %ds", secs)
	case stToolRunning:
		return "• 执行中"
	case stApproving:
		return "• 等待批准"
	default:
		if m.term != "" {
			return "■ " + core.TerminalLabel(m.term.String())
		}
		return "就绪"
	}
}

func (m *Model) stateLabelStyle() lipgloss.Style {
	if m.state == stToolRunning || m.state == stStreaming {
		return m.th.AccentStyle()
	}
	return m.th.DimStyle()
}

func (m *Model) hintLine() string {
	if m.err != nil {
		return m.th.DangerStyle().Render("✗ 错误: " + m.err.Error())
	}
	return m.th.WarnStyle().Render(m.hint)
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
