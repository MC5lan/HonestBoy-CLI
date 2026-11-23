package render

import (
	"encoding/json"
	"fmt"
	"strings"
)

func jsonUnmarshal(data []byte, v any) error { return json.Unmarshal(data, v) }

// ToolCard 生成折叠/展开态的工具卡片。折叠态严格一行，失败默认展开。
// 只接收值类型，避免 tui/render 依赖 tools 包（依赖方向约束）。
func ToolCard(name, args, output string, ok bool, durationMS int64, width int, collapsed bool) string {
	// Codex 风格：成功只显示 • （颜色由调用方区分），失败显示 ✗。
	mark := ""
	if !ok {
		mark = "✗ "
	}
	head := fmt.Sprintf("• %s%s (%s)", mark, name, formatDuration(durationMS))
	if width > 0 && Width(head) > width {
		head = Truncate(head, width)
	}
	if !ok {
		collapsed = false
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

// compactArgs 把 JSON 参数压成一行可读摘要，避免整段 JSON 塞进卡片。
func compactArgs(args string) string {
	s := strings.TrimSpace(strings.ReplaceAll(args, "\n", " "))
	if s == "" {
		return ""
	}
	var out map[string]any
	if err := jsonUnmarshal([]byte(s), &out); err == nil {
		// 优先显示 path / command / pattern 这些最有信息量的字段。
		for _, key := range []string{"path", "command", "pattern", "target"} {
			if v, ok := out[key]; ok {
				return key + "=" + fmt.Sprint(v)
			}
		}
	}
	if len([]rune(s)) > 40 {
		return string([]rune(s)[:40]) + "…"
	}
	return s
}
