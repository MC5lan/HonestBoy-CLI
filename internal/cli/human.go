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

// HumanWriter 在整条事件流上过滤 [SELF]/[TEST PLAN] 控制块，只展示正文，
// 并在回合结束时汇总“改动 / 验证 / 未验证 / 下一步”。
type HumanWriter struct {
	w         io.Writer
	filter    core.VisibleTextFilter
	control   map[string]bool // request_done / declare_untested 的 CallID
	reqs      map[string]core.ToolRequestData
	sessionID string
	changes   []string
	changeSet map[string]bool
	verify    []string
	verifySet map[string]bool
	untested  []string
	missing   []string
	denyWhy   string
	lastError string
}

func NewHumanWriter(w io.Writer) *HumanWriter {
	h := &HumanWriter{w: w}
	h.reset()
	return h
}

func (h *HumanWriter) reset() {
	h.control = make(map[string]bool)
	h.reqs = make(map[string]core.ToolRequestData)
	h.changes = nil
	h.changeSet = make(map[string]bool)
	h.verify = nil
	h.verifySet = make(map[string]bool)
	h.untested = nil
	h.missing = nil
	h.denyWhy = ""
	h.lastError = ""
}

// WriteEvent 输出一条事件；文本增量先经过可见性过滤。
func (h *HumanWriter) WriteEvent(e core.Event) error {
	if e.SessionID != "" {
		h.sessionID = e.SessionID
	}
	switch e.Type {
	case core.EvtTurnStart:
		h.reset()
		h.filter.Reset()
	case core.EvtTextDelta:
		d, ok := e.Data.(core.TextDeltaData)
		if !ok {
			return nil
		}
		vis := h.filter.Write(d.Text)
		if vis == "" {
			return nil
		}
		_, err := io.WriteString(h.w, vis)
		return err
	case core.EvtToolRequest:
		d, ok := e.Data.(core.ToolRequestData)
		if !ok {
			break
		}
		h.reqs[d.CallID] = d
		if core.IsControlTool(d.Tool) {
			h.control[d.CallID] = true
			return nil
		}
	case core.EvtToolEnd:
		d, ok := e.Data.(core.ToolEndData)
		if !ok {
			break
		}
		req, known := h.reqs[d.CallID]
		delete(h.reqs, d.CallID)
		if h.control[d.CallID] {
			delete(h.control, d.CallID)
			return nil
		}
		if known {
			h.noteToolResult(req, d)
		}
	case core.EvtDiffReview:
		if d, ok := e.Data.(core.DiffReviewData); ok {
			for _, f := range d.Files {
				h.addChange(f)
			}
		}
	case core.EvtGateDeny:
		if d, ok := e.Data.(core.GateDenyData); ok {
			h.missing = d.Missing
			h.denyWhy = d.Reason
		}
	case core.EvtAbstain:
		if d, ok := e.Data.(core.AbstainData); ok {
			h.untested = append([]string{}, d.NotDone...)
			if len(h.untested) == 0 && d.Reason != "" {
				h.untested = []string{d.Reason}
			}
		}
	case core.EvtError:
		if d, ok := e.Data.(core.ErrorData); ok {
			h.lastError = d.Message
		}
	case core.EvtTurnEnd:
		if vis := h.filter.Flush(); vis != "" {
			if _, err := io.WriteString(h.w, vis); err != nil {
				return err
			}
		}
		d, ok := e.Data.(core.TurnEndData)
		if !ok {
			return writeHumanEvent(h.w, e)
		}
		if err := h.writeReceipt(d); err != nil {
			return err
		}
		h.reset()
		return nil
	}
	return writeHumanEvent(h.w, e)
}

func (h *HumanWriter) noteToolResult(req core.ToolRequestData, end core.ToolEndData) {
	if !end.OK {
		if label := verifyLabel(req.Tool); label != "" {
			h.addVerify(label + " ✗")
		}
		return
	}
	switch req.Tool {
	case "write_file", "edit_file":
		var a struct {
			Path string `json:"path"`
		}
		if json.Unmarshal([]byte(req.Args), &a) == nil && a.Path != "" {
			h.addChange(a.Path)
		}
	case "apply_patch":
		for _, p := range applyPatchPaths(req.Args) {
			h.addChange(p)
		}
	}
	if label := verifyLabel(req.Tool); label != "" {
		h.addVerify(label + " ✓")
	}
}

func (h *HumanWriter) addChange(path string) {
	path = strings.TrimSpace(path)
	if path == "" || h.changeSet[path] {
		return
	}
	h.changeSet[path] = true
	h.changes = append(h.changes, path)
}

func (h *HumanWriter) addVerify(item string) {
	if h.verifySet[item] {
		return
	}
	h.verifySet[item] = true
	h.verify = append(h.verify, item)
}

