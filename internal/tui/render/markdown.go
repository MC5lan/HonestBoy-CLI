package render

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/charmbracelet/glamour"
)

const renderCacheMax = 200

// Renderer 是固定宽度与主题的 Markdown 渲染器。
// cache 是 200 块的 LRU 缓存，键包含内容、宽度、主题与代码展开状态。
type Renderer struct {
	width    int
	theme    string
	glam     *glamour.TermRenderer
	cache    map[string]string
	cacheLRU []string
}

func New(width int, theme string) *Renderer {
	return &Renderer{width: width, theme: theme, cache: map[string]string{}}
}

func (r *Renderer) Render(src string) (string, error) {
	return r.RenderWithOptions(src, false)
}

// RenderWithOptions 允许 Ctrl+O 展开长代码块；结果进入 LRU 缓存。
func (r *Renderer) RenderWithOptions(src string, expandCode bool) (string, error) {
	if r.width <= 0 || strings.TrimSpace(src) == "" {
		return src, nil
	}
	key := r.cacheKey(src, expandCode)
	if v, ok := r.cacheGet(key); ok {
		return v, nil
	}
	prepared := prepareMarkdown(src, expandCode)
	if r.glam == nil {
		style := "dark"
		switch r.theme {
		case "light":
			style = "light"
		case "none":
			style = "notty" // NO_COLOR：Markdown 也不能输出颜色码
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
	out, err := r.glam.Render(prepared)
	if err != nil {
		return src, nil
	}
	out = strings.TrimRight(out, "\n")
	if LinkSupported() {
		out = WrapURLs(out)
	}
	r.cachePut(key, out)
	return out, nil
}

func (r *Renderer) cacheKey(src string, expand bool) string {
	sum := sha256.Sum256([]byte(src))
	return hex.EncodeToString(sum[:]) + fmt.Sprintf("|%d|%s|%v", r.width, r.theme, expand)
}

func (r *Renderer) cacheGet(key string) (string, bool) {
	v, ok := r.cache[key]
	if !ok {
		return "", false
	}
	for i, k := range r.cacheLRU {
		if k == key {
			r.cacheLRU = append(append(r.cacheLRU[:i], r.cacheLRU[i+1:]...), key)
			break
		}
	}
	return v, true
}

func (r *Renderer) cachePut(key, value string) {
	if _, exists := r.cache[key]; !exists {
		r.cacheLRU = append(r.cacheLRU, key)
	}
	r.cache[key] = value
	for len(r.cacheLRU) > renderCacheMax {
		old := r.cacheLRU[0]
		r.cacheLRU = r.cacheLRU[1:]
		delete(r.cache, old)
	}
}

// Invalidate 在宽度变化时重建 glamour 实例并清空缓存。
func (r *Renderer) Invalidate(newWidth int) {
	if newWidth != r.width {
		r.width = newWidth
		r.glam = nil
		r.cache = map[string]string{}
		r.cacheLRU = nil
	}
}
