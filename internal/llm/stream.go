package llm

import (
	"strings"
	"unicode/utf8"
)

// SanitizeUTF8 把非法字节序列替换为 U+FFFD。
// 模型流式输出在多字节字符边界切断时，单块 JSON 可能带不完整序列。
func SanitizeUTF8(s string) string {
	if utf8.ValidString(s) {
		return s
	}
	// []rune 转换会把非法字节替换为 U+FFFD。
	return string([]rune(s))
}

// AppendDelta 把增量文本追加到 Builder，写回前做一次 UTF-8 保障。
func AppendDelta(b *strings.Builder, delta string) {
	b.WriteString(SanitizeUTF8(delta))
}

// ValidDelta 判断增量是否是合法 UTF-8；用于决定是否上报 warn notice。
func ValidDelta(s string) bool { return utf8.ValidString(s) }
