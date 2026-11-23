package render

import (
	"strings"

	"github.com/charmbracelet/glamour"
)

// Renderer 是固定宽度与主题的 Markdown 渲染器。
type Renderer struct {
	width int
	theme string
	glam  *glamour.TermRenderer
}

func New(width int, theme string) *Renderer {
	return &Renderer{width: width, theme: theme}
}

func (r *Renderer) Render(src string) (string, error) {
	if r.width <= 0 || strings.TrimSpace(src) == "" {
		return src, nil
	}
	if r.glam == nil {
		style := "dark"
		if r.theme == "light" {
			style = "light"
		}
		wrap := r.width - 4
		if wrap < 10 {
			wrap = 10
		}
		gr, err := glamour.NewTermRenderer(
			glamour.WithStandardStyle(style),
			glamour.WithWordWrap(wrap),
		)
		if err != nil {
			return src, nil // 渲染失败降级为原文
		}
		r.glam = gr
	}
	out, err := r.glam.Render(src)
	if err != nil {
		return src, nil
	}
	return strings.TrimRight(out, "\n"), nil
}

// Invalidate 在宽度变化时重建 glamour 实例。
func (r *Renderer) Invalidate(newWidth int) {
	if newWidth != r.width {
		r.width = newWidth
		r.glam = nil
	}
}
