package engine

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/MC5lan/HonestBoy-CLI/internal/core"
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

// historyText 把已发生的事件压成一段可读对话历史，供下一回合继续。
// 只保留用户输入、助手文本、工具结果与终态；总长度按 rune 截断。
func historyText(s *core.Session, maxRunes int) string {
	if s == nil || len(s.Events) == 0 {
		return ""
	}
	var b strings.Builder
	var assistant strings.Builder
	flush := func() {
		text := strings.TrimSpace(assistant.String())
		assistant.Reset()
		if text != "" {
			fmt.Fprintf(&b, "助手: %s\n", truncateRunes(text, 2000))
		}
	}
	for _, ev := range s.Events {
		switch ev.Type {
		case core.EvtTurnStart:
			flush()
			if d, ok := ev.Data.(core.TurnStartData); ok && d.Input != "" {
				fmt.Fprintf(&b, "用户: %s\n", truncateRunes(d.Input, 300))
			}
		case core.EvtTextDelta:
			if d, ok := ev.Data.(core.TextDeltaData); ok {
				assistant.WriteString(d.Text)
			}
		case core.EvtToolEnd:
			flush()
			if d, ok := ev.Data.(core.ToolEndData); ok {
				status := "成功"
				if !d.OK {
					status = "失败"
				}
				fmt.Fprintf(&b, "工具(%s): %s\n", status,
					truncateRunes(strings.ReplaceAll(d.Output, "\n", " "), 160))
			}
		case core.EvtGateDeny:
			if d, ok := ev.Data.(core.GateDenyData); ok {
				fmt.Fprintf(&b, "Gate 拒绝: %s\n", truncateRunes(d.Reason, 200))
			}
		case core.EvtTurnEnd:
			flush()
			if d, ok := ev.Data.(core.TurnEndData); ok {
				fmt.Fprintf(&b, "结果: %s\n", d.Terminal)
			}
		}
	}
	flush()
	text := strings.TrimSpace(b.String())
	if r := []rune(text); len(r) > maxRunes {
		text = "…（较早历史已省略）\n" + string(r[len(r)-maxRunes:])
	}
	return text
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
