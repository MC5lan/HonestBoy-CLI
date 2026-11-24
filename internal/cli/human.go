package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"regexp"
	"strings"

	"github.com/MC5lan/HonestBoy-CLI/internal/core"
	"github.com/MC5lan/HonestBoy-CLI/internal/errs"
)

var jsonErrorRe = regexp.MustCompile(`\{"error"\s*:.*`)

// WriteHumanEvent 输出人类可读事件：技术术语翻译成中文，原始 JSON 错误转成可执行建议。
func WriteHumanEvent(w io.Writer, e core.Event) error {
	switch e.Type {
	case core.EvtTurnStart:
		if d, ok := e.Data.(core.TurnStartData); ok {
			input := []rune(d.Input)
			if len(input) > 60 {
				input = input[:60]
			}
			return writeLine(w, "▶ 开始任务: %s", string(input))
		}
	case core.EvtTextDelta:
		if d, ok := e.Data.(core.TextDeltaData); ok {
			_, err := io.WriteString(w, d.Text)
			return err
		}
	case core.EvtToolRequest:
		if d, ok := e.Data.(core.ToolRequestData); ok {
			return writeLine(w, "  · 使用工具 %s", friendlyTool(d.Tool))
		}
	case core.EvtToolStart:
		// 不单独输出，避免刷屏。
	case core.EvtToolEnd:
		if d, ok := e.Data.(core.ToolEndData); ok {
			mark := "✓"
			if !d.OK {
				mark = "✗"
			}
			out := friendlyOutput(d.Output)
			return writeLine(w, "    %s %s  %s", mark, oneLine(out, 70), formatMS(d.DurationMS))
		}
	case core.EvtToolDenied:
		if d, ok := e.Data.(core.ToolDeniedData); ok {
			return writeLine(w, "    ⊘ 已跳过（需批准）: %s", d.Reason)
		}
	case core.EvtGateAllow:
		return writeLine(w, "✓ 检查通过，任务完成")
	case core.EvtGateDeny:
		if d, ok := e.Data.(core.GateDenyData); ok {
			return writeLine(w, "✗ 还不能算完成: %s", friendlyGate(d.Rule, d.Reason))
		}
	case core.EvtAbstain:
		if d, ok := e.Data.(core.AbstainData); ok {
			return writeLine(w, "⏸ 已弃权: %s（%d 项未验证）", d.Reason, len(d.NotDone))
		}
	case core.EvtNotice:
		if d, ok := e.Data.(core.NoticeData); ok {
			return writeLine(w, "  ℹ %s", d.Text)
		}
	case core.EvtError:
		if d, ok := e.Data.(core.ErrorData); ok {
			return writeLine(w, "✗ 出错: %s", friendlyError(d.Message))
		}
	case core.EvtTurnEnd:
		if d, ok := e.Data.(core.TurnEndData); ok {
			terminalCN := map[string]string{
				"done": "完成", "untested": "未验证", "denied": "未完成", "halt": "已暂停", "failed": "失败",
			}[d.Terminal]
			if terminalCN == "" {
				terminalCN = d.Terminal
			}
			return writeLine(w, "\n■ 结果: %s · 用时 %s · 输入 %d / 输出 %d tokens",
				terminalCN, formatMS(d.DurationMS), d.TokensIn, d.TokensOut)
		}
	}
	return nil
}

func writeLine(w io.Writer, format string, args ...any) error {
	_, err := fmt.Fprintf(w, format+"\n", args...)
	return err
}

func formatMS(ms int64) string {
	if ms < 1000 {
		return fmt.Sprintf("%d毫秒", ms)
	}
	return fmt.Sprintf("%.1f秒", float64(ms)/1000)
}

// friendlyTool 把内部工具名翻译成中文动作。
func friendlyTool(name string) string {
	names := map[string]string{
		"read_file": "读取文件", "write_file": "写入文件", "edit_file": "编辑文件",
		"list_dir": "查看目录", "search": "搜索", "shell": "执行命令",
		"verify_build": "验证构建", "verify_test": "运行测试", "verify_vet": "静态检查",
		"apply_patch": "应用补丁", "request_done": "申请完成", "declare_untested": "声明未验证",
	}
	if cn, ok := names[name]; ok {
		return cn
	}
	return name
}

// friendlyOutput 把验证器原始输出压成一句人话，绝不改写结论。
func friendlyOutput(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	if lines := strings.Split(s, "\n"); len(lines) > 0 && strings.TrimSpace(lines[0]) != "" {
		s = strings.TrimSpace(lines[0])
	}
	if len([]rune(s)) > 70 {
		s = string([]rune(s)[:70]) + "…"
	}
	return s
}

// friendlyGate 把 Gate 规则翻译成中文。
func friendlyGate(rule, reason string) string {
	switch rule {
	case "coverage":
		return "还有改动没验证：" + reason
	case "evidence_failed":
		return "验证没通过：" + reason
	case "provenance":
		return "证据来源不够可靠：" + reason
	case "evidence_kind":
		return "缺少需要的验证类型：" + reason
	default:
		return reason
	}
}

// friendlyError 把模型 API 返回的原始 JSON 错误转成中文可执行建议。
func friendlyError(msg string) string {
	// 错误可能带 "环境错误: " 等前缀，先定位到 JSON 起点。
	if i := strings.Index(msg, `{"error"`); i >= 0 {
		msg = msg[i:]
	}
	if jsonErrorRe.MatchString(msg) {
		var parsed struct {
			Error struct {
				Message string `json:"message"`
				Type    string `json:"type"`
			} `json:"error"`
		}
		if err := json.Unmarshal([]byte(msg), &parsed); err == nil {
			inner := parsed.Error.Message
			lower := strings.ToLower(inner)
			switch {
			case strings.Contains(lower, "authentication") || strings.Contains(lower, "invalid_api_key") || strings.Contains(lower, "unauthorized"):
				return "密钥不对或没生效。请检查：1) 是否运行了 hb init；2) api_key_env 是不是环境变量名（如 HB_API_KEY，不是 sk- 开头的值）；3) 当前窗口是否执行了 $env:HB_API_KEY='你的密钥'"
			case strings.Contains(lower, "rate limit") || strings.Contains(lower, "429"):
				return "请求太频繁，稍等几秒再试。"
			case strings.Contains(lower, "connection") || strings.Contains(lower, "timeout") || strings.Contains(lower, "network"):
				return "连不上模型服务。检查 base_url 是否正确、网络是否通。"
			default:
				return inner
			}
		}
	}
	// Go/Windows 的传输层错误不带 JSON，也要给同样的可执行建议。
	lower := strings.ToLower(msg)
	if strings.Contains(lower, "dial tcp") || strings.Contains(lower, "connection refused") ||
		strings.Contains(lower, "no such host") || strings.Contains(lower, "connectex") {
		if advice, ok := errs.Explain(string(errs.NetUnavailable)); ok {
			return advice
		}
	}
	return msg
}

func oneLine(s string, n int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.TrimSpace(s)
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}
