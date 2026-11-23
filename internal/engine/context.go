package engine

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// AssembleContext 按固定顺序装配上下文（T210）：system → HB.md → env → history → input。
func AssembleContext(system, hb, env, history, input string) string {
	parts := []string{system}
	if hb != "" {
		parts = append(parts, "## 项目约定（HB.md / AGENTS.md）\n"+hb)
	}
	if env != "" {
		parts = append(parts, "## 环境\n"+env)
	}
	if history != "" {
		parts = append(parts, "## 相关历史\n"+history)
	}
	parts = append(parts, "## 本轮任务\n"+input)
	return strings.Join(parts, "\n\n")
}

// LoadProjectContext 读取工作区的 HB.md / AGENTS.md，超限时截断并标注。
func LoadProjectContext(workdir string, limit int) string {
	if limit <= 0 {
		limit = 16384
	}
	for _, name := range []string{"HB.md", "AGENTS.md"} {
		data, err := os.ReadFile(filepath.Join(workdir, name))
		if err != nil {
			continue
		}
		s := string(data)
		if len(s) > limit {
			s = s[:limit] + "\n… 已截断（超过 HB.md 上限）"
		}
		return fmt.Sprintf("[%s]\n%s", name, s)
	}
	return ""
}

// RepoMapContext 生成仓库符号地图（Aider 同类做法），控制在预算内。
func RepoMapContext(workdir string, maxTokens int) string {
	m := BuildRepoMap(workdir, maxTokens)
	if m == nil || len(m.Symbols) == 0 {
		return ""
	}
	return "## 仓库符号地图（按被引用次数排序）\n" + m.Render()
}

// EnvContext 生成环境信息，包含 git status / diff --stat（对标 Aider 的 git 注入）。
func EnvContext(workdir string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "OS: %s/%s\n", runtime.GOOS, runtime.GOARCH)
	fmt.Fprintf(&b, "工作区: %s\n", workdir)
	fmt.Fprintf(&b, "日期: %s\n", time.Now().Format("2006-01-02"))
	if shell := os.Getenv("SHELL"); shell != "" {
		fmt.Fprintf(&b, "Shell: %s\n", shell)
	}
	if info := gitInfo(workdir); info != "" {
		b.WriteString(info)
	}
	return b.String()
}

func gitInfo(workdir string) string {
	if _, err := os.Stat(filepath.Join(workdir, ".git")); err != nil {
		return ""
	}
	var out strings.Builder
	for _, args := range [][]string{
		{"status", "--short"},
		{"diff", "--stat"},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = workdir
		data, err := cmd.Output()
		if err != nil {
			continue
		}
		s := strings.TrimSpace(string(data))
		if s == "" {
			continue
		}
		lines := strings.Split(s, "\n")
		if len(lines) > 40 {
			s = strings.Join(lines[:40], "\n") + "\n… 截断"
		}
		fmt.Fprintf(&out, "git %s:\n%s\n", strings.Join(args, " "), s)
	}
	return out.String()
}
