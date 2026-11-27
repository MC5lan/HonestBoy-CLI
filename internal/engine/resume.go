package engine

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/MC5lan/HonestBoy-CLI/internal/core"
	"github.com/MC5lan/HonestBoy-CLI/internal/llm"
)

// MaxResumeAttempts 是同一会话允许的续跑次数上限；超过后要求人工检查。
const MaxResumeAttempts = 2

// resumeOutcome 把终态与中断原因映射成"是否可续跑"。
// 只有崩溃/超时/失败/网络中断可续；用户主动中断、untested、denied、done 都不可续。
func resumeOutcome(terminal core.Terminal, err error, ctx context.Context) (bool, string) {
	switch terminal {
	case core.TermDone, core.TermUntest, core.TermDenied:
		return false, ""
	case core.TermHalt:
		switch {
		case errors.Is(ctx.Err(), context.DeadlineExceeded):
			return true, "timeout"
		case errors.Is(ctx.Err(), context.Canceled):
			return false, "user_abort"
		default:
			return false, ""
		}
	case core.TermFailed:
		if errors.Is(err, context.DeadlineExceeded) {
			return true, "timeout"
		}
		var re *llm.RetryableError
		if errors.As(err, &re) {
			// Status=0 是连接失败/EOF，按网络中断处理。
			if re.Status == 0 {
				return true, "network"
			}
			if !re.Retryable {
				// 401/403/400 这类永久错误，重试没有意义。
				return false, "permanent"
			}
		}
		return true, "failed"
	default:
		return false, ""
	}
}

// BuildResumePrompt 根据会话事件生成"从断点继续"的提示。
// 它只陈述已观察到的事实：原始任务、已完成/失败的工具、上次已输出文本。
func BuildResumePrompt(s *core.Session, reason string) string {
	if s == nil {
		return "继续上次未完成的任务。"
	}

	original := firstTaskInput(s)
	if strings.TrimSpace(original) == "" {
		original = lastTaskInput(s)
	}

	completed, failed, unknown := toolSummaries(s, 20)
	assistant := lastAssistantText(s)

	var b strings.Builder
	fmt.Fprintf(&b, "继续上次未完成的任务（原因：%s）。\n\n", resumeReasonText(reason))
	fmt.Fprintf(&b, "原始任务：\n%s\n\n", truncateRunes(strings.TrimSpace(original), 2000))

	b.WriteString("已完成（不要重复执行）：\n")
	if len(completed) == 0 {
		b.WriteString("（无）\n")
	} else {
		for _, line := range completed {
			b.WriteString("- " + line + "\n")
		}
	}
	b.WriteString("\n未完成或失败：\n")
	if len(failed) == 0 {
		b.WriteString("（无）\n")
	} else {
		for _, line := range failed {
			b.WriteString("- " + line + "\n")
		}
	}
	b.WriteString("\n中断时仍在执行（状态未知）：\n")
	if len(unknown) == 0 {
		b.WriteString("（无）\n")
	} else {
		for _, line := range unknown {
			b.WriteString("- " + line + "\n")
		}
	}

	b.WriteString("\n上次已输出（可能不完整）：\n")
	if strings.TrimSpace(assistant) == "" {
		b.WriteString("（无）\n")
	} else {
		b.WriteString(truncateRunes(assistant, 2000) + "\n")
	}

	b.WriteString(`
要求：
1. 先检查当前工作区的真实状态，不要假设未执行的工具已经成功。
2. 对"状态未知"的工具，先检查工作区确认是否已生效，不要假设成功或失败。
3. 不要重复执行上面"已完成"的工具调用；如确需重做，先说明原因。
4. 继续未完成的部分；无法完成时调用 declare_untested 并列出未完成项。
5. 完成时仍按正常流程提交可接地证据，不要复用上次回合的证据。`)
	return b.String()
}

// ResumeReasonText 把中断原因码转成中文，供 CLI/TUI 展示。
func ResumeReasonText(reason string) string { return resumeReasonText(reason) }

