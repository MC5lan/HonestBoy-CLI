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
	"github.com/MC5lan/HonestBoy-CLI/internal/git"
	"github.com/MC5lan/HonestBoy-CLI/internal/logging"
	"github.com/MC5lan/HonestBoy-CLI/internal/store"
	"github.com/MC5lan/HonestBoy-CLI/internal/tools"
)

// RunRun 实现 hb run：接引擎、消费事件流、按终态退出。
func RunRun(opts GlobalOptions, args []string) error {
	_ = logging.Init(opts.Debug, "")
	defer logging.Close()
	logging.L().Info("hb run", "model", opts.Model, "protocol", opts.Protocol)

	cfg, err := store.Load()
	if err != nil {
		return fmt.Errorf("%w: %v", ErrEnvironment, err)
	}
	workdir, err := ResolveWorkdir(opts.Cwd, cfg.Workdir)
	if err != nil {
		return err
	}

	prompt, err := readPrompt(args)
	if err != nil {
		return err
	}
	if prompt == "" {
		return ErrUsage
	}
	if opts.Diff {
		prompt, err = withGitDiff(prompt, workdir)
		if err != nil {
			return err
		}
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
	if err := tools.RegisterCustomTools(registry, cfg.Tools.CustomDir, workdir); err != nil {
		return fmt.Errorf("%w: 自定义工具加载失败: %v", ErrEnvironment, err)
	}
	if err := tools.RegisterWebFetch(registry, cfg.Web.Enabled, cfg.Web.AllowedDomains); err != nil {
		return fmt.Errorf("%w: %v", ErrEnvironment, err)
	}

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
		Rigor:           cfg.Rigor,
		MaxOutputTokens: cfg.MaxOutputTokens,
		SelfAssessment:  true, AmbiguityHalt: true, LedgerRecall: true, PlanFirst: true,
		Tracer: core.NewTracer(filepath.Join(store.DefaultPaths().Root, "trace")),
	})
	ctx, cancel := context.WithTimeout(context.Background(), parseTimeout(opts.Timeout, cfg.Timeout))
	defer cancel()
	SetupSignal(ctx, cancel)

	events, errs := eng.Run(ctx, prompt)
	terminal := core.TermFailed
	seenTurnEnd := false
	var profile core.ProfileData
	for ev := range events {
		if ev.Type == core.EvtProfile {
			if d, ok := ev.Data.(core.ProfileData); ok {
				profile = d
			}
		}
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
	if opts.Profile && !opts.JSON {
		fmt.Fprintf(os.Stdout, "\nprofile  总 %dms · 上下文 %dms · 流 %dms · 首 token %dms · 工具 %dms（%d 次）\n",
			profile.TotalMS, profile.ContextMS, profile.StreamMS, profile.FirstTokenMS, profile.ToolMS, profile.ToolCalls)
	}
	if terminal == core.TermDone {
		return nil
	}
	return &TerminalError{Terminal: terminal}
}

// readPrompt 拼合命令行参数与管道输入。
// withGitDiff 把工作区 diff 追加到任务上下文；空 diff 也明确说明。
func withGitDiff(prompt, workdir string) (string, error) {
	repo, err := git.Open(workdir)
	if err != nil {
		return "", fmt.Errorf("%w: --diff 需要 git 工作区: %v", ErrEnvironment, err)
	}
	diff, err := repo.Diff()
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrEnvironment, err)
	}
	if strings.TrimSpace(diff) == "" {
		return prompt + "\n\n## git diff\n（工作区无未提交改动）", nil
	}
	return prompt + "\n\n## git diff\n```diff\n" + diff + "\n```", nil
}

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
