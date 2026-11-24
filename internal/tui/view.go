package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/MC5lan/HonestBoy-CLI/internal/core"
	"github.com/MC5lan/HonestBoy-CLI/internal/tui/render"
	"github.com/charmbracelet/lipgloss"
)

// banner 是顶部固定三行的简洁标题栏：品牌、模型、工作区。
// 不再占用空间放吉祥物或球场动画，释放的行数全部给对话区。
func (m *Model) banner() string {
	boxW := m.width
	if boxW < 1 {
		boxW = 1
	}
	inner := boxW - 2
	if inner < 1 {
		inner = 1
	}
	border := m.th.AccentStyle()
	title := fmt.Sprintf(" Honest Boy · %s · %s",
		render.Truncate(firstNonEmpty(m.cfg.Model, "default"), 18), dirName(m.cfg.Workdir))
	line := func(text string) string {
		pad := inner - render.DisplayWidth(text)
		if pad < 0 {
			pad = 0
		}
		return border.Render("│") + text + strings.Repeat(" ", pad) + border.Render("│")
	}
	lines := []string{
		border.Render("╭" + strings.Repeat("─", inner) + "╮"),
		line(render.Truncate(title, inner)),
		border.Render("╰" + strings.Repeat("─", inner) + "╯"),
	}
	return strings.Join(lines, "\n")
}

func dirName(p string) string {
	p = strings.TrimRight(p, `\/`)
	if i := strings.LastIndexAny(p, `\/`); i >= 0 && i+1 < len(p) {
		return p[i+1:]
	}
	return p
}

func shortSessionID(id string) string {
	r := []rune(id)
	if len(r) <= 10 {
		return id
	}
	return string(r[:10])
}

func shortCwd(p string) string {
	r := []rune(p)
	if len(r) > 20 {
		return "~/" + string(r[len(r)-18:])
	}
	return p
}

func firstNonEmpty(v, fallback string) string {
	if strings.TrimSpace(v) == "" {
		return fallback
	}
	return v
}

func (m *Model) View() string {
	if !m.ready {
		return "正在初始化…"
	}
	l := m.layout()

	parts := []string{m.banner()}

	body := m.view.View()
	switch {
	case m.showHelp:
		body = m.helpPage(l.ViewHeight)
	case m.showSessions:
		body = m.sessionsPage()
	case len(m.blocks) == 0:
		body = "  " + m.th.DimStyle().Render("输入任务开始，过程会显示在这个框里。") + "\n" +
			"  " + m.th.DimStyle().Render("/ 命令面板 · ? 快捷键 · Enter 提交 · Alt+Enter 换行")
	}
	body = clipLines(body, l.ViewHeight)

	dialogW := m.dialogWidth(l.ShowPanel)
	dialog := m.th.DialogBox().Width(dialogW).Height(l.ViewHeight).Render(body)
	if l.ShowPanel {
		panelBody := clipLines(m.renderPanel(panelWidth), l.ViewHeight)
		panel := m.th.PanelBox().Width(panelWidth).Height(l.ViewHeight).Render(panelBody)
		parts = append(parts, lipgloss.JoinHorizontal(lipgloss.Top, dialog, panel))
	} else {
		parts = append(parts, dialog)
	}

	if m.pending != nil {
		parts = append(parts, m.approvalCard().Render(m.width-2, m.approvalBorder()))
	}
	if l.HintHeight > 0 {
		parts = append(parts, m.hintLine())
	}
	if l.BusyHeight > 0 {
		secs := 0
		if !m.startedAt.IsZero() {
			secs = int(time.Since(m.startedAt).Seconds())
		}
		working := fmt.Sprintf("%s Working (%ds · esc 中断)", m.spinnerFrame(), secs)
		parts = append(parts, m.th.AccentStyle().Render("  "+working))
	}
	if l.PaletteHeight > 0 {
		parts = append(parts, m.renderCommandPanel(m.slashSuggestions()))
	}

	prompt := m.input.View()
	if narrowMode(m.width) {
		parts = append(parts, prompt)
	} else {
		parts = append(parts, m.th.ComposerStyle().Width(m.width-2).Render(prompt))
	}

	if l.StatusHeight > 0 {
		parts = append(parts, m.statusBar())
		if l.ShowShortcuts {
			right := "? 快捷键 · / 命令 · ctrl+o 工具输出"
			if !m.follow {
				right = "↑ 已暂停跟随 · PgDn 回到底部"
				if m.newBelow {
					right = "↓ 有新内容 · PgDn 回到底部"
				}
			}
			parts = append(parts, m.th.DimStyle().Render("  "+render.Truncate(right, maxInt(1, m.width-4))))
		}
	}
	out := lipgloss.JoinVertical(lipgloss.Left, parts...)
	return clipLines(out, m.height)
}

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

