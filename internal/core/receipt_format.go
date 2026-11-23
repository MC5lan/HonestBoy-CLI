package core

import (
	"fmt"
	"strings"
)

// FormatEvidence 生成 TUI/CLI 共用的一行证据展示。
// 宽度 ≥60 列不折行；调用方在 <60 列时自行降级为两行。
func FormatEvidence(e Evidence) string {
	mark := "✗"
	if e.OK {
		mark = "✓"
	}
	dur := formatDuration(e.Duration)
	summary := strings.ReplaceAll(e.Summary, "\n", " ")
	if len([]rune(summary)) > 60 {
		summary = string([]rune(summary)[:57]) + "..."
	}
	return fmt.Sprintf("%s %-8s %-28s %8s  %q", mark, e.Kind, e.Target, dur, summary)
}

func formatDuration(ms int64) string {
	if ms < 1000 {
		return fmt.Sprintf("%dms", ms)
	}
	return fmt.Sprintf("%.1fs", float64(ms)/1000)
}

// FormatEvidenceCompact 在窄终端下把摘要换到第二行。
func FormatEvidenceCompact(e Evidence) string {
	first := FormatEvidence(e)
	if len([]rune(first)) <= 60 {
		return first
	}
	return strings.TrimSpace(first[:strings.LastIndex(first, "  \"")]) + "\n    " + e.Summary
}
