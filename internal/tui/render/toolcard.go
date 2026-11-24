package render

import (
	"fmt"
	"strings"
)

// ToolCard 生成折叠/展开态的工具卡片。折叠态严格一行，失败默认展开。
// 只接收值类型，避免 tui/render 依赖 tools 包（依赖方向约束）。
func ToolCard(name, args, output string, ok bool, durationMS int64, width int, collapsed bool) string {
	// Codex 风格：成功只显示 • （颜色由调用方区分），失败显示 ✗。
	mark := ""
	if !ok {
		mark = "✗ "
	}
	head := fmt.Sprintf("• %s%s (%s)", mark, name, formatDuration(durationMS))
	if collapsed && strings.TrimSpace(output) != "" {
		head += " · ctrl+o 展开"
	}
	if !ok {
		collapsed = false
	}
	if width > 0 && Width(head) > width {
		head = Truncate(head, width)
	}
	if collapsed {
		return head
	}
	// 输出预览：取前 20 行、每行先去掉缩进以外的空白噪声。
	lines := strings.Split(strings.TrimRight(output, "\n"), "\n")
	if len(lines) > 20 {
		lines = append(lines[:20], fmt.Sprintf("… 折叠 %d 行", len(lines)-20))
	}
	var b strings.Builder
	b.WriteString(head)
	for _, line := range lines {
		b.WriteString("\n    ")
		b.WriteString(Truncate(strings.TrimRight(line, "\r"), 120))
	}
	return b.String()
}

func formatDuration(ms int64) string {
	if ms < 1000 {
		return fmt.Sprintf("%d毫秒", ms)
	}
	return fmt.Sprintf("%.1f秒", float64(ms)/1000)
}