func (h *HumanWriter) writeReceipt(d core.TurnEndData) error {
	terminalCN := map[string]string{
		"done": "完成", "untested": "未验证", "denied": "未完成", "halt": "已暂停", "failed": "失败",
	}[d.Terminal]
	if terminalCN == "" {
		terminalCN = d.Terminal
	}
	if err := writeLine(h.w, "\n■ %s · 用时 %s · 输入 %d / 输出 %d tokens",
		terminalCN, formatMS(d.DurationMS), d.TokensIn, d.TokensOut); err != nil {
		return err
	}
	if strings.TrimSpace(d.Summary) != "" {
		if err := writeLine(h.w, "  摘要: %s", oneLine(d.Summary, 180)); err != nil {
			return err
		}
	}
	if len(h.changes) > 0 {
		if err := writeLine(h.w, "  改动: %s", summarizeList(h.changes, 6)); err != nil {
			return err
		}
	} else if err := writeLine(h.w, "  改动: 无"); err != nil {
		return err
	}
	if len(h.verify) > 0 {
		if err := writeLine(h.w, "  验证: %s", strings.Join(h.verify, " · ")); err != nil {
			return err
		}
	} else if err := writeLine(h.w, "  验证: —"); err != nil {
		return err
	}
	if unverified := h.unverifiedText(d); unverified != "" {
		if err := writeLine(h.w, "  未验证: %s", unverified); err != nil {
			return err
		}
	} else if err := writeLine(h.w, "  未验证: —"); err != nil {
		return err
	}
	if next := h.nextSteps(d); len(next) > 0 {
		if err := writeLine(h.w, "  下一步: %s", strings.Join(next, " · ")); err != nil {
			return err
		}
	}
	return nil
}

func (h *HumanWriter) unverifiedText(d core.TurnEndData) string {
	if d.Terminal == string(core.TermDone) {
		return ""
	}
	if len(h.untested) > 0 {
		return summarizeList(h.untested, 4)
	}
	if len(h.missing) > 0 {
		return "缺少: " + summarizeList(h.missing, 4)
	}
	if strings.TrimSpace(h.denyWhy) != "" {
		return oneLine(h.denyWhy, 120)
	}
	if strings.TrimSpace(h.lastError) != "" {
		return oneLine(friendlyError(h.lastError), 120)
	}
	if d.Terminal == string(core.TermUntest) {
		return "未运行验证或未申请完成"
	}
	return ""
}

func (h *HumanWriter) nextSteps(d core.TurnEndData) []string {
	var out []string
	seen := map[string]bool{}
	add := func(s string) {
		if s != "" && !seen[s] && len(out) < 3 {
			seen[s] = true
			out = append(out, s)
		}
	}
	if d.Resumable {
		add("hb resume --last")
	}
	switch d.Terminal {
	case string(core.TermDone):
		add("hb verify")
		if len(h.changes) > 0 {
			add("hb diff")
		}
	default:
		if h.sessionID != "" {
			add("hb sessions show " + h.sessionID)
		}
	}
	return out
}

func verifyLabel(tool string) string {
	switch tool {
	case "verify_build":
		return "build"
	case "verify_test":
		return "test"
	case "verify_vet":
		return "vet"
	}
	return ""
}

func applyPatchPaths(args string) []string {
	var a struct {
		Patch string `json:"patch"`
	}
	if json.Unmarshal([]byte(args), &a) != nil {
		return nil
	}
	var out []string
	for _, line := range strings.Split(a.Patch, "\n") {
		t := strings.TrimSpace(line)
		for _, prefix := range []string{"*** Add File: ", "*** Update File: ", "*** Delete File: "} {
			if strings.HasPrefix(t, prefix) {
				out = append(out, strings.TrimSpace(strings.TrimPrefix(t, prefix)))
			}
		}
	}
	return out
}

func summarizeList(items []string, max int) string {
	if len(items) <= max {
		return strings.Join(items, ", ")
	}
	return strings.Join(items[:max], ", ") + fmt.Sprintf(" 等 %d 个", len(items))
}

// WriteHumanEvent 输出单条人类可读事件（不做跨事件文本过滤）。
func WriteHumanEvent(w io.Writer, e core.Event) error {
	return writeHumanEvent(w, e)
}

func writeHumanEvent(w io.Writer, e core.Event) error {
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
			if core.IsControlTool(d.Tool) {
				return nil
			}
			// 前面可能刚输出过流式正文，先换行再显示工具行。
			return writeLine(w, "\n  · 使用工具 %s", friendlyTool(d.Tool))
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
		if d, ok := e.Data.(core.GateAllowData); ok && strings.TrimSpace(d.Claim) != "" {
			return writeLine(w, "\n✓ 完成：%s", oneLine(d.Claim, 200))
		}
		return writeLine(w, "\n✓ 检查通过，任务完成")
	case core.EvtGateDeny:
		if d, ok := e.Data.(core.GateDenyData); ok {
			return writeLine(w, "\n✗ 还不能算完成: %s", friendlyGate(d.Rule, d.Reason))
		}
	case core.EvtAbstain:
		if d, ok := e.Data.(core.AbstainData); ok {
			return writeLine(w, "\n⏸ 已弃权: %s（%d 项未验证）", d.Reason, len(d.NotDone))
		}
	case core.EvtNotice:
		if d, ok := e.Data.(core.NoticeData); ok {
			return writeLine(w, "\n  ℹ %s", d.Text)
		}
	case core.EvtError:
		if d, ok := e.Data.(core.ErrorData); ok {
			return writeLine(w, "\n✗ 出错: %s", friendlyError(d.Message))
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
	// 不带 JSON 的纯状态码也要给可执行建议。
	lower := strings.ToLower(msg)
	switch {
	case strings.Contains(lower, "401") || strings.Contains(lower, "unauthorized") || strings.Contains(lower, "authentication"):
		return statusAdvice(401)
	case strings.Contains(lower, "403") || strings.Contains(lower, "forbidden"):
		return statusAdvice(403)
	case strings.Contains(lower, "404") || strings.Contains(lower, "model not found"):
		return statusAdvice(404)
	case strings.Contains(lower, "429") || strings.Contains(lower, "rate limit"):
		return statusAdvice(429)
	}
	// Go/Windows 的传输层错误不带 JSON，也要给同样的可执行建议。
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
