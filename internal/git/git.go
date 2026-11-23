// Package git 提供 Aider 风格的工作区版本管理：diff 审阅、自动提交与撤销。
// 只依赖标准库 + os/exec，不依赖 tools（避免循环）。
package git

import (
	"bytes"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

var (
	ErrNotRepo         = errors.New("not a git repository")
	ErrNothingToCommit = errors.New("nothing to commit")
)

// CommitAuthor 是自动提交使用的作者（Aider 用 "(aider)" 后缀区分）。
const CommitAuthor = "Honest Boy <honestboy@local>"

type Repo struct {
	Workdir string
}

func Open(workdir string) (*Repo, error) {
	cmd := exec.Command("git", "rev-parse", "--is-inside-work-tree")
	cmd.Dir = workdir
	out, err := cmd.Output()
	if err != nil || strings.TrimSpace(string(out)) != "true" {
		return nil, ErrNotRepo
	}
	return &Repo{Workdir: workdir}, nil
}

func (r *Repo) run(args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = r.Workdir
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}
	return stdout.String(), nil
}

// IsDirty 判断工作区是否有未提交变更（含未跟踪文件）。
func (r *Repo) IsDirty() (bool, error) {
	out, err := r.run("status", "--porcelain")
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(out) != "", nil
}

// Diff 返回工作区 diff（已跟踪文件）。
func (r *Repo) Diff() (string, error) {
	out, err := r.run("diff", "--no-color")
	if err != nil {
		return "", err
	}
	return out, nil
}

// DiffStat 返回变更统计。
func (r *Repo) DiffStat() (string, error) {
	out, err := r.run("diff", "--stat")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

// ChangedFiles 返回已修改与未跟踪的文件列表。
func (r *Repo) ChangedFiles() ([]string, error) {
	out, err := r.run("status", "--porcelain")
	if err != nil {
		return nil, err
	}
	var files []string
	for _, line := range strings.Split(out, "\n") {
		if len(line) < 4 {
			continue
		}
		path := strings.TrimSpace(line[3:])
		if idx := strings.Index(path, " -> "); idx >= 0 {
			path = path[idx+4:]
		}
		if path != "" {
			files = append(files, path)
		}
	}
	return files, nil
}

// Commit 把当前所有变更提交为一个自动提交，返回短 SHA。
// message 由调用方给出；author 固定为 Honest Boy 以便区分。
func (r *Repo) Commit(message string) (string, error) {
	if message == "" {
		message = fmt.Sprintf("hb: auto commit %s", time.Now().Format("2006-01-02 15:04"))
	}
	dirty, err := r.IsDirty()
	if err != nil {
		return "", err
	}
	if !dirty {
		return "", ErrNothingToCommit
	}
	if _, err := r.run("add", "-A"); err != nil {
		return "", err
	}
	// 用 -c user.name/-c user.email 覆盖，不改仓库配置。
	if _, err := r.run("-c", "user.name=Honest Boy", "-c", "user.email=honestboy@local",
		"commit", "-m", message); err != nil {
		return "", err
	}
	out, err := r.run("rev-parse", "--short", "HEAD")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

// LastAutoCommit 返回最近一条 Honest Boy 提交的 SHA；没有则返回空。
func (r *Repo) LastAutoCommit() (string, error) {
	out, err := r.run("log", "-1", "--format=%H%x00%an%x00%s")
	if err != nil {
		return "", err
	}
	fields := strings.Split(strings.TrimSpace(out), "\x00")
	if len(fields) < 2 || !strings.Contains(fields[1], "Honest Boy") {
		return "", nil
	}
	short, err := r.run("rev-parse", "--short", fields[0])
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(short), nil
}

// UndoLast 撤销最近一条自动提交，保留文件改动（mixed reset）。
func (r *Repo) UndoLast() (string, error) {
	sha, err := r.LastAutoCommit()
	if err != nil || sha == "" {
		return "", ErrNothingToCommit
	}
	if _, err := r.run("reset", "--mixed", "HEAD~1"); err != nil {
		return "", err
	}
	return sha, nil
}

// Log 返回最近 n 条提交摘要。
func (r *Repo) Log(n int) (string, error) {
	out, err := r.run("log", fmt.Sprintf("-%d", n), "--format=%h %an: %s")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}
