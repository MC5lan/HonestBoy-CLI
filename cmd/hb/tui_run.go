package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/MC5lan/HonestBoy-CLI/internal/cli"
	"github.com/MC5lan/HonestBoy-CLI/internal/core"
	"github.com/MC5lan/HonestBoy-CLI/internal/engine"
	"github.com/MC5lan/HonestBoy-CLI/internal/git"
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
	sessionStore := store.NewSessionStore(store.DefaultPaths())
	var replay []core.Event
	if resumeID != "" {
		sess, err := sessionStore.Load(resumeID)
		if err != nil {
			return fmt.Errorf("%w: %v", cli.ErrEnvironment, err)
		}
		replay = sess.Events
		// 从其他目录恢复时以会话记录的工作区为准，避免沙箱与历史错位。
		if sess.Cwd != "" {
			if st, statErr := os.Stat(sess.Cwd); statErr == nil && st.IsDir() {
				wd = sess.Cwd
			}
		}
	}
	sb, err := tools.Open(wd)
	if err != nil {
		return fmt.Errorf("%w: %v", cli.ErrEnvironment, err)
	}
	defer sb.Close()
	registry := tools.NewDefaultRegistry(sb, tools.NewReadGuard(), cfg.Shell)
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
	keyEnv := cfg.APIKeyEnv
	if keyEnv == "" {
		keyEnv = "HB_API_KEY"
	}

	run := func(prompt, sessionID string) (<-chan core.Event, <-chan error) {
		ctx, c := context.WithCancel(context.Background())
		cancel = c
		eng := engine.New(engine.Config{
			Workdir: wd, Model: cfg.Model, Protocol: cfg.Protocol, BaseURL: cfg.BaseURL,
			APIKey: os.Getenv(keyEnv), Registry: registry, Store: sessionStore,
			ResumeID:        sessionID,
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
			MaxOutputTokens: cfg.MaxOutputTokens,
			Tracer:          core.NewTracer(filepath.Join(store.DefaultPaths().Root, "trace")),
		})
		return eng.Run(ctx, prompt)
	}

	setModel := func(name string) error {
		cfg.Model = name
		return store.Save(cfg)
	}

	exec := func(cmd string, args []string, sessionID string) (string, error) {
		switch cmd {
		case "/verify":
			return cli.VerifyReport()
		case "/stats":
			return cli.StatsReport()
		case "/diff":
			repo, err := git.Open(wd)
			if err != nil {
				return "", err
			}
			d, err := repo.Diff()
			if err != nil {
				return "", err
			}
			if strings.TrimSpace(d) == "" {
				return "工作区无未提交改动", nil
			}
			return d, nil
		case "/undo":
			repo, err := git.Open(wd)
			if err != nil {
				return "", err
			}
			sha, err := repo.UndoLast()
			if err != nil {
				return "", err
			}
			return "已撤销自动提交 " + sha + "（文件改动保留在工作区）", nil
		case "/rewind":
			return rewindReport(sessionID, args, wd)
		}
		return "", fmt.Errorf("未知命令 %s", cmd)
	}

	listSessions := func() ([]tui.SessionInfo, error) {
		metas, err := sessionStore.List()
		if err != nil {
			return nil, err
		}
		out := make([]tui.SessionInfo, 0, 20)
		for _, m := range metas {
			// TUI 的工具沙箱绑定当前工作区；列出其他项目的会话会造成误导。
			if m.Cwd != "" && !samePath(m.Cwd, wd) {
				continue
			}
			out = append(out, tui.SessionInfo{
				ID: m.ID, Summary: m.InputSummary, Terminal: m.Terminal,
				Modified: m.ModifiedAt.Format("2006-01-02 15:04"),
			})
			if len(out) >= 20 {
				break
			}
		}
		return out, nil
	}

	resumeSession := func(id string) ([]core.Event, error) {
		sess, err := sessionStore.Load(id)
		if err != nil {
			return nil, err
		}
		return sess.Events, nil
	}

	approvePlan := func(id string) (string, error) {
		sess, err := sessionStore.Load(id)
		if err != nil {
			return "", err
		}
		if sess.Cwd != "" && !samePath(sess.Cwd, wd) {
			return "", fmt.Errorf("该计划属于其他工作区: %s", sess.Cwd)
		}
		for i := len(sess.Receipts) - 1; i >= 0; i-- {
			if p, ok := engine.PlanApproved(sess.Receipts[i]); ok {
				return p, nil
			}
		}
		return "", nil
	}

	historyPath := filepath.Join(store.DefaultPaths().Root, "tui_history.json")
	history := loadHistory(historyPath)
	persistHistory := func(items []string) { saveHistory(historyPath, items) }

	onboard := cfg.BaseURL == "" || os.Getenv(keyEnv) == ""
	m := tui.NewModel(tui.Config{
		Workdir: wd, Model: cfg.Model, Theme: cfg.Theme, Replay: replay,
		History: history, SessionID: resumeID,
		ContextWindow: contextWindow, Protocol: cfg.Protocol, BaseURL: cfg.BaseURL,
		SetModel: setModel, Exec: exec, Onboard: onboard,
		ListSessions: listSessions, ResumeSession: resumeSession, ApprovePlan: approvePlan,
		SaveHistory: persistHistory,
		Run:         run,
		Cancel: func() {
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

func rewindReport(sessionID string, args []string, wd string) (string, error) {
	paths := store.DefaultPaths()
	if sessionID == "" {
		metas, err := store.NewSessionStore(paths).List()
		if err != nil || len(metas) == 0 {
			return "", fmt.Errorf("没有会话")
		}
		for _, m := range metas {
			if m.Cwd == "" || samePath(m.Cwd, wd) {
				sessionID = m.ID
				break
			}
		}
		if sessionID == "" {
			return "", fmt.Errorf("当前工作区没有会话")
		}
	}
	dir := filepath.Join(paths.Sessions, sessionID)
	cps, err := engine.ListCheckpoints(dir)
	if err != nil {
		return "", err
	}
	if len(args) >= 2 && args[0] == "restore" {
		for i := range cps {
			if cps[i].ID == args[1] {
				n, err := engine.RestoreCheckpoint(workdirOf(sessionID), &cps[i])
				if err != nil {
					return "", err
				}
				return fmt.Sprintf("已恢复 %d 个文件到检查点 %s（对话历史保留）", n, cps[i].ID), nil
			}
		}
		return "", fmt.Errorf("检查点不存在: %s", args[1])
	}
	if len(cps) == 0 {
		return "暂无检查点", nil
	}
	var b strings.Builder
	fmt.Fprintf(&b, "会话 %s 的检查点：\n", sessionID)
	for _, cp := range cps {
		prompt := []rune(cp.Prompt)
		if len(prompt) > 40 {
			prompt = prompt[:40]
		}
		fmt.Fprintf(&b, "%-28s %-16s %s\n", cp.ID, cp.CreatedAt.Format("2006-01-02 15:04"), string(prompt))
	}
	b.WriteString("\n恢复：/rewind restore <id>")
	return strings.TrimRight(b.String(), "\n"), nil
}

// workdirOf 读取会话记录里的 cwd；读不到时退回当前目录。
func workdirOf(sessionID string) string {
	sess, err := store.NewSessionStore(store.DefaultPaths()).Load(sessionID)
	if err == nil && sess.Cwd != "" {
		return sess.Cwd
	}
	wd, _ := os.Getwd()
	return wd
}

// samePath 在 Windows 上按大小写不敏感比较工作区路径。
func samePath(a, b string) bool {
	return strings.EqualFold(filepath.Clean(a), filepath.Clean(b))
}

func saveHistory(path string, items []string) {
	_ = os.MkdirAll(filepath.Dir(path), 0o700)
	if raw, err := json.Marshal(items); err == nil {
		_ = os.WriteFile(path, raw, 0o600)
	}
}

func loadHistory(path string) []string {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var out []string
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil
	}
	if len(out) > 200 {
		out = out[len(out)-200:]
	}
	return out
}

func exitOf(err error) int {
	if err == nil {
		return 0
	}
	return cli.ExitCodeOf(err)
}
