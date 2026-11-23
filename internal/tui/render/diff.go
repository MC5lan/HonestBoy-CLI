package render

import (
	"fmt"
	"strings"
)

// Diff 生成按行 LCS 差异，带行号与 +/− 前缀（非 TTY 下仍可辨识增删）。
func Diff(before, after string) string {
	a := strings.Split(strings.TrimRight(before, "\n"), "\n")
	b := strings.Split(strings.TrimRight(after, "\n"), "\n")
	n, m := len(a), len(b)
	dp := make([][]int, n+1)
	for i := range dp {
		dp[i] = make([]int, m+1)
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
	type row struct {
		mark string
		num  int
		text string
	}
	var rows []row
	i, j := 0, 0
	for i < n && j < m {
		switch {
		case a[i] == b[j]:
			rows = append(rows, row{" ", j + 1, b[j]})
			i++
			j++
		case dp[i+1][j] >= dp[i][j+1]:
			rows = append(rows, row{"-", i + 1, a[i]})
			i++
		default:
			rows = append(rows, row{"+", j + 1, b[j]})
			j++
		}
	}
	for ; i < n; i++ {
		rows = append(rows, row{"-", i + 1, a[i]})
	}
	for ; j < m; j++ {
		rows = append(rows, row{"+", j + 1, b[j]})
	}
	var out strings.Builder
	for _, r := range rows {
		fmt.Fprintf(&out, "%s%4d │ %s\n", r.mark, r.num, r.text)
	}
	return strings.TrimRight(out.String(), "\n")
}
