package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/MC5lan/HonestBoy-CLI/internal/cli"
	"github.com/MC5lan/HonestBoy-CLI/internal/core"
	"github.com/MC5lan/HonestBoy-CLI/internal/engine"
	"github.com/MC5lan/HonestBoy-CLI/internal/store"
	"github.com/MC5lan/HonestBoy-CLI/internal/tools"
	"github.com/MC5lan/HonestBoy-CLI/internal/tui"
	tea "github.com/charmbracelet/bubbletea"
)

// runTUI 由 cmd 层装配 engine/tools/store，再交给 tui（保持 tui 的依赖方向）。
func runTUI(resumeID string) error {
	cfg, err := store.Load()
	if err != nil {
		return fmt.Errorf("%w: %v", cli.ErrEnvironment, err)
	}
	wd, err := cli.ResolveWorkdir("", cfg.Workdir)
	if err != nil {
		return err
	}
	contextWindow := cfg.Context.Window
	if contextWindow <= 0 {
		contextWindow = 960000
	}
	sb, err := tools.Open(wd)
	if err != nil {
		return fmt.Errorf("%w: %v", cli.ErrEnvironment, err)
	}
	defer sb.Close()
	registry := tools.NewDefaultRegistry(sb, tools.NewReadGuard(), cfg.Shell)
	sessionStore := store.NewSessionStore(store.DefaultPaths())
	approvalCh := make(chan tui.ApprovalAsk)
	approvalHandler := func(ctx context.Context, req core.ApprovalRequest) (core.ApprovalDecision, error) {
		resp := make(chan core.ApprovalDecision, 1)
		select {
		case approvalCh <- tui.ApprovalAsk{Req: req, Resp: resp}:
		case <-ctx.Done():
			return core.ApprovalDecision{}, ctx.Err()
		}
		select {
		case d := <-resp:
			return d, nil
		case <-ctx.Done():
			return core.ApprovalDecision{}, ctx.Err()
		}
	}
	var cancel context.CancelFunc

	run := func(prompt string) (<-chan core.Event, <-chan error) {
		ctx, c := context.WithCancel(context.Background())
		cancel = c
		eng := engine.New(engine.Config{
			Workdir: wd, Model: cfg.Model, Protocol: cfg.Protocol, BaseURL: cfg.BaseURL,
			APIKey: os.Getenv(cfg.APIKeyEnv), Registry: registry, Store: sessionStore,
			Approval:        engine.ApprovalPolicy{Write: cfg.Approval.Write, Exec: cfg.Approval.Exec, Net: cfg.Approval.Net},
			ApprovalHandler: approvalHandler,
			MaxTurns:        cfg.MaxTurns,
			SelfAssessment:  true,
			AmbiguityHalt:   true,
			LedgerRecall:    true,
			PlanFirst:       true,
			ReasoningEffort: cfg.ReasoningEffort,
			ContextWindow:   contextWindow,
			Rigor:           cfg.Rigor,
			Tracer:          core.NewTracer(filepath.Join(store.DefaultPaths().Root, "trace")),
		})
		return eng.Run(ctx, prompt)
	}

	var replay []core.Event
	if resumeID != "" {
		sess, err := sessionStore.Load(resumeID)
		if err != nil {
			return fmt.Errorf("%w: %v", cli.ErrEnvironment, err)
		}
		replay = sess.Events
	}

	setModel := func(name string) error {
		cfg.Model = name
		return store.Save(cfg)
	}
	exec := func(cmd string, args []string) (string, error) {
		switch cmd {
		case "/verify":
			return cli.VerifyReport()
		case "/stats":
			return cli.StatsReport()
		case "/sessions":
			return cli.SessionsReport(10)
		default:
			return "", nil
		}
	}
	keyEnv := cfg.APIKeyEnv
	if keyEnv == "" {
		keyEnv = "HB_API_KEY"
	}
	onboard := cfg.BaseURL == "" || os.Getenv(keyEnv) == ""
	m := tui.NewModel(tui.Config{
		Workdir: wd, Model: cfg.Model, Theme: cfg.Theme, Replay: replay,
		ContextWindow: contextWindow, Protocol: cfg.Protocol, BaseURL: cfg.BaseURL,
		SetModel: setModel, Exec: exec, Onboard: onboard,
		Run: run, Cancel: func() {
			if cancel != nil {
				cancel()
			}
		},
		Approvals: approvalCh,
	})
	program := tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseCellMotion())
	_, err = program.Run()
	return err
}

func exitOf(err error) int {
	if err == nil {
		return 0
	}
	return cli.ExitCodeOf(err)
}
