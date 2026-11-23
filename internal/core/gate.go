package core

import "strings"

// Policy 是 Gate 的可配置策略。Strict 为 true 时禁止任何降级。
type Policy struct{ Strict bool }

// Gate 是唯一有权判定“完成”的组件。判定是纯函数：
// 不入参时间、不入参环境、不读文件，同样输入永远同样输出。
type Gate struct{ policy Policy }

func NewGate(p Policy) *Gate { return &Gate{policy: p} }

// Decision 是 Gate 的结论。Rule 给机器，Reason 给人。
type Decision struct {
	Allow      bool
	Terminal   Terminal
	Evidence   []string // 被采信的证据 ID
	Missing    []string // 缺失的目标或证据类型
	Rule       string
	Reason     string // 给模型的中文短句
	Summary    string // 触发失败的证据原文摘要
	Provenance string // 触发失败的证据来源
	Needed     []string
	Got        string
}

func decideDeny(rule, reason string) Decision {
	return Decision{Allow: false, Terminal: TermDenied, Rule: rule, Reason: reason}
}

// Evaluate 按固定顺序判定：
// evidence_kind → evidence_failed → coverage → provenance → allow。
// 顺序不可更换。
func (g *Gate) Evaluate(c Claim, bag EvidenceBag) Decision {
	items := bag.active()

	// 纯文本回答：无改动、无证据要求，允许 done。
	if c.Req.NoEvidence || (len(c.Req.Kinds) == 0 && len(c.Targets) == 0) {
		return Decision{Allow: true, Terminal: TermDone, Reason: "纯文本回答，无需证据"}
	}

	allowed := make(map[EvidenceKind]bool, len(c.Req.Kinds))
	for _, k := range c.Req.Kinds {
		allowed[k] = true
	}

	// 1. 证据类型必须在允许集合内。
	for _, e := range items {
		if !allowed[e.Kind] {
			return decideDeny("evidence_kind", ReasonFor(Decision{
				Rule: "evidence_kind", Needed: kindNames(c.Req.Kinds), Got: string(e.Kind),
			}, true))
		}
	}

	// 2. 已运行的证据必须 OK。未运行属于证据缺失，不是失败测试。
	for _, e := range items {
		if e.Ran && !e.OK {
			d := decideDeny("evidence_failed", ReasonFor(Decision{
				Rule: "evidence_failed", Summary: e.Summary,
			}, true))
			d.Summary = e.Summary
			return d
		}
	}

	// 3. 覆盖全部目标与必需证据类型。
	var missing []string
	covered := make(map[string]bool, len(items))
	haveKind := make(map[EvidenceKind]bool, len(items))
	for _, e := range items {
		if e.Ran && e.OK {
			covered[e.Target] = true
			haveKind[e.Kind] = true
		} else if !e.Ran {
			// 验证器未真正运行（缺失/超时），按证据缺失处理，不当失败也不当成功。
			missing = append(missing, string(e.Kind))
		}
	}
	for _, t := range c.Targets {
		if !covered[t] {
			missing = append(missing, t)
		}
	}
	for _, k := range c.Req.Kinds {
		if !haveKind[k] {
			missing = append(missing, string(k))
		}
	}
	if c.Req.NeedReadEcho && countKind(items, EvFile) < 2 {
		// 一条 file 证据给变更后哈希，另一条给读取回显。
		missing = append(missing, "read_echo")
	}
	if c.Req.NeedTestName && !hasTestCaseName(items) {
		missing = append(missing, "test_name")
	}
	if len(missing) > 0 {
		d := decideDeny("coverage", ReasonFor(Decision{Rule: "coverage", Missing: missing}, true))
		d.Missing = missing
		return d
	}

	// 4. 接地谓词：只有一级验证器、实际运行且通过的证据能支撑完成。
	for _, e := range items {
		if e.CanGround() {
			continue
		}
		why := e.Provenance.String()
		switch {
		case !e.Ran:
			why = "未运行"
		case e.Level != EvidenceLevelVerifier:
			why = e.Level.String() + "（非一级验证器）"
		case !e.VerifyRawHash():
			why = "原文哈希不匹配"
		case e.ExitCode < 0:
			why = "退出码无效"
		}
		d := decideDeny("provenance", ReasonFor(Decision{
			Rule: "provenance", Provenance: why,
		}, true))
		d.Provenance = why
		return d
	}

	// 5. 全部满足。
	ids := make([]string, 0, len(items))
	for _, e := range items {
		ids = append(ids, e.ID)
	}
	return Decision{Allow: true, Terminal: TermDone, Evidence: ids, Reason: "证据齐备"}
}

func kindNames(kinds []EvidenceKind) []string {
	out := make([]string, 0, len(kinds))
	for _, k := range kinds {
		out = append(out, string(k))
	}
	return out
}

func countKind(items []Evidence, kind EvidenceKind) int {
	n := 0
	for _, e := range items {
		if e.Kind == kind && e.OK {
			n++
		}
	}
	return n
}

func hasTestCaseName(items []Evidence) bool {
	for _, e := range items {
		if e.Kind != EvTest || !e.OK {
			continue
		}
		if strings.Contains(e.Summary, "Test") || strings.Contains(e.Summary, "PASS") || strings.Contains(e.Summary, "ok ") {
			return true
		}
	}
	return false
}
