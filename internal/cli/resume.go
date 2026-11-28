package cli

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/MC5lan/HonestBoy-CLI/internal/engine"
	"github.com/MC5lan/HonestBoy-CLI/internal/logging"
	"github.com/MC5lan/HonestBoy-CLI/internal/store"
)

// RunResumeLast 实现 hb resume --last：续跑当前工作区最近一条未完成会话。
func RunResumeLast(opts GlobalOptions, args []string) error {
	_ = logging.Init(opts.Debug, "")
	defer func() { _ = logging.Close() }()

	cfg, err := store.Load()
	if err != nil {
		return fmt.Errorf("%w: %v", ErrEnvironment, err)
	}
	workdir, err := ResolveWorkdir(opts.Cwd, cfg.Workdir)
	if err != nil {
		return err
	}
	return resumeLatest(opts, cfg, workdir, strings.TrimSpace(strings.Join(args, " ")))
}

// RunResumeID 实现 hb resume <id> --headless：在脚本/CI 中续跑指定会话。
func RunResumeID(opts GlobalOptions, id string, args []string) error {
	_ = logging.Init(opts.Debug, "")
	defer func() { _ = logging.Close() }()

	cfg, err := store.Load()
	if err != nil {
		return fmt.Errorf("%w: %v", ErrEnvironment, err)
	}
	ss := store.NewSessionStore(store.DefaultPaths())
	metas, err := ss.List()
	if err != nil {
		return fmt.Errorf("%w: %v", ErrEnvironment, err)
	}
	var meta store.SessionMeta
	found := false
	for _, m := range metas {
		if m.ID == id {
			meta, found = m, true
			break
		}
	}
	if !found {
		return fmt.Errorf("%w: 会话 %s 不存在；用 hb sessions 查看", ErrEnvironment, id)
	}
	if !meta.Resumable {
		return fmt.Errorf("%w: 会话 %s 不是未完成状态；用 hb resume %s 打开 TUI，或 hb run 新任务",
			ErrEnvironment, id, id)
	}
	workdir := meta.Cwd
	if workdir == "" {
		workdir, err = ResolveWorkdir(opts.Cwd, cfg.Workdir)
		if err != nil {
			return err
		}
	} else if st, statErr := os.Stat(workdir); statErr != nil || !st.IsDir() {
		return fmt.Errorf("%w: 会话记录的工作区不存在: %s", ErrEnvironment, workdir)
	}
	return resumeSession(opts, cfg, meta, workdir, strings.TrimSpace(strings.Join(args, " ")))
}

// resumeLatest 是 hb resume --last 与 hb run --auto-resume 的共同实现。
func resumeLatest(opts GlobalOptions, cfg store.Config, workdir, extra string) error {
	ss := store.NewSessionStore(store.DefaultPaths())
	meta, err := ss.LatestResumable(workdir)
	if err != nil {
		if errors.Is(err, store.ErrNoResumable) {
			return fmt.Errorf("%w: 当前工作区没有可恢复的会话。用 hb sessions 查看；要开新任务请给出任务内容", ErrEnvironment)
		}
		return fmt.Errorf("%w: %v", ErrEnvironment, err)
	}
	return resumeSession(opts, cfg, meta, workdir, extra)
}

func resumeSession(opts GlobalOptions, cfg store.Config, meta store.SessionMeta, workdir, extra string) error {
	if meta.ResumeAttempts >= engine.MaxResumeAttempts {
		return fmt.Errorf("%w: 会话 %s 已续跑 %d 次仍未完成；请先人工检查（hb sessions show %s）",
			ErrEnvironment, meta.ID, meta.ResumeAttempts, meta.ID)
	}
	ss := store.NewSessionStore(store.DefaultPaths())
	sess, err := ss.Load(meta.ID)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrEnvironment, err)
	}
	prompt := engine.BuildResumePrompt(sess, meta.ResumeReason)
	if extra != "" {
		prompt += "\n\n补充要求：\n" + extra
	}
	fmt.Fprintf(os.Stderr, "续跑会话 %s（原因：%s，第 %d 次）\n",
		meta.ID, meta.ResumeReason, meta.ResumeAttempts+1)
	return runOnce(opts, cfg, workdir, prompt, meta.ID, meta.ResumeAttempts+1)
}
