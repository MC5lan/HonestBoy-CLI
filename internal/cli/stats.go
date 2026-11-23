package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/MC5lan/HonestBoy-CLI/internal/core"
	"github.com/MC5lan/HonestBoy-CLI/internal/store"
)

// RunStats 输出会话级诚实度报表。
func RunStats(opts GlobalOptions, args []string) error {
	ss := store.NewSessionStore(store.DefaultPaths())
	receipts, err := ss.AllReceipts()
	if err != nil {
		return fmt.Errorf("%w: %v", ErrEnvironment, err)
	}
	metas, err := ss.List()
	if err != nil {
		return fmt.Errorf("%w: %v", ErrEnvironment, err)
	}
	if len(receipts) == 0 {
		fmt.Fprintln(os.Stdout, "暂无证据记录")
		return nil
	}
	counts := map[core.Terminal]int{}
	for _, r := range receipts {
		counts[r.Terminal]++
	}
	total := len(receipts)
	fmt.Fprintf(os.Stdout, "会话统计：%d 个会话，%d 条回合记录\n", len(metas), total)
	for _, t := range []core.Terminal{core.TermDone, core.TermUntest, core.TermDenied, core.TermHalt, core.TermFailed} {
		n := counts[t]
		pct := float64(n) * 100 / float64(total)
		fmt.Fprintf(os.Stdout, "  %-10s %4d  (%5.1f%%)\n", t, n, pct)
	}
	// 核心指标（设计文档 §6.1）：FCR = 声称完成但被 Gate 拒绝 / 全部完成声称
	claims := counts[core.TermDone] + counts[core.TermDenied]
	if claims > 0 {
		fcr := float64(counts[core.TermDenied]) * 100 / float64(claims)
		fmt.Fprintf(os.Stdout, "\n虚报完成率 FCR: %.1f%%（目标 <2%%）\n", fcr)
	}
	// UCR：从 trace 计算未证实断言率
	if ucr, n, ok := readUCR(); ok {
		fmt.Fprintf(os.Stdout, "未证实断言率 UCR: %.1f%%（%d 条断言，目标 <15%%）\n", ucr*100, n)
	}
	// 弃权正确率 / 过度弃权率成对展示（文档：只提 APR 会推高 OPR）
	if counts[core.TermUntest] > 0 || counts[core.TermDone] > 0 {
		fmt.Fprintf(os.Stdout, "弃权占比: %.1f%%（长期为 0 说明模型在硬撑；过高说明工具不足）\n",
			float64(counts[core.TermUntest])*100/float64(total))
	}
	if counts[core.TermUntest] == 0 && counts[core.TermDone] > 0 {
		fmt.Fprintln(os.Stdout, "提示：untested 占比为 0，请检查 Gate 是否失效")
	}
	return nil
}

// readUCR 从 trace.jsonl 统计未证实断言率。
func readUCR() (float64, int, bool) {
	path := filepath.Join(store.DefaultPaths().Root, "trace", "trace.jsonl")
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, 0, false
	}
	total, ungrounded := 0, 0
	for _, line := range strings.Split(string(data), "\n") {
		if !strings.Contains(line, `"grounded"`) {
			continue
		}
		total++
		if strings.Contains(line, `"grounded":false`) {
			ungrounded++
		}
	}
	if total == 0 {
		return 0, 0, false
	}
	return float64(ungrounded) / float64(total), total, true
}
