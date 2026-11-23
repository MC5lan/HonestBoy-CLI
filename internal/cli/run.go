package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/MC5lan/HonestBoy-CLI/internal/core"
	"github.com/MC5lan/HonestBoy-CLI/internal/engine"
	"github.com/MC5lan/HonestBoy-CLI/internal/logging"
	"github.com/MC5lan/HonestBoy-CLI/internal/store"
	"github.com/MC5lan/HonestBoy-CLI/internal/tools"
)

// RunRun 实现 hb run：接引擎、消费事件流、按终态退出。
func RunRun(opts GlobalOptions, args []string) error {
	_ = logging.Init(opts.Debug, "")
	defer logging.Close()
	logging.L().Info("hb run", "model", opts.Model, "protocol", opts.Protocol)

	workdir := opts.Cwd
	if workdir == "" {
		wd, err := os.Getwd()
		if err != nil {
			return ErrEnvironment
		}
		workdir = wd
	}
	st, err := os.Stat(workdir)
	if err != nil || !st.IsDir() {
		return ErrUsage
	}

	prompt, err := readPrompt(args)
	if err != nil {
		return err
	}
	if prompt == "" {
		return ErrUsage
	}

	cfg, err := store.Load()
	if err != nil {
		return fmt.Errorf("%w: %v", ErrEnvironment, err)
	}
	baseURL := firstNonEmpty(opts.BaseURL, cfg.BaseURL)
	protocol := firstNonEmpty(opts.Protocol, cfg.Protocol)
	apiKey := os.Getenv(firstNonEmpty(cfg.APIKeyEnv, "HB_API_KEY"))
	contextWindow := cfg.Context.Window
	if contextWindow <= 0 {
		contextWindow = 960000
	}
	if baseURL == "" && apiKey == "" {
		return fmt.Errorf("%w: 尚未配置模型端点或密钥。先运行 hb init 完成设置，或设置环境变量 %s / 传 --base-url",
			ErrEnvironment, firstNonEmpty(cfg.APIKeyEnv, "HB_API_KEY"))
	}

	sb, err := tools.Open(workdir)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrEnvironment, err)
	}
	defer sb.Close()
	guard := tools.NewReadGuard()
	registry := tools.NewDefaultRegistry(sb, guard, cfg.Shell)

	approval := engine.ApprovalPolicy{
		Write: firstNonEmpty(cfg.Approval.Write, "auto"),
		Exec:  cfg.Approval.Exec,
		Net:   cfg.Approval.Net,
	}
	if opts.Yes {
		approval.Write = "auto"
	}
	eng := engine.New(engine.Config{
		Workdir: workdir, Model: firstNonEmpty(opts.Model, cfg.Model),
		Protocol: protocol, BaseURL: baseURL, APIKey: apiKey,
		Registry: registry, Approval: approval, MaxTurns: opts.MaxTurns,
		PlanMode: opts.Plan, AutoCommit: opts.AutoCommit,
		ReasoningEffort: cfg.ReasoningEffort,
		ContextWindow:   contextWindow,
		SelfAssessment:  true, AmbiguityHalt: true, LedgerRecall: true, PlanFirst: true,
		Tracer: core.NewTracer(filepath.Join(store.DefaultPaths().Root, "trace")),
	})
	ctx, cancel := context.WithTimeout(context.Background(), parseTimeout(opts.Timeout, cfg.Timeout))
	defer cancel()
	SetupSignal(ctx, cancel)

	events, errs := eng.Run(ctx, prompt)
	terminal := core.TermFailed
	seenTurnEnd := false
	for ev := range events {
		if opts.JSON {
			if err := WriteJSONEvent(os.Stdout, ev); err != nil {
				return err
			}
		} else if !opts.Quiet {
			if err := WriteHumanEvent(os.Stdout, ev); err != nil {
				return err
			}
		}
		if ev.Type == core.EvtTurnEnd {
			if d, ok := ev.Data.(core.TurnEndData); ok {
				terminal, seenTurnEnd = core.Terminal(d.Terminal), true
			}
		}
	}
	if err := <-errs; err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return ErrInterrupted
		}
		fmt.Fprintln(os.Stderr, "hb:", err)
		return fmt.Errorf("%w: %v", ErrEnvironment, err)
	}
	if !seenTurnEnd {
		return ErrEnvironment
	}
	if terminal == core.TermDone {
		return nil
	}
	return &TerminalError{Terminal: terminal}
}

// readPrompt 拼合命令行参数与管道输入。
func readPrompt(args []string) (string, error) {
	var parts []string
	if len(args) > 0 {
		parts = append(parts, strings.Join(args, " "))
	}
	info, err := os.Stdin.Stat()
	if err == nil && (info.Mode()&os.ModeCharDevice) == 0 {
		data, rerr := io.ReadAll(io.LimitReader(os.Stdin, 4<<20))
		if rerr != nil {
			return "", ErrEnvironment
		}
		if len(data) > 0 {
			if len(parts) > 0 {
				parts = append(parts, "--- stdin ---")
			}
			parts = append(parts, string(data))
		}
	}
	return strings.Join(parts, "\n\n"), nil
}

func parseTimeout(flagValue, cfgValue string) time.Duration {
	for _, s := range []string{flagValue, cfgValue, "10m"} {
		if s != "" {
			if d, err := time.ParseDuration(s); err == nil {
				return d
			}
		}
	}
	return 10 * time.Minute
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}
