package engine

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/MC5lan/HonestBoy-CLI/internal/core"
	"github.com/MC5lan/HonestBoy-CLI/internal/llm"
	"github.com/MC5lan/HonestBoy-CLI/internal/tools"
)

// runTurn 是单次回合主循环：模型流 → 工具执行 → Gate 判定 → 终态与 Receipt。
func (e *Engine) runTurn(ctx context.Context, input, history string) (core.Terminal, error) {
	s := e.session
	turn := &core.Turn{ID: "t_" + shortID(input+time.Now().UTC().Format(time.RFC3339Nano))}
	s.Turns = append(s.Turns, turn)
	if err := turn.Transition(core.StThinking); err != nil {
		return core.TermFailed, err
	}
	var plan []core.PlanItem // 测试计划（文档 §4.3）
	var changes []core.Change
	toolCalls := 0
	remediation := 0
	abstainRetries := 0
	selfRetries := 0
	terminal := core.Terminal("")

	e.emit(core.EvtTurnStart, core.TurnStartData{Input: input, Model: e.cfg.Model})

	// 回合结束时校验测试计划三态（passed 必须有可接地的证据）。
	defer func() {
		if !e.cfg.PlanFirst || len(plan) == 0 {
			return
		}
		if err := core.Finalize(plan); err != nil {
			e.emit(core.EvtNotice, core.NoticeData{Level: "warn", Text: "测试计划校验失败：" + err.Error()})
		}
	}()

	// 歧义暂停（文档 §4.2）：悬空指代一律 Warn，明确要求 Halt 时直接停。
	if e.cfg.AmbiguityHalt && core.HasDanglingReference(input) {
		reason := "输入包含指代（如“那个文件”），无法确定唯一目标"
		e.emit(core.EvtNotice, core.NoticeData{Level: "warn", Text: reason + "，请说明具体文件或路径"})
		e.writeReceipt(core.TermHalt, input)
		_ = turn.Transition(core.StClosed)
		if e.cfg.Tracer != nil {
			e.cfg.Tracer.RecordHalt(reason)
		}
		return core.TermHalt, nil
	}

	if isHomeDir(e.cfg.Workdir) {
		e.emit(core.EvtNotice, core.NoticeData{Level: "warn", Text: "当前是用户主目录，已跳过检查点；建议在具体项目目录运行 hb"})
	} else if cp, cerr := CreateCheckpoint(e.cfg.Workdir, filepath.Join(e.store.Paths.Sessions, s.ID), turn.ID, input); cerr == nil {
		if cp.Truncated {
			e.emit(core.EvtNotice, core.NoticeData{
				Level: "warn",
				Text:  fmt.Sprintf("检查点已截断（达到 %d 文件或 %dMB 上限）；建议在更小的项目目录运行 hb", maxCheckpointFiles, maxCheckpointBytes/(1<<20)),
			})
		} else {
			e.emit(core.EvtNotice, core.NoticeData{Level: "info", Text: "检查点已创建 " + cp.ID})
		}
	}

	repoMap := ""
	if e.cfg.RepoMapTokens >= 0 && !isHomeDir(e.cfg.Workdir) {
		repoMap = RepoMapContext(e.cfg.Workdir, e.cfg.RepoMapTokens)
	}
	extra := repoMap
	if history != "" {
		if extra != "" {
			extra += "\n\n"
		}
		extra += "## 之前回合\n" + history
	}
	contextStart := time.Now()
	hbContext := LoadProjectContext(e.cfg.Workdir, e.cfg.HBMDLimit)
	sys := AssembleContext(systemPrompt, hbContext, EnvContext(e.cfg.Workdir), extra, "")
	if e.cfg.PlanMode {
		sys = WithPlanRules(sys)
	}
	if e.cfg.SelfAssessment {
		sys = sys + "\n" + core.SelfPromptFragment
	}
	if core.RigorEnabled(e.cfg.Rigor) {
		sys = sys + "\n" + core.RigorPromptFragment
	}
	// 历史失败案例注入（文档 §3.5.4）：Top-3、阈值 0.35、标明来源是历史。
	if e.cfg.LedgerRecall {
		hits := e.ledger.Search(core.Query{Tags: extractTags(input), Files: changesPaths(input)}, 3, 0.35)
		if ctxText := core.RenderLedgerContext(hits); ctxText != "" {
			sys = sys + "\n" + ctxText
			e.emit(core.EvtNotice, core.NoticeData{Level: "info", Text: fmt.Sprintf("注入了 %d 条历史失败案例", len(hits))})
		}
	}
	e.profile.ContextMS += time.Since(contextStart).Milliseconds()
	messages := []llm.Message{
		{Role: "system", Content: sys},
		{Role: "user", Content: input},
	}
	budget := Budget{Ratio: 0.7, KeepRecent: 2, WindowSize: e.cfg.ContextWindow}
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
			MaxTokens: e.cfg.MaxOutputTokens, ReasoningEffort: e.cfg.ReasoningEffort,
		}
		streamStart := time.Now()
		firstChunkAt := time.Time{}
		chunks, errCh := e.cfg.ResolveClient().Stream(ctx, req)
		var text strings.Builder
		var reasoning strings.Builder
		var calls []llm.ToolCall
		var stepUsage llm.Usage
		streamDone := false
		truncated := false
		for !streamDone {
			select {
			case c, ok := <-chunks:
				if !ok {
					streamDone = true
					continue
				}
				if c.Reasoning != "" {
					reasoning.WriteString(c.Reasoning)
				}
				if c.Truncated {
					truncated = true
				}
				if c.Text != "" {
					if firstChunkAt.IsZero() {
						firstChunkAt = time.Now()
					}
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
		e.profile.StreamMS += time.Since(streamStart).Milliseconds()
		if e.profile.FirstTokenMS == 0 && !firstChunkAt.IsZero() {
			e.profile.FirstTokenMS = firstChunkAt.Sub(streamStart).Milliseconds()
		}
		if stepUsage.In > 0 || stepUsage.Out > 0 {
			e.usage.Add(stepUsage)
			toolTokens := 0
			for _, m := range messages {
				if m.Role == "tool" {
					toolTokens += core.TokenEstimate(m.Content)
				}
			}
			e.emit(core.EvtUsage, core.UsageData{
				TokensIn: int64(e.usage.In), TokensOut: int64(e.usage.Out),
				CostMicros: e.usage.CostMicros, PromptTokens: int64(stepUsage.In),
				SystemTokens:  int64(core.TokenEstimate(systemPrompt)),
				HBMTokens:     int64(core.TokenEstimate(hbContext)),
				HistoryTokens: int64(core.TokenEstimate(history)),
				ToolTokens:    int64(toolTokens),
			})
		}
		// 输出被 max_tokens 截断且没有任何工具调用：本步没有可执行结果，
		// 不能伪装成“未验证”，按失败结束并给出原因。
		if truncated && len(calls) == 0 {
			msg := "模型输出达到 max_tokens 上限（推理可能耗尽预算），本步没有产生工具调用"
			e.emit(core.EvtError, core.ErrorData{Message: msg})
			e.writeReceipt(core.TermFailed, msg)
			_ = turn.Transition(core.StClosed)
			return core.TermFailed, nil
		}

		// 形态观测（文档 §5）：确定性规则检测自我保证/投降表述，只警告不改判据。
		if text.Len() > 0 {
			rep := core.Analyze(text.String(), "normal")
			if len(rep.Warrants) > 0 {
				e.emit(core.EvtNotice, core.NoticeData{Level: "warn", Text: "检测到自我保证表述：" + rep.Warrants[0]})
			}
			if len(rep.Deference) > 0 {
				e.emit(core.EvtNotice, core.NoticeData{Level: "warn", Text: "检测到投降表述：" + rep.Deference[0]})
			}
		}
		// 测试计划（文档 §4.3）：解析后供回合末三态校验。
		if e.cfg.PlanFirst {
			if items := core.ParseTestPlan(text.String()); len(items) > 0 {
				plan = items
				e.emit(core.EvtNotice, core.NoticeData{Level: "info", Text: fmt.Sprintf("记录测试计划 %d 条", len(items))})
			}
		}

		// 自评块：解析并让它影响控制流（文档 §4.1.3）。
		sa, selfOK := core.ParseSelf(text.String())
		if e.cfg.SelfAssessment && !selfOK {
			sa = core.SelfAssessment{Conf: "unknown"}
		}
		if selfOK {
			// 一次合规自评即清零连续缺失计数：偶发遗漏可补救，
			// 只有连续多次缺失才终止回合。
			selfRetries = 0
		}
		if e.cfg.Tracer != nil {
			e.cfg.Tracer.RecordClaim(text.String(), "thinking", false)
		}
		if e.cfg.SelfAssessment && sa.BlocksWrite() && hasWriteCall(calls) {
			// 缺少自评块时先拒执行本步并给模型一次补救机会；
			// 明确 conf=unknown 或多次缺失才终止回合。
			if !selfOK && selfRetries < 2 {
				selfRetries++
				e.emit(core.EvtNotice, core.NoticeData{
					Level: "warn", Text: "缺少 [SELF] 自评块，本步动作未执行；请补自评后重试",
				})
				messages = append(messages, llm.Message{Role: "assistant", Content: text.String(), Calls: calls, Reasoning: reasoning.String()})
				for _, c := range calls {
					reason := "缺少 [SELF] 自评块，本步未执行"
					e.emit(core.EvtToolDenied, core.ToolDeniedData{CallID: c.ID, Reason: reason})
					e.emit(core.EvtToolEnd, core.ToolEndData{CallID: c.ID, OK: false, Output: reason})
					messages = append(messages, toolResultMsg(c.ID, "系统拒绝执行："+reason+"。请先输出自评块再重试。"))
				}
				continue steps
			}
			reason := "自评置信度为 unknown 或假设缺少证伪条件，本轮不执行写操作"
			if !selfOK {
				reason = "未输出自评块，按 unknown 处理；本轮不执行写操作"
			}
			for _, c := range calls {
				e.emit(core.EvtToolDenied, core.ToolDeniedData{CallID: c.ID, Reason: reason})
				e.emit(core.EvtToolEnd, core.ToolEndData{CallID: c.ID, OK: false, Output: reason})
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
			// 自评被拒后模型没有任何输出，说明补救无法继续；按 halt 结束而不是伪装成弃权。
			if selfRetries > 0 && strings.TrimSpace(text.String()) == "" {
				reason := "模型在自评补救后未返回任何动作"
				e.emit(core.EvtNotice, core.NoticeData{Level: "warn", Text: reason})
				e.writeReceipt(core.TermHalt, reason)
				_ = turn.Transition(core.StClosed)
				return core.TermHalt, nil
			}
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
		prefetched := e.prefetchReadOnly(ctx, calls)
		for idx, call := range calls {
			toolCalls++
			if halt, reason := e.guard.Check(e.ledger, toolCalls, turn.ID); halt {
				e.emit(core.EvtNotice, core.NoticeData{Level: "error", Text: reason})
				e.writeReceipt(core.TermHalt, reason)
				_ = turn.Transition(core.StClosed)
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
					// 授权判定统一走 core.Authorize（文档 F5）：不可逆动作只认本次授权。
					auth := AuthorityOf(tool, req.Command)
					if !core.Authorize(auth, map[core.Authority]bool{auth: decision.Allowed}) {
						decision.Allowed = false
						if decision.Reason == "" {
							decision.Reason = "未获得该权限级别的授权"
						}
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
					if decision.AllowAll {
						if auth == core.AuthIrreversible {
							e.emit(core.EvtNotice, core.NoticeData{
								Level: "warn", Text: "不可逆动作不记忆授权：下次仍需单独批准",
							})
						} else {
							e.approvedAll[call.Name] = true
						}
					}
				}
				e.emit(core.EvtToolStart, core.ToolStartData{CallID: call.ID})
				started := time.Now()
				var r tools.Result
				var runErr error
				durationMS := int64(0)
				if prefetched != nil && prefetched[idx].done {
					r = prefetched[idx].r
					durationMS = prefetched[idx].durationMS
				} else {
					r, runErr = tool.Run(ctx, []byte(call.Args))
					if runErr != nil {
						r = tools.Result{OK: false, Output: runErr.Error(), Provenance: core.ProvObserved}
					}
					durationMS = time.Since(started).Milliseconds()
				}
				exitCode := 0
				if r.Meta != nil && r.Meta["exit_code"] != "" {
					_, _ = fmt.Sscanf(r.Meta["exit_code"], "%d", &exitCode)
				}
				e.profile.ToolMS += durationMS
				e.profile.ToolCalls++
				e.emit(core.EvtToolEnd, core.ToolEndData{
					CallID: call.ID, OK: r.OK, ExitCode: exitCode,
					DurationMS: durationMS,
					Output:     truncateRunes(r.Output, 2000), Truncated: len(r.Output) > 2000,
					Diff: truncateRunes(r.Meta["diff"], 12000),
				})
				if !r.OK {
					target := argString(call.Args, "path")
					if target == "" {
						target = argString(call.Args, "target")
					}
					e.recordFailure(core.LedgerEntry{
						TurnID: turn.ID, What: call.Name,
						Why:  truncateRunes(r.Output, 200),
						Kind: call.Name,
						Tags: extractTags(input),
						Files: func() []string {
							if target != "" {
								return []string{target}
							}
							return nil
						}(),
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
				if core.RigorEnabled(e.cfg.Rigor) && r.OK && (call.Name == "write_file" || call.Name == "edit_file" || call.Name == "apply_patch") {
					if issues := core.PlaceholderWarnings(qualityText(call.Args)); len(issues) > 0 {
						msg := "检测到占位符/未实现逻辑：" + strings.Join(issues, ", ")
						e.emit(core.EvtNotice, core.NoticeData{Level: "warn", Text: msg + "（" + target + "）"})
						result += "\n[质量检查] " + msg + "。请补全实现；如无法完成，请调用 declare_untested 说明缺口。"
					}
				}
				if call.Name == "shell" {
					// shell 成功但无输出很常见（如 node --check）；把退出码回传给模型，
					// 避免它把“空输出”误判成失败。
					code := r.Meta["exit_code"]
					if strings.TrimSpace(result) == "" {
						result = "(命令结束，退出码 " + code + "，无输出)"
					} else {
						result = "(退出码 " + code + ")\n" + result
					}
				}
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
		messages = append(messages, llm.Message{Role: "assistant", Content: text.String(), Calls: calls, Reasoning: reasoning.String()})
		messages = append(messages, toolResults...)
	}
	if terminal == "" {
		terminal = core.TermHalt
	}
	return terminal, nil
}

// extractTags 从用户输入提取检索标签（简单分词，限定小写单词）。
func extractTags(input string) []string {
	fields := strings.FieldsFunc(input, func(r rune) bool {
		return !(r >= 'a' && r <= 'z') && !(r >= 'A' && r <= 'Z') && !(r >= '0' && r <= '9')
	})
	var out []string
	seen := map[string]bool{}
	for _, f := range fields {
		f = strings.ToLower(f)
		if len(f) < 3 || seen[f] {
			continue
		}
		seen[f] = true
		out = append(out, f)
	}
	return out
}

// changesPaths 从输入里提取可能的文件路径（含 / 或 . 的词）。
func changesPaths(input string) []string {
	var out []string
	for _, f := range strings.Fields(input) {
		if strings.ContainsAny(f, `/\`) || strings.Contains(f, ".go") || strings.Contains(f, ".md") {
			out = append(out, strings.Trim(f, `,.;:()[]{}"'`))
		}
	}
	return out
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

// qualityText 只取本次写入的新内容做质量扫描，避免 edit 的 old 片段误报。
func qualityText(args string) string {
	var m map[string]any
	if json.Unmarshal([]byte(args), &m) != nil {
		return args
	}
	for _, k := range []string{"content", "new", "patch", "new_string"} {
		if v, ok := m[k].(string); ok {
			return v
		}
	}
	return args
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

type prefetchedTool struct {
	r          tools.Result
	durationMS int64
	done       bool
}

// prefetchReadOnly 在同一模型步里并行执行只读工具；写/exec/net 与完成申请保持串行。
// 结果按原调用顺序返回，事件与证据仍由主循环串行产出。
func (e *Engine) prefetchReadOnly(ctx context.Context, calls []llm.ToolCall) []prefetchedTool {
	if len(calls) < 2 {
		return nil
	}
	for _, call := range calls {
		if call.Name == "request_done" || call.Name == "declare_untested" {
			return nil
		}
		tool, ok := e.registry.Get(call.Name)
		if !ok || tool.Risk() != tools.RiskRead || e.approval.NeedApproval(tool, "") {
			return nil
		}
	}
	out := make([]prefetchedTool, len(calls))
	var wg sync.WaitGroup
	for i, call := range calls {
		wg.Add(1)
		go func(i int, call llm.ToolCall) {
			defer wg.Done()
			defer func() {
				if rec := recover(); rec != nil {
					out[i] = prefetchedTool{
						r:    tools.Result{OK: false, Output: fmt.Sprint("工具 panic: ", rec), Provenance: core.ProvObserved},
						done: true,
					}
				}
			}()
			tool, _ := e.registry.Get(call.Name)
			started := time.Now()
			r, err := tool.Run(ctx, []byte(call.Args))
			if err != nil {
				r = tools.Result{OK: false, Output: err.Error(), Provenance: core.ProvObserved}
			}
			out[i] = prefetchedTool{r: r, durationMS: time.Since(started).Milliseconds(), done: true}
		}(i, call)
	}
	wg.Wait()
	return out
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
