package render

import (
	"strings"
)

// Table 渲染中英混排表格：按显示宽度计算列宽，数字右对齐，文本左对齐。
func Table(headers []string, rows [][]string) string {
	if len(headers) == 0 {
		return ""
	}
	widths := make([]int, len(headers))
	for i, h := range headers {
		widths[i] = Width(h)
		if widths[i] < 6 {
			widths[i] = 6
		}
	}
	for _, row := range rows {
		for i, cell := range row {
			if i >= len(widths) {
				break
			}
			if w := Width(cell); w > widths[i] {
				widths[i] = w
			}
		}
	}
	var out strings.Builder
	writeRow := func(cells []string) {
		for i, w := range widths {
			cell := ""
			if i < len(cells) {
				cell = cells[i]
			}
			if isNumeric(cell) {
				out.WriteString(strings.Repeat(" ", w-Width(cell)))
				out.WriteString(cell)
			} else {
				out.WriteString(Pad(cell, w))
			}
			if i < len(widths)-1 {
				out.WriteString("  ")
			}
		}
		out.WriteString("\n")
	}
	writeRow(headers)
	out.WriteString(strings.Repeat("─", lineWidth(widths)) + "\n")
	for _, row := range rows {
		writeRow(row)
	}
	return strings.TrimRight(out.String(), "\n")
}

func lineWidth(widths []int) int {
	total := 0
	for _, w := range widths {
		total += w
	}
	return total + 2*(len(widths)-1)
}

func isNumeric(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}
