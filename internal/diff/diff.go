// Package diff 生成可读的统一格式行级差异，供工具与界面共用。
// 纯函数、只依赖标准库，不引入任何 UI 或 IO。
package diff

import (
	"fmt"
	"strings"
)

// maxLCSLines 是精确 LCS 的行数上限；超过时退化为前缀/后缀摘要，
// 避免大文件把内存吃满（1200² 的 DP 表约 11MB）。
const maxLCSLines = 1200

// Unified 返回 path 从 before 到 after 的行级差异。
// 没有差异时返回空字符串。输出不含颜色，渲染层按 +/− 前缀着色。
func Unified(path, before, after string) string {
	before = strings.ReplaceAll(strings.ReplaceAll(before, "\r\n", "\n"), "\r", "\n")
	after = strings.ReplaceAll(strings.ReplaceAll(after, "\r\n", "\n"), "\r", "\n")
	if before == after {
		return ""
	}
	a := splitLines(before)
	b := splitLines(after)
	if len(a) > maxLCSLines || len(b) > maxLCSLines {
		return summary(path, a, b)
	}
	ops := lcsOps(a, b)
	if len(ops) == 0 {
		return ""
	}
	var sb strings.Builder
	fmt.Fprintf(&sb, "--- a/%s\n+++ b/%s\n", path, path)
	for _, h := range hunks(ops, 3) {
		writeHunk(&sb, h)
	}
	return sb.String()
}

type opKind uint8

const (
	opKeep opKind = iota
	opDel
	opAdd
)

type op struct {
	kind opKind
	text string
}

func splitLines(s string) []string {
	if s == "" {
		return nil
	}
	s = strings.TrimSuffix(s, "\n")
	if s == "" {
		return []string{""}
	}
	return strings.Split(s, "\n")
}

// lcsOps 用最长公共子序列把两份文本对齐；行数已由调用方限制。
func lcsOps(a, b []string) []op {
	n, m := len(a), len(b)
	dp := make([][]int32, n+1)
	for i := range dp {
		dp[i] = make([]int32, m+1)
	}
	for i := n - 1; i >= 0; i-- {
		for j := m - 1; j >= 0; j-- {
			if a[i] == b[j] {
				dp[i][j] = dp[i+1][j+1] + 1
			} else if dp[i+1][j] >= dp[i][j+1] {
				dp[i][j] = dp[i+1][j]
			} else {
				dp[i][j] = dp[i][j+1]
			}
		}
	}
	var ops []op
	i, j := 0, 0
	for i < n && j < m {
		switch {
		case a[i] == b[j]:
			ops = append(ops, op{opKeep, a[i]})
			i++
			j++
		case dp[i+1][j] >= dp[i][j+1]:
			ops = append(ops, op{opDel, a[i]})
			i++
		default:
			ops = append(ops, op{opAdd, b[j]})
			j++
		}
	}
	for ; i < n; i++ {
		ops = append(ops, op{opDel, a[i]})
	}
	for ; j < m; j++ {
		ops = append(ops, op{opAdd, b[j]})
	}
	return ops
}

type hunk struct {
	oldStart, oldCount int
	newStart, newCount int
	lines              []op
}

// hunks 把操作序列切成带上下文的块，间隔很近的改动合并。
func hunks(ops []op, context int) []hunk {
	var changed []int
	for i, o := range ops {
		if o.kind != opKeep {
			changed = append(changed, i)
		}
	}
	if len(changed) == 0 {
		return nil
	}
	var out []hunk
	start := maxInt(0, changed[0]-context)
	end := minInt(len(ops), changed[0]+context+1)
	for _, c := range changed[1:] {
		if c-context <= end {
			end = minInt(len(ops), c+context+1)
			continue
		}
		out = append(out, buildHunk(ops, start, end))
		start = maxInt(0, c-context)
		end = minInt(len(ops), c+context+1)
	}
	out = append(out, buildHunk(ops, start, end))
	return out
}

func buildHunk(ops []op, start, end int) hunk {
	h := hunk{oldStart: 1, newStart: 1}
	for i := 0; i < start; i++ {
		switch ops[i].kind {
		case opKeep:
			h.oldStart++
			h.newStart++
		case opDel:
			h.oldStart++
		case opAdd:
			h.newStart++
		}
	}
	for _, o := range ops[start:end] {
		h.lines = append(h.lines, o)
		switch o.kind {
		case opKeep:
			h.oldCount++
			h.newCount++
		case opDel:
			h.oldCount++
		case opAdd:
			h.newCount++
		}
	}
	return h
}

func writeHunk(sb *strings.Builder, h hunk) {
	fmt.Fprintf(sb, "@@ -%d,%d +%d,%d @@\n", h.oldStart, h.oldCount, h.newStart, h.newCount)
	for _, o := range h.lines {
		switch o.kind {
		case opKeep:
			sb.WriteString(" " + o.text + "\n")
		case opDel:
			sb.WriteString("-" + o.text + "\n")
		case opAdd:
			sb.WriteString("+" + o.text + "\n")
		}
	}
}

// summary 用于超大文件：只报告前后缀与变更规模，不逐行比对。
func summary(path string, a, b []string) string {
	prefix := 0
	for prefix < len(a) && prefix < len(b) && a[prefix] == b[prefix] {
		prefix++
	}
	suffix := 0
	for suffix < len(a)-prefix && suffix < len(b)-prefix &&
		a[len(a)-1-suffix] == b[len(b)-1-suffix] {
		suffix++
	}
	var sb strings.Builder
	fmt.Fprintf(&sb, "--- a/%s\n+++ b/%s\n", path, path)
	fmt.Fprintf(&sb, "@@ 省略逐行比对：%d 行 → %d 行（公共前缀 %d 行，公共后缀 %d 行）@@\n",
		len(a), len(b), prefix, suffix)
	for _, line := range a[prefix : len(a)-suffix] {
		sb.WriteString("-" + line + "\n")
	}
	for _, line := range b[prefix : len(b)-suffix] {
		sb.WriteString("+" + line + "\n")
	}
	return sb.String()
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
