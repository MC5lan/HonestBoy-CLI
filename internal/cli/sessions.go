package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/MC5lan/HonestBoy-CLI/internal/core"
	"github.com/MC5lan/HonestBoy-CLI/internal/store"
)

// RunSessions 实现 hb sessions / show / rm。
func RunSessions(opts GlobalOptions, args []string) error {
	ss := store.NewSessionStore(store.DefaultPaths())
	if len(args) > 0 {
		switch args[0] {
		case "diag":
			if len(args) < 2 {
				return ErrUsage
			}
			sess, err := ss.Load(args[1])
			if err != nil {
				return fmt.Errorf("%w: %v", ErrEnvironment, err)
			}
			denied, gateDenies := 0, map[string]int{}
			for _, ev := range sess.Events {
				if ev.Type == core.EvtGateDeny {
					if d, ok := ev.Data.(core.GateDenyData); ok {
						gateDenies[d.Rule]++
					}
				}
			}
			for _, r := range sess.Receipts {
				if r.Terminal == core.TermDenied {
					denied++
				}
			}
			fmt.Fprintf(os.Stdout, "会话 %s：%d 回合，%d 次 denied，%d 条 Receipt\n", args[1], len(sess.Turns), denied, len(sess.Receipts))
			for rule, n := range gateDenies {
				fmt.Fprintf(os.Stdout, "  Gate 拒绝 %s: %d 次\n", rule, n)
			}
			return nil
		case "show":
			if len(args) < 2 {
				return ErrUsage
			}
			sess, err := ss.Load(args[1])
			if err != nil {
				return fmt.Errorf("%w: %v", ErrEnvironment, err)
			}
			tail := 50
			if len(args) >= 4 && args[2] == "--tail" {
				fmt.Sscanf(args[3], "%d", &tail)
			}
			start := 0
			if len(sess.Events) > tail {
				start = len(sess.Events) - tail
			}
			for _, ev := range sess.Events[start:] {
				WriteJSONEvent(os.Stdout, ev)
			}
			return nil
		case "rm":
			if len(args) < 2 || (!opts.Yes && len(args) < 3) {
				return ErrUsage
			}
			path := store.DefaultPaths().Sessions + string(os.PathSeparator) + args[1] + ".jsonl"
			if err := os.Remove(path); err != nil {
				return fmt.Errorf("%w: %v", ErrEnvironment, err)
			}
			fmt.Printf("已删除会话 %s\n", args[1])
			return nil
		}
	}
	metas, err := ss.List()
	if err != nil {
		return fmt.Errorf("%w: %v", ErrEnvironment, err)
	}
	limit := 20
	for i := 1; i < len(args); i += 2 {
		if args[i-1] == "--limit" {
			fmt.Sscanf(args[i], "%d", &limit)
		}
	}
	if len(metas) > limit {
		metas = metas[:limit]
	}
	fmt.Fprintf(os.Stdout, "%-22s %-20s %-14s %-10s %s\n", "ID", "时间", "终态", "cwd", "首条输入")
	for _, m := range metas {
		term := displayTerminal(m.Terminal)
		switch {
		case m.Resumable && m.ResumeReason == "crash":
			term = "(未正常退出)"
		case m.Resumable:
			term = "(可恢复:" + m.ResumeReason + ")"
		case m.Terminal == "" && !m.Ended:
			term = "(未正常退出)"
		}
		fmt.Fprintf(os.Stdout, "%-22s %-20s %-14s %-10s %s\n",
			m.ID, m.ModifiedAt.Format("2006-01-02 15:04"), term,
			shortPath(m.Cwd), oneLine(m.InputSummary, 40))
	}
	return nil
}

func displayTerminal(t string) string {
	if t == "" {
		return "(中断)"
	}
	return t
}

func shortPath(p string) string {
	parts := strings.Split(strings.ReplaceAll(p, "\\", "/"), "/")
	if len(parts) > 2 {
		return strings.Join(parts[len(parts)-2:], "/")
	}
	return p
}
