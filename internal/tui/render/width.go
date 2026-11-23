package render

import (
	"strings"
	"unicode"

	"github.com/rivo/uniseg"
)

// Width 返回终端显示宽度。用 uniseg 处理 ZWJ/组合字符，避免逐码点累加。
func Width(s string) int {
	return uniseg.StringWidth(s)
}

// Truncate 按显示宽度截断，且不切断组合序列。
func Truncate(s string, w int) string {
	if w <= 0 {
		return ""
	}
	if Width(s) <= w {
		return s
	}
	gr := uniseg.NewGraphemes(s)
	width := 0
	end := 0
	for gr.Next() {
		cluster := gr.Str()
		cw := uniseg.StringWidth(cluster)
		if width+cw > w {
			break
		}
		// rune 数（不是字节数）才是安全的切片位置。
		end += len([]rune(cluster))
		width += cw
	}
	runes := []rune(s)
	if end > len(runes) {
		end = len(runes)
	}
	return string(runes[:end])
}

// Pad 右侧补齐到指定显示宽度。
func Pad(s string, w int) string {
	if n := w - Width(s); n > 0 {
		return s + strings.Repeat(" ", n)
	}
	return s
}

// VisibleRunes 过滤控制字符，用于快照测试的可读输出。
func VisibleRunes(s string) string {
	var b strings.Builder
	for _, r := range s {
		if unicode.IsControl(r) && r != '\n' && r != '\t' {
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}
