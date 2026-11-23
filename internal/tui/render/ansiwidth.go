package render

import "regexp"

var ansiEscape = regexp.MustCompile(`\x1b\[[0-9;]*[A-Za-z]`)

// StripANSI 去掉 ANSI 转义序列。
func StripANSI(s string) string {
	return ansiEscape.ReplaceAllString(s, "")
}

// DisplayWidth 返回不含 ANSI 转义的真实显示宽度。
// 含彩色字符的画框必须用这个，否则转义序列会被算进宽度导致错位。
func DisplayWidth(s string) int {
	return Width(StripANSI(s))
}

// PadDisplay 用空格把彩色字符串补齐到指定显示宽度。
func PadDisplay(s string, w int) string {
	if n := w - DisplayWidth(s); n > 0 {
		return s + spaces(n)
	}
	return s
}

func spaces(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = ' '
	}
	return string(b)
}
