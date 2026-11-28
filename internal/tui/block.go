package tui

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/MC5lan/HonestBoy-CLI/internal/core"
	"github.com/MC5lan/HonestBoy-CLI/internal/tui/render"
	"github.com/MC5lan/HonestBoy-CLI/internal/tui/theme"
)

type blockKind uint8

const (
	blkUser blockKind = iota
	blkAssistant
	blkTool
	blkNotice
	blkEvidence
	blkError
	blkAbstain
	blkGateDeny
	blkGateAllow
	blkTurnEnd
)

type block struct {
	kind      blockKind
	content   string
	rendered  string
	dirty     bool
	collapsed bool
	meta      map[string]string
	duration  int64
	ok        bool
	expires   time.Time // 非零表示 info 通知，到点后从对话中移除
	// Markdown 增量渲染缓存：mdKey 是已渲染的稳定前缀字节数。
	mdKey int
	mdOut string
}

// isMarkdown 判断文本是否含真正的 Markdown 结构，避免把普通文本送进 glamour
// 导致整行 padding 与多余 ANSI（这是之前渲染脏乱的根因）。
func isMarkdown(s string) bool {
	for _, line := range strings.Split(s, "\n") {
		t := strings.TrimSpace(line)
		if strings.HasPrefix(t, "# ") || strings.HasPrefix(t, "## ") ||
			strings.HasPrefix(t, "- ") || strings.HasPrefix(t, "* ") ||
			strings.HasPrefix(t, "> ") || strings.HasPrefix(t, "```") ||
			strings.HasPrefix(t, "| ") {
			return true
		}
	}
	return false
}

// RenderWith 的 verbose 控制工具输出是否展开；失败工具始终展开。
func (b *block) RenderWith(r *render.Renderer, width int, th theme.Theme, verbose bool) string {
	if !b.dirty && b.rendered != "" {
		return b.rendered
	}
	var out string
	switch b.kind {
	case blkUser:
		out = th.UserStyle().Render(formatUser(b.content))
	case blkAssistant:
		if isMarkdown(b.content) {
			stable := stableMarkdownPrefix(b.content)
			if stable == 0 {
				// 还没有完整段落：先按原文显示，避免每个 delta 都重跑 glamour。
				out = strings.TrimRight(b.content, "\n")
				break
			}
			if stable != b.mdKey || b.mdOut == "" {
				if md, err := r.RenderWithOptions(b.content[:stable], verbose); err == nil {
					b.mdKey, b.mdOut = stable, md
				}
			}
			if b.mdOut != "" {
				tail := strings.TrimRight(b.content[stable:], "\n")
				if tail != "" {
					out = b.mdOut + "\n" + tail
				} else {
					out = b.mdOut
				}
				break
			}
		}
		out = strings.TrimRight(b.content, "\n")
	case blkTool:
		expanded := verbose || !b.collapsed || !b.ok
		action := core.ToolAction(b.meta["tool"], b.meta["args"])
		out = render.ToolCard(action, b.meta["args"], b.content, b.ok, b.duration, width, !expanded)
		if expanded {
			if d := b.meta["diff"]; d != "" {
				out += "\n" + colorDiff(d, th, width)
			}
		}
	case blkNotice:
		style := th.DimStyle()
		switch b.meta["level"] {
		case "warn":
			style = th.WarnStyle()
		case "error":
			style = th.DangerStyle()
		}
		out = style.Render("! " + b.content)
	case blkError:
		out = th.DangerStyle().Render("✗ " + b.content)
	case blkEvidence:
		out = th.SuccessStyle().Render("✓ " + b.content)
	case blkAbstain:
		out = th.WarnStyle().Render("⏸ " + b.content)
	case blkGateDeny:
		out = th.DangerStyle().Render("✗ Gate 拒绝 [" + b.meta["rule"] + "] " + b.content)
	case blkGateAllow:
		out = th.SuccessStyle().Render("✓ Gate 通过")
		if strings.TrimSpace(b.content) != "" {
			out += "：" + b.content
		}
	case blkTurnEnd:
		out = th.DimStyle().Render("— " + b.content)
	}
	b.rendered = out
	b.dirty = false
	return out
}

// stableMarkdownPrefix 返回最后一个完整段落（空行）之前的字节数。
// 代码块内部不切分，避免把未闭合的 ``` 送去渲染。
func stableMarkdownPrefix(s string) int {
	stable := 0
	inFence := false
	offset := 0
	for _, line := range strings.SplitAfter(s, "\n") {
		t := strings.TrimSpace(line)
		if strings.HasPrefix(t, "```") {
			inFence = !inFence
		}
		offset += len(line)
		if t == "" && !inFence {
			stable = offset
		}
	}
	return stable
}

func formatUser(s string) string {
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	var b strings.Builder
	for i, line := range lines {
		if i > 0 {
			b.WriteString("\n  ")
		} else {
			b.WriteString("› ")
		}
		b.WriteString(line)
	}
	return b.String()
}

// colorDiff 只做前缀着色，不改写验证器/工具产出的原文。
func parseHunkStart(line string) (int, int) {
	fields := strings.Fields(line)
	if len(fields) < 3 {
		return 1, 1
	}
	old := strings.TrimPrefix(fields[1], "-")
	newer := strings.TrimPrefix(fields[2], "+")
	parse := func(s string) int {
		if i := strings.Index(s, ","); i >= 0 {
			s = s[:i]
		}
		n, err := strconv.Atoi(s)
		if err != nil {
			return 1
		}
		return n
	}
	return parse(old), parse(newer)
}

func colorDiff(d string, th theme.Theme, width int) string {
	lines := strings.Split(strings.TrimRight(d, "\n"), "\n")
	const maxLines = 120
	if len(lines) > maxLines {
		lines = append(lines[:maxLines], "… 其余 diff 已省略")
	}
	var out []string
	oldNo, newNo := 0, 0
	contentWidth := width - 13
	if contentWidth < 10 {
		contentWidth = 10
	}
	for _, line := range lines {
		if strings.HasPrefix(line, "@@") {
			oldNo, newNo = parseHunkStart(line)
			out = append(out, th.AccentStyle().Render(render.Truncate(line, width)))
			continue
		}
		if strings.HasPrefix(line, "+++") || strings.HasPrefix(line, "---") {
			out = append(out, th.DimStyle().Render(render.Truncate(line, width)))
			continue
		}
		prefix := " "
		text := line
		if len(line) > 0 {
			prefix, text = line[:1], line[1:]
		}
		var num string
		switch prefix {
		case "+":
			num = fmt.Sprintf("     %4d +", newNo)
			newNo++
		case "-":
			num = fmt.Sprintf("%4d      -", oldNo)
			oldNo++
		default:
			num = fmt.Sprintf("%4d %4d  ", oldNo, newNo)
			oldNo++
			newNo++
		}
		formatted := num + render.Truncate(text, contentWidth)
		switch prefix {
		case "+":
			out = append(out, th.SuccessStyle().Render(formatted))
		case "-":
			out = append(out, th.DangerStyle().Render(formatted))
		default:
			out = append(out, th.DimStyle().Render(formatted))
		}
	}
	return strings.Join(out, "\n")
}
