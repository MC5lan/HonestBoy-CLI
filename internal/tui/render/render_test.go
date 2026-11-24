package render

import (
	"fmt"
	"strings"
	"testing"
)

func TestWidthAndTruncate(t *testing.T) {
	cases := []struct {
		s    string
		want int
	}{
		{"hello", 5},
		{"中文测试", 8},
		{"a👨‍👩‍👧b", 4}, // ZWJ 家庭 emoji 按 2 计，不逐码点累加
		{"e\u0301", 1},
	}
	for _, tc := range cases {
		if got := Width(tc.s); got != tc.want {
			t.Fatalf("Width(%q) = %d, want %d", tc.s, got, tc.want)
		}
	}
	if got := Truncate("中文测试", 4); got != "中文" {
		t.Fatalf("Truncate = %q", got)
	}
	if got := Pad("中", 4); got != "中  " {
		t.Fatalf("Pad = %q", got)
	}
}

func TestDiff(t *testing.T) {
	out := Diff("a\nb\nc", "a\nB\nc")
	if !strings.Contains(out, "-") || !strings.Contains(out, "+") || !strings.Contains(out, "3 │ c") {
		t.Fatalf("diff = %q", out)
	}
}

func TestTable(t *testing.T) {
	out := Table([]string{"名称", "值"}, [][]string{{"abc", "123"}, {"中文", "4"}})
	lines := strings.Split(out, "\n")
	if len(lines) != 4 || !strings.Contains(lines[0], "名称") || !strings.Contains(lines[3], "中文") {
		t.Fatalf("table = %q", out)
	}
}

func TestToolCard(t *testing.T) {
	collapsed := ToolCard("edit_file", `{"path":"a.go"}`, "ok", true, 12, 80, true)
	if strings.Contains(collapsed, "\n") || !strings.Contains(collapsed, "edit_file") {
		t.Fatalf("collapsed card = %q", collapsed)
	}
	expanded := ToolCard("shell", "echo hi", "line1\nline2", false, 30, 80, true)
	if !strings.Contains(expanded, "line2") || !strings.Contains(expanded, "✗") {
		t.Fatalf("failed card should expand: %q", expanded)
	}
}

func TestMarkdownCodeFoldAndExpand(t *testing.T) {
	var b strings.Builder
	b.WriteString("```go\n")
	for i := 1; i <= 260; i++ {
		b.WriteString("fmt.Println(1)\n")
	}
	b.WriteString("```\n")
	r := New(100, "dark")
	folded, err := r.Render(b.String())
	if err != nil || !strings.Contains(folded, "折叠") {
		t.Fatalf("folded code missing marker: %v\n%s", err, folded)
	}
	expanded, err := r.RenderWithOptions(b.String(), true)
	if err != nil || strings.Contains(expanded, "折叠") {
		t.Fatalf("expanded code still folded: %v\n%s", err, expanded)
	}
}

func TestMarkdownImageAndBase64Fallback(t *testing.T) {
	src := "![示意图](https://example.com/a.png)\n\n" + strings.Repeat("A", 300) + "\n"
	r := New(100, "none")
	out, err := r.Render(src)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "[图片: 示意图") || !strings.Contains(out, "[base64") {
		t.Fatalf("fallback missing:\n%s", out)
	}
}

func TestWrapURLs(t *testing.T) {
	t.Setenv("TERM_PROGRAM", "vscode")
	got := WrapURLs("see https://example.com/x for details")
	if !strings.Contains(got, "\x1b]8;;https://example.com/x") {
		t.Fatalf("OSC8 link missing: %q", got)
	}
	t.Setenv("TERM_PROGRAM", "Apple_Terminal")
	if got := WrapURLs("see https://example.com/x"); strings.Contains(got, "\x1b]8;;") {
		t.Fatalf("unsupported terminal should not get OSC8: %q", got)
	}
}

func TestRendererCacheLRU(t *testing.T) {
	r := New(80, "none")
	for i := 0; i < renderCacheMax+10; i++ {
		_, _ = r.Render("## " + fmt.Sprint(i))
	}
	if len(r.cache) != renderCacheMax {
		t.Fatalf("cache size = %d, want %d", len(r.cache), renderCacheMax)
	}
}

func BenchmarkMarkdown200Code(b *testing.B) {
	var sb strings.Builder
	sb.WriteString("```go\n")
	for i := 0; i < 200; i++ {
		sb.WriteString("fmt.Println(\"line\")\n")
	}
	sb.WriteString("```\n")
	src := sb.String()
	r := New(100, "dark")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r.cache = map[string]string{}
		r.cacheLRU = nil
		_, _ = r.RenderWithOptions(src, true)
	}
}

func TestMarkdownAndCode(t *testing.T) {
	r := New(80, "none")
	out, err := r.Render("# 标题\n\n正文")
	if err != nil || !strings.Contains(out, "标题") {
		t.Fatalf("markdown = %q, %v", out, err)
	}
	r.Invalidate(60)
	if r.width != 60 || r.glam != nil {
		t.Fatal("Invalidate should reset renderer")
	}
	code, err := Highlight("func main() {}", "go", "dark")
	if err != nil || !strings.Contains(code, "1 │") {
		t.Fatalf("highlight = %q, %v", code, err)
	}
	folded, ok := FoldCode(strings.Repeat("x\n", 250), 200)
	if !ok || !strings.Contains(folded, "折叠") {
		t.Fatal("FoldCode failed")
	}
}
