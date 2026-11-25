package core

import (
	"strings"
	"testing"
)

func TestVisibleTextFiltersControlBlocks(t *testing.T) {
	raw := `[SELF]
conf: high
missing: none
assumption: none

我先看一下文件。
[TEST PLAN]
- A1: go build ./...
- A2: go test ./...

然后开始修改。
`
	got := VisibleText(raw)
	if strings.Contains(got, "[SELF]") || strings.Contains(got, "conf: high") {
		t.Fatalf("self block leaked: %q", got)
	}
	if strings.Contains(got, "[TEST PLAN]") || strings.Contains(got, "A1:") {
		t.Fatalf("test plan leaked: %q", got)
	}
	for _, want := range []string{"我先看一下文件。", "然后开始修改。"} {
		if !strings.Contains(got, want) {
			t.Fatalf("visible text missing %q: %q", want, got)
		}
	}
}

func TestVisibleTextStreamingSplit(t *testing.T) {
	f := &VisibleTextFilter{}
	var out strings.Builder
	for _, part := range []string{"[SEL", "F]\ncon", "f: high\nmissing: none\n\n正常", "文字\n"} {
		out.WriteString(f.Write(part))
	}
	out.WriteString(f.Flush())
	got := out.String()
	if got != "正常文字\n" {
		t.Fatalf("got %q, want %q", got, "正常文字\n")
	}
}

func TestVisibleTextWithoutBlankLine(t *testing.T) {
	raw := "[SELF]\nconf: high\nassumption: none\n直接开始。\n"
	if got := VisibleText(raw); got != "直接开始。\n" {
		t.Fatalf("got %q", got)
	}
}

func TestVisibleTextPartialControlDropped(t *testing.T) {
	f := &VisibleTextFilter{}
	if got := f.Write("[SELF]\nconf: hig"); got != "" {
		t.Fatalf("partial control block leaked: %q", got)
	}
	if got := f.Flush(); got != "" {
		t.Fatalf("partial control block leaked on flush: %q", got)
	}
}

func TestVisibleTextNormalUnchanged(t *testing.T) {
	raw := "普通文本\n第二行\n"
	if got := VisibleText(raw); got != raw {
		t.Fatalf("got %q", got)
	}
	// 普通正文立即输出，不必等到换行。
	f := &VisibleTextFilter{}
	if got := f.Write("没有换行"); got != "没有换行" {
		t.Fatalf("unexpected %q", got)
	}
	if got := f.Flush(); got != "" {
		t.Fatalf("flush = %q", got)
	}
	// 只有 [ 开头且可能成为控制块的行会缓冲。
	f = &VisibleTextFilter{}
	if got := f.Write("[SEL"); got != "" {
		t.Fatalf("control prefix leaked: %q", got)
	}
	if got := f.Flush(); got != "" {
		t.Fatalf("control flush leaked: %q", got)
	}
}
