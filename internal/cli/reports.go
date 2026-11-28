package cli

import (
	"fmt"
	"strings"

	"github.com/MC5lan/HonestBoy-CLI/internal/core"
	"github.com/MC5lan/HonestBoy-CLI/internal/store"
)

// VerifyReport 返回会话证据链校验文本（供 TUI 内联展示）。
func VerifyReport() (string, error) {
	ss := store.NewSessionStore(store.DefaultPaths())
	metas, err := ss.List()
	if err != nil {
		return "", err
	}
	if len(metas) == 0 {
		return "暂无会话记录", nil
	}
	var b strings.Builder
	for _, m := range metas {
		sess, err := ss.Load(m.ID)
		if err != nil {
			fmt.Fprintf(&b, "会话 %s: 读取失败\n", m.ID)
			continue
		}
		at, err := core.VerifyChain(sess.Receipts)
		if err != nil {
			fmt.Fprintf(&b, "会话 %s: 第 %d 条断裂\n", m.ID, at+1)
			continue
		}
		fmt.Fprintf(&b, "会话 %s: %d 条记录，链完整\n", m.ID, len(sess.Receipts))
	}
	return strings.TrimRight(b.String(), "\n"), nil
}

// StatsReport 返回诚实度统计文本（供 TUI 内联展示）。
func StatsReport() (string, error) {
	ss := store.NewSessionStore(store.DefaultPaths())
	receipts, err := ss.AllReceipts()
	if err != nil {
		return "", err
	}
	metas, err := ss.List()
	if err != nil {
		return "", err
	}
	if len(receipts) == 0 {
		return "暂无证据记录", nil
	}
	counts := map[core.Terminal]int{}
	for _, r := range receipts {
		counts[r.Terminal]++
	}
	total := len(receipts)
	var b strings.Builder
	fmt.Fprintf(&b, "会话 %d 个，回合记录 %d 条\n", len(metas), total)
	for _, t := range []core.Terminal{core.TermDone, core.TermUntest, core.TermDenied, core.TermHalt, core.TermFailed} {
		fmt.Fprintf(&b, "%-9s %4d  (%5.1f%%)\n", t, counts[t], float64(counts[t])*100/float64(total))
	}
	if claims := counts[core.TermDone] + counts[core.TermDenied]; claims > 0 {
		fmt.Fprintf(&b, "FCR 虚报完成率 %.1f%%（目标 <2%%）\n",
			float64(counts[core.TermDenied])*100/float64(claims))
	}
	return strings.TrimRight(b.String(), "\n"), nil
}
