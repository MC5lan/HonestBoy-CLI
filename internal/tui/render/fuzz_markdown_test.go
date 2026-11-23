package render

import (
	"strings"
	"testing"
)

// FuzzMarkdown 对 Markdown 渲染做模糊测试（TA08）：任意输入不得 panic。
// 渲染失败允许降级为原文，但不能崩溃。
func FuzzMarkdown(f *testing.F) {
	f.Add("# 标题\n\n正文 **粗体**\n")
	f.Add("- a\n- b\n\n```go\nfunc main() {}\n```")
	f.Add("| a | b |\n|---|---|\n| 1 | 2 |")
	f.Add("")
	f.Fuzz(func(t *testing.T, src string) {
		if len(src) > 4096 {
			return
		}
		r := New(40, "dark")
		out, err := r.Render(src)
		if err != nil {
			t.Fatalf("Render returned error: %v", err)
		}
		_ = strings.TrimSpace(out)
	})
}
