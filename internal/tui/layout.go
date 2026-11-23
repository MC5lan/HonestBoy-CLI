package tui

// Layout 是屏幕分区的高度预算。
type Layout struct {
	BannerHeight int // 顶部品牌框（含吉祥物）
	ViewHeight   int // 对话框内部可视高度（不含边框）
	InputHeight  int // 输入区
	StatusHeight int // 状态行 + 快捷键行
	BusyHeight   int // 忙碌行
}

// ComputeLayout 计算各区高度：顶部品牌框常驻，对话区弹性，底部固定。
func ComputeLayout(w, h int) Layout {
	// 顶部球场框：上框1 + 标题1 + 球场10 + 下框1 = 13
	banner := 13
	if narrowMode(w) {
		banner = 5 // 窄屏简化，不显示球场
	}
	status := 2 // 状态行 + 快捷键行
	input := 1
	busy := 1
	dialogBorder := 2 // 对话框上下边框

	fixed := banner + status + input + busy + dialogBorder
	view := h - fixed
	if view < 1 {
		view = 1
	}
	return Layout{BannerHeight: banner, ViewHeight: view, InputHeight: input, StatusHeight: status, BusyHeight: busy}
}

func narrowMode(w int) bool { return w < 60 }