// renderCommandPanel 渲染斜杠命令面板；选中项始终在窗口内。
func (m *Model) renderCommandPanel(sug []slashCmd) string {
	if len(sug) == 0 {
		return ""
	}
	start := m.paletteOffset
	if start < 0 {
		start = 0
	}
	if start > len(sug) {
		start = len(sug)
	}
	end := start + paletteWindow
	if end > len(sug) {
		end = len(sug)
	}
	var lines []string
	for i := start; i < end; i++ {
		c := sug[i]
		text := render.Truncate(render.PadDisplay(c.name, 12)+" "+c.desc, maxInt(20, m.width-6))
		if i == m.paletteIdx {
			lines = append(lines, m.th.AccentStyle().Render(" ▸ "+text))
		} else {
			lines = append(lines, m.th.DimStyle().Render("   "+text))
		}
	}
	if len(sug) > paletteWindow {
		lines = append(lines, m.th.DimStyle().Render(render.Truncate(fmt.Sprintf("   … 共 %d 条（↑↓ 翻页）", len(sug)), maxInt(20, m.width-4))))
	}
	lines = append(lines, m.th.DimStyle().Render(render.Truncate("   ↑↓ 选择 · Tab 补全 · Enter 执行/补全参数 · Esc 关闭", maxInt(20, m.width-4))))
	return strings.Join(lines, "\n")
}

func (m *Model) sessionsPage() string {
	var lines []string
	lines = append(lines, m.th.AccentStyle().Render("选择会话后回车恢复（下一次输入会续接该会话）"))
	start := 0
	if m.sessionIdx >= 8 {
		start = m.sessionIdx - 7
	}
	end := start + 8
	if end > len(m.sessionList) {
		end = len(m.sessionList)
	}
	for i := start; i < end; i++ {
		s := m.sessionList[i]
		term := s.Terminal
		if term == "" {
			term = "中断"
		}
		text := render.PadDisplay(s.ID, 24) + " " + render.PadDisplay(term, 8) + " " + s.Modified + " " + render.Truncate(s.Summary, 30)
		if s.ID == m.sessionID {
			text += " · 当前"
		}
		text = render.Truncate(text, maxInt(20, m.width-8))
		if i == m.sessionIdx {
			lines = append(lines, m.th.AccentStyle().Render(" ▸ "+text))
		} else {
			lines = append(lines, m.th.DimStyle().Render("   "+text))
		}
	}
	if len(m.sessionList) > 8 {
		lines = append(lines, m.th.DimStyle().Render(fmt.Sprintf("   … 共 %d 条", len(m.sessionList))))
	}
	lines = append(lines, "")
	lines = append(lines, m.th.DimStyle().Render(render.Truncate("↑↓ 选择 · Enter 恢复 · Esc 关闭", maxInt(20, m.width-8))))
	return strings.Join(lines, "\n")
}

func (m *Model) statusBar() string {
	model := firstNonEmpty(m.cfg.Model, "default")
	left := fmt.Sprintf("  %s · %s", model, shortCwd(m.cfg.Workdir))
	if m.width >= 76 {
		left += fmt.Sprintf(" · 上下文 ~%d%%", m.contextPercent())
		if m.term != "" {
			left += fmt.Sprintf(" · %s", core.TerminalLabel(m.term.String()))
		}
	}
	if m.width >= 92 {
		left += fmt.Sprintf(" · 证据 %d", m.turnEvidence)
		if m.sessionID != "" {
			left += " · 会话 " + shortSessionID(m.sessionID)
		}
	}
	if len(m.queue) > 0 {
		left += fmt.Sprintf(" · 队列 %d", len(m.queue))
	}
	right := m.stateLabel()
	left = render.Truncate(left, maxInt(1, m.width-lipgloss.Width(right)-1))
	gap := m.width - lipgloss.Width(left) - lipgloss.Width(right)
	if gap < 1 {
		gap = 1
	}
	return m.th.DimStyle().Render(left+strings.Repeat(" ", gap)) + m.stateLabelStyle().Render(right)
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
		return m.th.DangerStyle().Render(render.Truncate("✗ 错误: "+m.err.Error(), m.width-2))
	}
	return m.th.WarnStyle().Render(render.Truncate(m.hint, m.width-2))
}
