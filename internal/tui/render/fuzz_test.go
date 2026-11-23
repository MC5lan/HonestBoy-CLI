package render

import (
	"strings"
	"testing"
)

// FuzzDiff 对 diff 生成器做模糊测试（TA08）：任意输入不得 panic，
// 且每一行的标记必须是 " " / "-" / "+" 之一。
func FuzzDiff(f *testing.F) {
	f.Add("a\nb\n", "a\nc\n")
	f.Add("", "")
	f.Add("x", "x\nx")
	f.Add(strings.Repeat("line\n", 40), strings.Repeat("line\n", 39)+"tail\n")
	f.Fuzz(func(t *testing.T, before, after string) {
		// 限制规模，避免 LCS 的 O(n*m) 在模糊测试里失控。
		if len(before) > 4096 || len(after) > 4096 {
			return
		}
		out := Diff(before, after)
		for _, line := range strings.Split(out, "\n") {
			if line == "" {
				continue
			}
			if line[0] != ' ' && line[0] != '-' && line[0] != '+' {
				t.Fatalf("bad diff line %q", line)
			}
		}
	})
}