func resumeReasonText(reason string) string {
	switch reason {
	case "crash":
		return "进程异常退出"
	case "timeout":
		return "运行超时"
	case "network":
		return "网络中断"
	case "failed":
		return "上一回合失败"
	case "user_abort":
		return "用户中断"
	default:
		return "上次未完成"
	}
}

// firstTaskInput 找最早的一条非续跑用户输入，作为"原始任务"。
func firstTaskInput(s *core.Session) string {
	for _, ev := range s.Events {
		if ev.Type != core.EvtTurnStart {
			continue
		}
		if d, ok := ev.Data.(core.TurnStartData); ok && !d.Resumed && strings.TrimSpace(d.Input) != "" {
			return d.Input
		}
	}
	return ""
}

func lastTaskInput(s *core.Session) string {
	for i := len(s.Events) - 1; i >= 0; i-- {
		if s.Events[i].Type != core.EvtTurnStart {
			continue
		}
		if d, ok := s.Events[i].Data.(core.TurnStartData); ok && strings.TrimSpace(d.Input) != "" {
			return d.Input
		}
	}
	return ""
}

// toolSummaries 返回已完成/失败/状态未知的工具描述，各取最近 limit 条。
// 状态未知 = 有 tool_request 但没有对应 tool_end（进程在工具执行中被杀）。
func toolSummaries(s *core.Session, limit int) (completed, failed, unknown []string) {
	req := make(map[string]core.ToolRequestData)
	var order []string
	ended := make(map[string]bool)
	type end struct {
		callID string
		ok     bool
		out    string
	}
	var ends []end
	for _, ev := range s.Events {
		switch ev.Type {
		case core.EvtToolRequest:
			if d, ok := ev.Data.(core.ToolRequestData); ok {
				if _, seen := req[d.CallID]; !seen {
					order = append(order, d.CallID)
				}
				req[d.CallID] = d
			}
		case core.EvtToolEnd:
			if d, ok := ev.Data.(core.ToolEndData); ok {
				ended[d.CallID] = true
				ends = append(ends, end{callID: d.CallID, ok: d.OK, out: d.Output})
			}
		}
	}
	if len(ends) > limit {
		ends = ends[len(ends)-limit:]
	}
	for _, e := range ends {
		r := req[e.callID]
		status := "失败"
		if e.ok {
			status = "成功"
		}
		line := toolLine(r, status)
		if out := oneLine(e.out, 120); out != "" {
			line += " — " + out
		}
		if e.ok {
			completed = append(completed, line)
		} else {
			failed = append(failed, line)
		}
	}
	for _, id := range order {
		if ended[id] {
			continue
		}
		unknown = append(unknown, toolLine(req[id], "状态未知"))
	}
	if len(unknown) > limit {
		unknown = unknown[len(unknown)-limit:]
	}
	return completed, failed, unknown
}

func toolLine(r core.ToolRequestData, status string) string {
	name := r.Tool
	if name == "" {
		name = "tool"
	}
	args := truncateRunes(strings.ReplaceAll(r.Args, "\n", " "), 80)
	if strings.TrimSpace(args) == "" {
		return name + "：" + status
	}
	return fmt.Sprintf("%s(%s)：%s", name, args, status)
}

func lastAssistantText(s *core.Session) string {
	start := -1
	for i, ev := range s.Events {
		if ev.Type == core.EvtTurnStart {
			start = i
		}
	}
	if start < 0 {
		return ""
	}
	var b strings.Builder
	for _, ev := range s.Events[start+1:] {
		if ev.Type == core.EvtTextDelta {
			if d, ok := ev.Data.(core.TextDeltaData); ok {
				b.WriteString(d.Text)
			}
		}
	}
	return strings.TrimSpace(core.VisibleText(b.String()))
}

func oneLine(s string, n int) string {
	s = strings.Join(strings.Fields(s), " ")
	return truncateRunes(s, n)
}
