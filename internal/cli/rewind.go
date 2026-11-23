package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/MC5lan/HonestBoy-CLI/internal/engine"
	"github.com/MC5lan/HonestBoy-CLI/internal/store"
)

// RunRewind 实现 hb rewind list / restore <checkpoint-id>。
func RunRewind(opts GlobalOptions, args []string) error {
	if len(args) == 0 {
		return ErrUsage
	}
	sessionID := ""
	rest := args
	if args[0] == "--session" {
		if len(args) < 2 {
			return ErrUsage
		}
		sessionID, rest = args[1], args[2:]
	}
	if sessionID == "" {
		metas, err := store.NewSessionStore(store.DefaultPaths()).List()
		if err != nil || len(metas) == 0 {
			return fmt.Errorf("%w: 没有会话", ErrEnvironment)
		}
		sessionID = metas[0].ID
	}
	dir := filepath.Join(store.DefaultPaths().Sessions, sessionID)
	switch rest[0] {
	case "list":
		cps, err := engine.ListCheckpoints(dir)
		if err != nil {
			return fmt.Errorf("%w: %v", ErrEnvironment, err)
		}
		if len(cps) == 0 {
			fmt.Fprintln(os.Stdout, "暂无检查点")
			return nil
		}
		fmt.Fprintf(os.Stdout, "%-28s %-20s %s\n", "ID", "时间", "任务")
		for _, cp := range cps {
			prompt := []rune(cp.Prompt)
			if len(prompt) > 40 {
				prompt = prompt[:40]
			}
			fmt.Fprintf(os.Stdout, "%-28s %-20s %s\n", cp.ID, cp.CreatedAt.Format("2006-01-02 15:04"), string(prompt))
		}
		return nil
	case "restore":
		if len(rest) < 2 {
			return ErrUsage
		}
		cps, err := engine.ListCheckpoints(dir)
		if err != nil {
			return fmt.Errorf("%w: %v", ErrEnvironment, err)
		}
		var target *engine.Checkpoint
		for i := range cps {
			if cps[i].ID == rest[1] {
				target = &cps[i]
				break
			}
		}
		if target == nil {
			return fmt.Errorf("%w: 检查点不存在", ErrUsage)
		}
		workdir := opts.Cwd
		if workdir == "" {
			wd, err := os.Getwd()
			if err != nil {
				return ErrEnvironment
			}
			workdir = wd
		}
		n, err := engine.RestoreCheckpoint(workdir, target)
		if err != nil {
			return fmt.Errorf("%w: %v", ErrEnvironment, err)
		}
		fmt.Fprintf(os.Stdout, "已恢复 %d 个文件到检查点 %s（对话历史保留）\n", n, target.ID)
		return nil
	default:
		return ErrUsage
	}
}
