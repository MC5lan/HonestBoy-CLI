package engine

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/MC5lan/HonestBoy-CLI/internal/core"
	"github.com/MC5lan/HonestBoy-CLI/internal/llm"
	"github.com/MC5lan/HonestBoy-CLI/internal/tools"
)

// runTurn 是单次回合主循环：模型流 → 工具执行 → Gate 判定 → 终态与 Receipt。
func (e *Engine) runTurn(ctx context.Context, input string) (core.Terminal, error) {
	s := e.session
	turn := &core.Turn{ID: "t_" + shortID(input+time.Now().UTC().Format(time.RFC3339Nano))}
	s.Turns = append(s.Turns, turn)
	if err := turn.Transition(core.StThinking); err != nil {
		return core.TermFailed, err
	}
	e.emit(core.EvtTurnStart, core.TurnStartData{Input: input, Model: e.cfg.Model})
	if cp, cerr := CreateCheckpoint(e.cfg.Workdir, filepath.Join(e.store.Paths.Sessions, s.ID), turn.ID, input); cerr == nil {
		e.emit(core.EvtNotice, core.NoticeData{Level: "info", Text: "检查点已创建 " + cp.ID})
	}

	repoMap := ""
	if e.cfg.RepoMapTokens >= 0 {
		repoMap = RepoMapContext(e.cfg.Workdir, e.cfg.RepoMapTokens)
	}
	sys := AssembleContext(systemPrompt,
		LoadProjectContext(e.cfg.Workdir, e.cfg.HBMDLimit),
		EnvContext(e.cfg.Workdir), repoMap, "")
	if e.cfg.PlanMode {
		sys = WithPlanRules(sys)
	}
	if e.cfg.SelfAssessment {
		sys = sys + "\n" + core.SelfPromptFragment
	}
	messages := []llm.Message{
		{Role: "system", Content: sys},
		{Role: "user", Content: input},
	}
	budget := Budget{Ratio: 0.7, KeepRecent: 2, WindowSize: e.cfg.ContextWindow}
	var changes []core.Change
	toolCalls := 0
	remediation := 0
	abstainRetries := 0
	terminal := core.Terminal("")

steps:
	for step := 0; step < e.cfg.MaxTurns; step++ {
		if err := ctx.Err(); err != nil {
			return core.TermHalt, err
		}

		trimmed, removed := budget.Trim(messages, budget.Limit())
		if removed > 0 {
			messages = trimmed
			compacted := true
			messages[0].Content = WithCompactMark(sys, compacted)
			e.emit(core.EvtNotice, core.NoticeData{Level: "info", Text: fmt.Sprintf("上下文已裁剪 %d 条历史（占用约 %d tokens）", removed, Usage(messages))})
		}
		specs := toolSpecs(e.registry)
		if e.cfg.PlanMode {
			specs = planToolSpecs(e.registry)
		}
		req := llm.Request{
			Model: e.cfg.Model, Messages: messages, Tools: specs,
			MaxTokens: e.cfg.MaxTurns * 512,
		}
		chunks, errCh := e.cfg.ResolveClient().Stream(ctx, req)
		var text strings.Builder
		var calls []llm.ToolCall
		var stepUsage llm.Usage
		streamDone := false
		for !streamDone {
			select {
			case c, ok := <-chunks:
				if !ok {
					streamDone = true
					continue
				}
				if c.Text != "" {
					text.WriteString(c.Text)
					e.emit(core.EvtTextDelta, core.TextDeltaData{Text: c.Text})
				}
				if c.Call != nil {
					calls = append(calls, *c.Call)
					e.emit(core.EvtToolRequest, core.ToolRequestData{
						CallID: c.Call.ID, Tool: c.Call.Name, Args: c.Call.Args,
					})
				}
				if c.Usage != nil {
					stepUsage = *c.Usage
				}
			case err, ok := <-errCh:
				if !ok {
					continue
				}
				if err != nil {
					return core.TermFailed, err
				}
			case <-ctx.Done():
				return core.TermHalt, ctx.Err()
			}
		}
		if stepUsage.In > 0 || stepUsage.Out > 0 {
			e.usage.Add(stepUsage)
			e.emit(core.EvtUsage, core.UsageData{
				TokensIn: int64(e.usage.In), TokensOut: int64(e.usage.Out),
				CostMicros: e.usage.CostMicros,
			})
		}

		// 自评块：解析并让它影响控制流（文档 §4.1.3）。
		sa, selfOK := core.ParseSelf(text.String())
		if e.cfg.SelfAssessment && !selfOK {
			sa = core.SelfAssessment{Conf: "unknown"}
		}
		if e.cfg.Tracer != nil {
			e.cfg.Tracer.RecordClaim(text.String(), "thinking", false)
		}
		if e.cfg.SelfAssessment && sa.BlocksWrite() && hasWriteCall(calls) {
			reason := "自评置信度为 unknown 或假设缺少证伪条件，本轮不执行写操作"
			if sa.Conf == "" {
				reason = "未输出自评块，按 unknown 处理；本轮不执行写操作"
			}
			e.emit(core.EvtNotice, core.NoticeData{Level: "warn", Text: reason})
			if e.cfg.Tracer != nil {
				e.cfg.Tracer.RecordHalt(reason)
			}
			e.writeReceipt(core.TermHalt, text.String())
			_ = turn.Transition(core.StClosed)
			return core.TermHalt, nil
		}

		// 计划模式下模型输出纯文本即视为计划成稿：暂停等待用户批准。
		if len(calls) == 0 && e.cfg.PlanMode {
			plan := strings.TrimSpace(text.String())
			if plan != "" {
				e.emit(core.EvtNotice, core.NoticeData{Level: "info", Text: "计划已生成，等待批准（hb approve <session-id>）"})
				e.writeReceipt(core.TermHalt, plan)
				_ = turn.Transition(core.StClosed)
				return core.TermHalt, nil
			}
		}
		// 模型没有走工具申请完成：只回答不申请 → untested（T402）。
		if len(calls) == 0 {
			a := core.Abstain{
				Reason: "未提交完成申请", Done: []string{text.String()},
				NotDone: []string{"未声明完成依据", "未调用 request_done"},
			}
			e.emit(core.EvtAbstain, a.ToAbstainData())
			e.writeReceipt(core.TermUntest, text.String())
			return core.TermUntest, nil
		}

		var toolResults []llm.Message
		stop := false
		for _, call := range calls {
			toolCalls++
			if halt, reason := e.guard.Check(e.ledger, toolCalls); halt {
				e.emit(core.EvtNotice, core.NoticeData{Level: "error", Text: reason})
				return core.TermHalt, errors.New(reason)
			} else if reason != "" {
				e.emit(core.EvtNotice, core.NoticeData{Level: "warn", Text: reason})
			}

			var result string
			switch call.Name {
			case "request_done":
				var a struct {
					Summary string `json:"summary"`
				}
				_ = json.Unmarshal([]byte(call.Args), &a)
				if a.Summary == "" {
					a.Summary = text.String()
				}
				if turn.State == core.StToolRunning {
					_ = turn.Transition(core.StThinking)
				}
				if turn.State == core.StThinking {
					_ = turn.Transition(core.StJudging)
				}
				claim := ExtractClaim(a.Summary, changes).WithTargets(collectTargets(changes))
				// 本回合有任何工具失败时，禁止走“纯问答无需证据”的短路。
				// 失败证据已在 bag 中，Gate 会以 evidence_failed 拒绝。
				for _, ev := range e.hook.Bag().All() {
					if ev.Ran && !ev.OK {
						claim.Req.NoEvidence = false
						if len(claim.Req.Kinds) == 0 {
							claim.Req.Kinds = []core.EvidenceKind{ev.Kind}
						}
						if len(claim.Targets) == 0 {
							claim.Targets = []string{ev.Target}
						}
						claim.Req.MinCount = len(claim.Req.Kinds)
						break
					}
				}
				// T410：自我保证措辞命中时只警告，但 Gate 加严——判据仍是证据。
				if core.DetectSelfAssurance(text.String() + " " + a.Summary) {
					e.emit(core.EvtNotice, core.NoticeData{Level: "warn", Text: "检测到自我保证措辞，已加严证据要求"})
					claim.Req.NoEvidence = false
					for _, k := range []core.EvidenceKind{core.EvTest, core.EvBuild} {
						found := false
						for _, have := range claim.Req.Kinds {
							if have == k {
								found = true
								break
							}
						}
						if !found {
							claim.Req.Kinds = append(claim.Req.Kinds, k)
						}
					}
					claim.Req.MinCount = len(claim.Req.Kinds)
				}
				decision := e.gate.Evaluate(claim, *e.hook.Bag())
				if decision.Allow {
					if e.cfg.Tracer != nil {
						e.cfg.Tracer.RecordClaim(a.Summary, "verify", true)
					}
					e.emit(core.EvtGateAllow, core.GateAllowData{ClaimID: claim.ID, EvidenceIDs: decision.Evidence})
					e.writeReceipt(core.TermDone, a.Summary)
					_ = turn.Transition(core.StClosed)
					return core.TermDone, nil
				}
				if e.cfg.Tracer != nil {
					e.cfg.Tracer.RecordClaim(a.Summary, "verify", false)
				}
				e.emit(core.EvtGateDeny, core.GateDenyData{
					ClaimID: claim.ID, Missing: decision.Missing, Rule: decision.Rule, Reason: decision.Reason,
				})
				e.emit(core.EvtNotice, core.NoticeData{Level: "warn", Text: decision.Reason})
				if remediation < 2 {
					remediation++
					result = "完成申请被拒绝：" + decision.Reason + "。请补充验证。"
					break
				}
				terminal = core.TermDenied
				e.writeReceipt(core.TermDenied, a.Summary)
				_ = turn.Transition(core.StClosed)
				stop = true
			case "declare_untested":
				var a core.Abstain
				if err := json.Unmarshal([]byte(call.Args), &a); err != nil || a.Valid() != nil {
					if abstainRetries == 0 {
						abstainRetries++
						msg := "弃权声明不合法，请填 reason 与 not_done"
						e.emit(core.EvtNotice, core.NoticeData{Level: "warn", Text: msg})
						result = msg
						break
					}
				}
				e.emit(core.EvtAbstain, a.ToAbstainData())
				e.writeReceipt(core.TermUntest, a.Reason)
				_ = turn.Transition(core.StClosed)
				terminal = core.TermUntest
				stop = true
			default:
				tool, ok := e.registry.Get(call.Name)
				if !ok {
					result = "未知工具: " + call.Name
					e.emit(core.EvtToolEnd, core.ToolEndData{CallID: call.ID, OK: false, Output: result})
					toolResults = append(toolResults, toolResultMsg(call.ID, result))
					continue
				}
				if !e.approvedAll[call.Name] && e.approval.NeedApproval(tool, argString(call.Args, "command")) {
					req := ApprovalRequest{
						CallID: call.ID, Tool: call.Name,
						Command: argString(call.Args, "command"), Risk: tool.Risk().String(),
						HighRisk: IsHighRisk(argString(call.Args, "command")),
					}
					e.emit(core.EvtApprovalNeeded, core.ApprovalData{
						CallID: req.CallID, Tool: req.Tool, Args: call.Args, Risk: req.Risk,
					})
					var decision ApprovalDecision
					if e.approvalHandler != nil {
						var aerr error
						decision, aerr = e.approvalHandler(ctx, req)
						if aerr != nil {
							decision = ApprovalDecision{Reason: aerr.Error()}
						}
					} else {
						decision = ApprovalDecision{Reason: "非交互模式：需要批准但未装配审批处理器"}
					}
					if !decision.Allowed {
						reason := decision.Reason
						if reason == "" {
							reason = "用户拒绝执行"
						}
						e.emit(core.EvtToolDenied, core.ToolDeniedData{CallID: call.ID, Reason: reason})
						result = "用户拒绝执行：" + reason
						e.emit(core.EvtToolEnd, core.ToolEndData{CallID: call.ID, OK: false, Output: result})
						toolResults = append(toolResults, toolResultMsg(call.ID, result))
						continue
					}
					if decision.AllowAll && !req.HighRisk {
						e.approvedAll[call.Name] = true
					}
				}
				e.emit(core.EvtToolStart, core.ToolStartData{CallID: call.ID})
				started := time.Now()
				r, runErr := tool.Run(ctx, []byte(call.Args))
				if runErr != nil {
					r = tools.Result{OK: false, Output: runErr.Error(), Provenance: core.ProvObserved}
				}
				exitCode := 0
				if r.Meta != nil && r.Meta["exit_code"] != "" {
					_, _ = fmt.Sscanf(r.Meta["exit_code"], "%d", &exitCode)
				}
				e.emit(core.EvtToolEnd, core.ToolEndData{
					CallID: call.ID, OK: r.OK, ExitCode: exitCode,
					DurationMS: time.Since(started).Milliseconds(),
					Output:     truncateRunes(r.Output, 2000), Truncated: len(r.Output) > 2000,
				})
				if !r.OK {
					e.ledger.Record(core.LedgerEntry{
						TurnID: turn.ID, What: call.Name, Why: truncateRunes(r.Output, 200),
					})
				}
				target := argString(call.Args, "path")
				if target == "" {
					target = argString(call.Args, "target")
				}
				e.hook.OnToolEnd(call.Name, target, r)
				if r.OK && (call.Name == "write_file" || call.Name == "edit_file") {
					changes = append(changes, core.Change{Path: target})
					e.didWrite = true
				}
				if r.OK && call.Name == "apply_patch" {
					e.didWrite = true
					for _, name := range patchPaths(call.Args) {
						changes = append(changes, core.Change{Path: name})
					}
				}
				if r.OK && call.Name == "shell" {
					changes = append(changes, core.Change{Kind: core.ChangeCommand})
				}
				result = tools.Compress(r.Output)
				if call.Name == "read_file" {
					if outside, handled := e.outsideRead(ctx, call, result, input); handled {
						result = outside
					}
				}
			}
			toolResults = append(toolResults, toolResultMsg(call.ID, result))
		}

		if stop {
			break steps
		}
		messages = append(messages, llm.Message{Role: "assistant", Content: text.String(), Calls: calls})
		messages = append(messages, toolResults...)
	}
	if terminal == "" {
		terminal = core.TermHalt
	}
	return terminal, nil
}

