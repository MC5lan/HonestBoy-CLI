package engine

import (
	"strings"

	"github.com/MC5lan/HonestBoy-CLI/internal/core"
	"github.com/MC5lan/HonestBoy-CLI/internal/llm"
)

// Budget 是上下文预算（T211）。Ratio 默认 0.7，保留最近 KeepRecent 轮。
type Budget struct {
	Ratio      float64
	KeepRecent int
	WindowSize int // 模型上下文窗口 token 数；0 时用默认 128k
}

// Limit 返回留给历史的 token 预算。
func (b Budget) Limit() int {
	window := b.WindowSize
	if window <= 0 {
		window = 128000
	}
	ratio := b.Ratio
	if ratio <= 0 {
		ratio = 0.7
	}
	return int(float64(window) * ratio)
}

// Trim 裁剪消息序列：system 永远保留；最近 KeepRecent 轮完整保留；
// 更早的 tool 结果截断到 200 字符，再旧的非 system 消息丢弃。
// 返回裁剪后的消息与被丢弃/压缩的条数。
func (b Budget) Trim(msgs []llm.Message, limit int) ([]llm.Message, int) {
	if len(msgs) <= 1 || limit <= 0 {
		return msgs, 0
	}
	total := 0
	for _, m := range msgs {
		total += core.TokenEstimate(m.Content)
	}
	if total <= limit {
		return msgs, 0
	}

	keep := b.KeepRecent
	if keep <= 0 {
		keep = 2
	}
	// 完整保留的尾部：system(0) 之外的最近 keep*2 条（一轮=user+assistant）。
	tail := keep * 2
	if tail > len(msgs)-1 {
		tail = len(msgs) - 1
	}
	headEnd := len(msgs) - tail

	out := make([]llm.Message, 0, len(msgs))
	out = append(out, msgs[0]) // system
	removed := 0
	for i := 1; i < headEnd; i++ {
		m := msgs[i]
		if m.Role == "tool" {
			// 工具结果保留原文摘要（截断到 200 字符），这是可审计性的下限。
			r := []rune(m.Content)
			if len(r) > 200 {
				m.Content = string(r[:200]) + " …[历史工具输出已压缩]"
			}
			out = append(out, m)
		} else {
			removed++ // 旧对话整条丢弃
		}
	}
	out = append(out, msgs[headEnd:]...)
	return out, removed
}

// Usage 返回当前消息的大致 token 占用，供状态栏显示。
func Usage(msgs []llm.Message) int {
	n := 0
	for _, m := range msgs {
		n += core.TokenEstimate(m.Content)
	}
	return n
}

// CompactMark 是注入上下文的压缩提示，明确告知模型历史已被裁剪。
const CompactMark = "（较早的对话与工具输出已被上下文压缩，仅保留摘要；如需细节请重新读取文件。）"

// WithCompactMark 在 system 消息末尾附加压缩标记。
func WithCompactMark(system string, compacted bool) string {
	if !compacted {
		return system
	}
	return strings.TrimRight(system, "\n") + "\n\n" + CompactMark
}
