package core

import "strings"

// IsControlTool 标记只用于协议出口、不需要在界面上单独展示的工具。
func IsControlTool(name string) bool {
	return name == "request_done" || name == "declare_untested"
}

// VisibleTextFilter 从流式文本中剔除框架控制块（[SELF] / [TEST PLAN]）。
// 只影响展示与历史注入，不改变引擎用于判定的原始文本。
type VisibleTextFilter struct {
	pending string
	mode    int // 0 normal | 1 self | 2 plan
}

// Reset 清空状态，用于新回合或重新回放。
func (f *VisibleTextFilter) Reset() {
	f.pending = ""
	f.mode = 0
}

// Write 接收一段流式文本，返回可以展示的完整行。
func (f *VisibleTextFilter) Write(delta string) string {
	if delta == "" {
		return ""
	}
	f.pending += delta
	var out strings.Builder
	for {
		i := strings.IndexByte(f.pending, '\n')
		if i < 0 {
			break
		}
		line := f.pending[:i+1]
		f.pending = f.pending[i+1:]
		f.processLine(&out, line)
	}
	// 普通正文不必等到换行才显示；只有以 '[' 开头的行需要缓冲，
	// 以便识别 [SELF] / [TEST PLAN] 控制块。
	if f.mode == 0 && f.pending != "" && f.pending[0] != '[' {
		out.WriteString(f.pending)
		f.pending = ""
	}
	return out.String()
}

// Flush 输出最后一段没有换行的普通文本；不完整的控制块直接丢弃。
func (f *VisibleTextFilter) Flush() string {
	if f.pending == "" {
		return ""
	}
	line := f.pending
	f.pending = ""
	if f.mode != 0 {
		return ""
	}
	if isControlMarker(strings.TrimSpace(line)) {
		return ""
	}
	return line
}

func isControlMarker(s string) bool {
	for _, marker := range []string{"[SELF]", "[TEST PLAN]"} {
		if strings.HasPrefix(marker, s) || strings.HasPrefix(s, marker) {
			return true
		}
	}
	return false
}

// VisibleText 是一次性过滤完整字符串的便捷函数。
func VisibleText(raw string) string {
	f := &VisibleTextFilter{}
	return f.Write(raw) + f.Flush()
}

func (f *VisibleTextFilter) processLine(out *strings.Builder, line string) {
	trimmed := strings.TrimSpace(line)
	switch f.mode {
	case 1: // [SELF]
		if trimmed == "" {
			f.mode = 0
			return
		}
		if isSelfLine(trimmed) {
			return
		}
		f.mode = 0 // 不是自评字段，按普通文本处理
	case 2: // [TEST PLAN]
		if trimmed == "" {
			f.mode = 0
			return
		}
		if isPlanItem(trimmed) {
			return
		}
		f.mode = 0
	}
	switch {
	case strings.EqualFold(trimmed, "[SELF]"):
		f.mode = 1
		return
	case strings.EqualFold(trimmed, "[TEST PLAN]"):
		f.mode = 2
		return
	}
	out.WriteString(line)
}

func isSelfLine(s string) bool {
	i := strings.IndexByte(s, ':')
	if i <= 0 {
		return false
	}
	key := strings.TrimSpace(s[:i])
	if key == "" {
		return false
	}
	for _, r := range key {
		if r == '_' || r == '-' || r == ' ' ||
			(r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			continue
		}
		return false
	}
	return true
}

func isPlanItem(s string) bool {
	if strings.HasPrefix(s, "- ") || strings.HasPrefix(s, "* ") || strings.HasPrefix(s, "• ") {
		return true
	}
	return len(s) > 2 && s[0] >= '0' && s[0] <= '9' && (s[1] == '.' || s[1] == ')')
}
