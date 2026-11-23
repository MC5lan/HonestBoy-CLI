package cli

import (
	"fmt"
	"io"
	"os"

	"github.com/MC5lan/HonestBoy-CLI/internal/core"
	"github.com/MC5lan/HonestBoy-CLI/internal/store"
)

// RunVerify 逐条重算 Receipt 哈希链并报告全部断点。
func RunVerify(opts GlobalOptions, args []string) error {
	ss := store.NewSessionStore(store.DefaultPaths())
	var sessions []string
	if len(args) >= 2 && args[0] == "--session" {
		sessions = []string{args[1]}
	} else {
		metas, err := ss.List()
		if err != nil {
			return fmt.Errorf("%w: %v", ErrEnvironment, err)
		}
		for _, m := range metas {
			sessions = append(sessions, m.ID)
		}
	}
	broken := false
	for _, id := range sessions {
		sess, err := ss.Load(id)
		if err != nil {
			fmt.Fprintf(os.Stderr, "会话 %s: 读取失败 %v\n", id, err)
			broken = true
			continue
		}
		at, err := core.VerifyChain(sess.Receipts)
		if err != nil {
			fmt.Printf("会话 %s: 第 %d 条断裂（期望 prev=%s）\n",
				id, at+1, prevHashOf(sess.Receipts, at))
			broken = true
			continue
		}
		fmt.Printf("会话 %s: %d 条记录，链完整\n", id, len(sess.Receipts))
	}
	if len(sessions) == 0 {
		_, _ = io.WriteString(os.Stdout, "暂无会话记录\n")
	}
	if broken {
		return ErrChainBroken
	}
	return nil
}

func prevHashOf(rs []core.Receipt, at int) string {
	if at < 0 || at >= len(rs) {
		return "-"
	}
	if at == 0 {
		return "0...0"
	}
	return rs[at-1].Hash[:minInt(12, len(rs[at-1].Hash))] + "…"
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
