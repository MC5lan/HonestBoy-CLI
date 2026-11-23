package cli

import (
	"fmt"
	"os"

	"github.com/MC5lan/HonestBoy-CLI/internal/git"
)

func gitRepo(opts GlobalOptions) (*git.Repo, error) {
	wd := opts.Cwd
	if wd == "" {
		var err error
		wd, err = os.Getwd()
		if err != nil {
			return nil, ErrEnvironment
		}
	}
	repo, err := git.Open(wd)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrEnvironment, err)
	}
	return repo, nil
}

// RunUndo 撤销最近一条 Honest Boy 自动提交（保留文件改动）。
func RunUndo(opts GlobalOptions, args []string) error {
	repo, err := gitRepo(opts)
	if err != nil {
		return err
	}
	sha, err := repo.UndoLast()
	if err != nil {
		return fmt.Errorf("%w: %v", ErrUsage, err)
	}
	fmt.Fprintf(os.Stdout, "已撤销自动提交 %s（文件改动保留在工作区）\n", sha)
	return nil
}

// RunDiff 显示当前工作区的 diff 审阅信息。
func RunDiff(opts GlobalOptions, args []string) error {
	repo, err := gitRepo(opts)
	if err != nil {
		return err
	}
	diff, err := repo.Diff()
	if err != nil {
		return fmt.Errorf("%w: %v", ErrEnvironment, err)
	}
	if diff == "" {
		fmt.Fprintln(os.Stdout, "工作区无未提交改动")
		return nil
	}
	fmt.Fprint(os.Stdout, diff)
	return nil
}
