package tui

import (
	"strings"

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

func (b *block) Render(r *render.Renderer, width int, th theme.Theme) string {
	if !b.dirty && b.rendered != "" {
		return b.rendered
	}
	var out string
	switch b.kind {
	case blkUser:
		out = th.UserStyle().Render("› " + strings.TrimSpace(b.content))
	case blkAssistant:
		if isMarkdown(b.content) {
			md, err := r.Render(b.content)
			if err == nil && md != "" {
				out = md
				break
			}
		}
		// 普通文本：直接输出，不经过 glamour（避免整行 padding）。
		out = strings.TrimRight(b.content, "\n")
	case blkTool:
		action := core.ToolAction(b.meta["tool"], b.meta["args"])
		out = render.ToolCard(action, b.meta["args"], b.content, b.ok, b.duration, width, b.collapsed)
	case blkNotice:
		style := th.DimStyle()
		if b.meta["level"] == "warn" {
			style = th.WarnStyle()
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
	case blkTurnEnd:
		out = th.DimStyle().Render("— " + b.content)
	}
	b.rendered = out
	b.dirty = false
	return out
}
