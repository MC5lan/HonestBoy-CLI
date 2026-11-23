package cli

import (
	"context"
	"fmt"
	"os"

	"github.com/MC5lan/HonestBoy-CLI/internal/core"
	"github.com/MC5lan/HonestBoy-CLI/internal/engine"
	"github.com/MC5lan/HonestBoy-CLI/internal/store"
	"github.com/MC5lan/HonestBoy-CLI/internal/tools"
)

// RunApprove 读取计划模式的 Receipt（terminal=halt），以其 Claim 为任务正式执行。
func RunApprove(opts GlobalOptions, args []string) error {
	if len(args) < 1 {
		return ErrUsage
	}
	sessionID := args[0]
	ss := store.NewSessionStore(store.DefaultPaths())
	sess, err := ss.Load(sessionID)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrEnvironment, err)
	}
	plan := ""
	for i := len(sess.Receipts) - 1; i >= 0; i-- {
		if p, ok := engine.PlanApproved(sess.Receipts[i]); ok {
			plan = p
			break
		}
	}
	if plan == "" {
		return fmt.Errorf("%w: 会话 %s 没有待批准的计划", ErrUsage, sessionID)
	}
	fmt.Fprintf(os.Stdout, "批准计划，开始执行：\n%s\n\n", truncatePlan(plan, 400))

	workdir := opts.Cwd
	if workdir == "" {
		workdir = sess.Cwd
	}
	st, err := os.Stat(workdir)
	if err != nil || !st.IsDir() {
		return ErrUsage
	}
	sb, err := tools.Open(workdir)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrEnvironment, err)
	}
	defer sb.Close()

	cfg, _ := store.Load()
	eng := engine.New(engine.Config{
		Workdir: workdir, Model: firstNonEmpty(opts.Model, cfg.Model),
		Protocol: cfg.Protocol, BaseURL: cfg.BaseURL,
		APIKey:   os.Getenv(cfg.APIKeyEnv),
		Registry: tools.NewDefaultRegistry(sb, tools.NewReadGuard(), cfg.Shell),
		MaxTurns: opts.MaxTurns,
	})
	ctx, cancel := context.WithTimeout(context.Background(), parseTimeout(opts.Timeout, cfg.Timeout))
	defer cancel()
	events, errs := eng.Run(ctx, plan)
	terminal := core.TermFailed
	for ev := range events {
		if opts.JSON {
			_ = WriteJSONEvent(os.Stdout, ev)
		} else {
			_ = WriteHumanEvent(os.Stdout, ev)
		}
		if ev.Type == core.EvtTurnEnd {
			if d, ok := ev.Data.(core.TurnEndData); ok {
				terminal = core.Terminal(d.Terminal)
			}
		}
	}
	if err := <-errs; err != nil {
		fmt.Fprintln(os.Stderr, "hb:", err)
		return fmt.Errorf("%w: %v", ErrEnvironment, err)
	}
	if terminal == core.TermDone {
		return nil
	}
	return &TerminalError{Terminal: terminal}
}

func truncatePlan(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}
