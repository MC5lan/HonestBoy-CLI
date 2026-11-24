package tui

import "strings"

// Layout 是屏幕分区的高度预算。
type Layout struct {
	BannerHeight   int // 顶部简洁标题栏（固定 3 行）
	ViewHeight     int // 对话框内部可视高度（不含边框）
	InputHeight    int // 输入区
	StatusHeight   int // 状态行 + 快捷键行
	BusyHeight     int // 忙碌行
	ApprovalHeight int
	PaletteHeight  int
	HintHeight     int
	ShowPanel      bool
	ShowShortcuts  bool
}

// ComputeLayout 是给测试与外部估算用的纯函数；实际渲染使用 Model.layout，
// 后者会把审批卡、命令面板、提示行等真实高度算进去。
func ComputeLayout(w, h int) Layout {
	const banner = 3 // 固定标题栏：上边框 + 标题 + 下边框
	status := 2
	if h < 20 {
		status = 1
	}
	fixed := banner + status + 1 + 1 + 2
	view := h - fixed
	if view < 1 {
		view = 1
	}
	return Layout{
		BannerHeight: banner, ViewHeight: view, InputHeight: 1,
		StatusHeight: status, BusyHeight: 1,
		ShowShortcuts: status == 2, ShowPanel: w >= 76 && h >= 24,
	}
}

const panelWidth = 26

// dialogWidth 返回对话框的 lipgloss Width：Width 含 padding、不含边框。
func (m *Model) dialogWidth(showPanel bool) int {
	if showPanel {
		return maxInt(10, m.width-panelWidth-4)
	}
	return maxInt(10, m.width-2)
}

func (m *Model) layout() Layout {
	const banner = 3
	status := 2
	if m.height < 20 {
		status = 1
	}
	input := m.composerHeight()
	busy := 0
	if m.state == stStreaming || m.state == stToolRunning {
		busy = 1
	}
	hint := 0
	if (m.hint != "" || m.err != nil) && !m.showHelp && !m.showSessions {
		hint = 1
	}
	approval := 0
	if m.pending != nil {
		approval = strings.Count(m.approvalCard().Render(m.width-2, m.th.Warn), "\n") + 1
	}
	palette := 0
	if m.slashPanelVisible() {
		palette = m.paletteHeight()
	}
	view := m.height - (banner + 2 + approval + hint + busy + palette + input + status)
	for view < 3 {
		switch {
		case palette > 0:
			palette = 0
		case hint > 0:
			hint = 0
		case busy > 0:
			busy = 0
		case status > 1:
			status = 1
		default:
			view = 1
		}
		if view == 1 {
			break
		}
		view = m.height - (banner + 2 + approval + hint + busy + palette + input + status)
	}
	if view < 1 {
		view = 1
	}
	showPanel := !m.showHelp && !m.showSessions && m.width >= 76 && m.height >= 24
	m.input.SetHeight(input)
	m.view.Height = view
	if m.follow {
		m.view.GotoBottom()
	}
	newWidth := maxInt(10, m.dialogWidth(showPanel)-2)
	if newWidth != m.view.Width {
		m.view.Width = newWidth
		m.renderer.Invalidate(newWidth)
		m.resetRenderCache()
		m.refreshView()
	}
	return Layout{
		BannerHeight: banner, ViewHeight: view, InputHeight: input,
		StatusHeight: status, BusyHeight: busy, ApprovalHeight: approval,
		PaletteHeight: palette, HintHeight: hint,
		ShowPanel:     showPanel,
		ShowShortcuts: status == 2,
	}
}

func (m *Model) composerHeight() int {
	lines := m.input.LineCount()
	if lines < 1 {
		lines = 1
	}
	if lines > 6 {
		lines = 6
	}
	return lines
}

func (m *Model) paletteHeight() int {
	n := len(m.slashSuggestions())
	if n > paletteWindow {
		n = paletteWindow
	}
	h := n + 1 // 底部操作提示
	if len(m.slashSuggestions()) > paletteWindow {
		h++ // "… N 条"
	}
	return h
}

func narrowMode(w int) bool { return w < 60 }
