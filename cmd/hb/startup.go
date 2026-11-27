package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/MC5lan/HonestBoy-CLI/internal/engine"
	"github.com/MC5lan/HonestBoy-CLI/internal/store"
)

// resumeStartupSession 在进入 TUI 前提示最近一条未完成会话。
// 只有“当前工作区最近一条会话就是未完成会话”时才提示，避免旧的崩溃会话反复打扰。
func resumeStartupSession(ss *store.SessionStore, wd string, resumeID *string) {
	resumeStartupSessionIO(ss, wd, resumeID, os.Stdin, os.Stdout)
}

func resumeStartupSessionIO(ss *store.SessionStore, wd string, resumeID *string, in io.Reader, out io.Writer) {
	meta, ok := ss.LatestForWorkdir(wd)
	if !ok || !meta.Resumable {
		return
	}
	reader := bufio.NewReader(in)
	for {
		summary := meta.InputSummary
		if strings.TrimSpace(summary) == "" {
			summary = "(无摘要)"
		}
		fmt.Fprintf(out, "\n上次任务在 %s 中断（%s）：%s\n",
			meta.ModifiedAt.Format("01-02 15:04"), engine.ResumeReasonText(meta.ResumeReason), summary)
		fmt.Fprint(out, "[Enter] 继续  [n] 新建  [s] 查看会话: ")
		line, err := reader.ReadString('\n')
		if err != nil && line == "" {
			// stdin 已关闭：按新建处理，不能卡住启动。
			return
		}
		switch strings.ToLower(strings.TrimSpace(line)) {
		case "", "y", "yes":
			*resumeID = meta.ID
			return
		case "n", "no":
			return
		case "s":
			listRecentSessions(ss, wd, out)
		default:
			fmt.Fprintln(out, "请输入 Enter / n / s")
		}
	}
}

func listRecentSessions(ss *store.SessionStore, wd string, out io.Writer) {
	metas, err := ss.List()
	if err != nil {
		fmt.Fprintln(out, "读取会话失败:", err)
		return
	}
	fmt.Fprintln(out, "最近会话：")
	n := 0
	for _, m := range metas {
		if m.Cwd != "" && !samePath(m.Cwd, wd) {
			continue
		}
		term := terminalLabel(m)
		summary := m.InputSummary
		if strings.TrimSpace(summary) == "" {
			summary = "(无摘要)"
		}
		fmt.Fprintf(out, "  %-22s %-14s %-16s %s\n",
			m.ID, m.ModifiedAt.Format("01-02 15:04"), term, summary)
		n++
		if n >= 10 {
			break
		}
	}
	fmt.Fprintln(out, "用 hb resume <id> 恢复指定会话")
}

func terminalLabel(m store.SessionMeta) string {
	switch {
	case m.Resumable && m.ResumeReason == "crash":
		return "未正常退出"
	case m.Resumable:
		return "可恢复:" + m.ResumeReason
	case m.Terminal == "":
		return "中断"
	}
	return m.Terminal
}