// hasWriteCall 判断本步是否包含写操作。
func hasWriteCall(calls []llm.ToolCall) bool {
	for _, c := range calls {
		switch c.Name {
		case "write_file", "edit_file", "apply_patch":
			return true
		case "shell":
			if !IsReadOnlyCommand(argString(c.Args, "command")) {
				return true
			}
		}
	}
	return false
}

// patchPaths 从 apply_patch 参数中提取被修改的文件路径。
func patchPaths(args string) []string {
	var m struct {
		Patch string `json:"patch"`
	}
	if json.Unmarshal([]byte(args), &m) != nil {
		return nil
	}
	var out []string
	for _, line := range strings.Split(m.Patch, "\n") {
		trimmed := strings.TrimSpace(line)
		for _, prefix := range []string{"*** Update File:", "*** Add File:", "*** Delete File:"} {
			if strings.HasPrefix(trimmed, prefix) {
				path := strings.TrimSpace(strings.TrimPrefix(trimmed, prefix))
				if path != "" {
					out = append(out, path)
				}
			}
		}
	}
	return out
}

// outsideRead 区分“用户要读”与“模型想读”：只有用户输入里出现该路径才发起单独审批。
func (e *Engine) outsideRead(ctx context.Context, call llm.ToolCall, result, input string) (string, bool) {
	if !strings.Contains(result, "outside sandbox") {
		return result, false
	}
	path := argString(call.Args, "path")
	if path == "" || !strings.Contains(input, path) {
		msg := "拒绝：该路径未出现在用户输入中，不能读取工作区外文件"
		e.emit(core.EvtNotice, core.NoticeData{Level: "warn", Text: msg})
		return msg, true
	}
	if e.approvalHandler == nil {
		msg := "拒绝：工作区外读取需要交互式批准"
		e.emit(core.EvtNotice, core.NoticeData{Level: "warn", Text: msg})
		return msg, true
	}
	d, err := e.approvalHandler(ctx, core.ApprovalRequest{
		CallID: call.ID, Tool: "outside_read", Command: path,
		Risk: "net", HighRisk: true,
	})
	if err != nil || !d.Allowed {
		msg := "用户拒绝工作区外读取"
		e.emit(core.EvtNotice, core.NoticeData{Level: "warn", Text: msg})
		return msg, true
	}
	data, err := os.ReadFile(path)
	if err != nil {
		msg := "读取失败: " + err.Error()
		e.emit(core.EvtNotice, core.NoticeData{Level: "warn", Text: msg})
		return msg, true
	}
	content := tools.Compress(string(data))
	e.emit(core.EvtNotice, core.NoticeData{Level: "warn", Text: "工作区外读取 " + path + ": " + truncateRunes(content, 80)})
	return content, true
}

func toolResultMsg(callID, output string) llm.Message {
	return llm.Message{Role: "tool", ToolID: callID, Content: output}
}

func argString(raw string, key string) string {
	if raw == "" {
		return ""
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		return ""
	}
	v, ok := m[key]
	if !ok {
		return ""
	}
	var s string
	if err := json.Unmarshal(v, &s); err != nil {
		return ""
	}
	return s
}

func collectTargets(changes []core.Change) []string {
	var out []string
	seen := map[string]bool{}
	for _, c := range changes {
		if c.Path != "" && !seen[c.Path] {
			seen[c.Path] = true
			out = append(out, c.Path)
		}
	}
	return out
}

func truncateRunes(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n])
}
