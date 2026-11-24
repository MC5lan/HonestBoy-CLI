package render

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

var goldenSamples = map[string]string{
	"plain":   "hello 世界\n第二行",
	"heading": "# 标题\n\n正文 **加粗**",
	"list":    "- 第一项\n- 第二项\n  - 子项",
	"quote":   "> 引用一段话\n> 第二行",
	"code":    "```go\nfunc main() {\n\tfmt.Println(1)\n}\n```",
	"inline":  "普通文本，带 **加粗** 与 `inline code`。",
	"link":    "见 [文档](https://example.com/docs)",
	"image":   "![示意图](https://example.com/a.png)",
	"long":    "这是一段比较长的中文内容，用来检查折行与宽度计算是否稳定。这是一段比较长的中文内容，用来检查折行与宽度计算是否稳定。",
	"mixed":   "# 混合\n\n- 中文 item\n- English item\n\n> 引用一行\n\n```go\nfmt.Println(1)\n```",
}

var goldenWidths = []int{80, 100, 120, 200}

func normalizeGolden(s string) string {
	s = StripANSI(s)
	lines := strings.Split(s, "\n")
	for i := range lines {
		lines[i] = strings.TrimRight(lines[i], " ")
	}
	return strings.Join(lines, "\n")
}

func TestGoldenMarkdown(t *testing.T) {
	// 超链接与环境相关，快照固定为无 OSC8 的稳定输出。
	t.Setenv("TERM_PROGRAM", "Apple_Terminal")
	t.Setenv("WT_SESSION", "")
	t.Setenv("WEZTERM_PANE", "")
	dir := filepath.Join("testdata", "snapshots")
	if os.Getenv("UPDATE_SNAPSHOTS") == "1" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for name, src := range goldenSamples {
		for _, width := range goldenWidths {
			t.Run(fmt.Sprintf("%s_%d", name, width), func(t *testing.T) {
				r := New(width, "dark")
				got, err := r.Render(src)
				if err != nil {
					t.Fatal(err)
				}
				// 快照锁可见文本与行首布局，不锁终端色彩/行尾 padding。
				got = normalizeGolden(got)
				file := filepath.Join(dir, name+"_"+strconv.Itoa(width)+"_dark.golden")
				if os.Getenv("UPDATE_SNAPSHOTS") == "1" {
					if err := os.WriteFile(file, []byte(got), 0o644); err != nil {
						t.Fatal(err)
					}
					return
				}
				want, err := os.ReadFile(file)
				if err != nil {
					t.Fatalf("missing golden %s; run UPDATE_SNAPSHOTS=1 go test ./internal/tui/render", file)
				}
				if got != string(want) {
					t.Fatalf("golden mismatch %s\n--- got ---\n%s\n--- want ---\n%s", file, got, want)
				}
			})
		}
	}
}
