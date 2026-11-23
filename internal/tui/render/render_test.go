package render

import (
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
