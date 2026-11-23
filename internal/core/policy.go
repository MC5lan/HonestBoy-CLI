package core

import (
	"path/filepath"
	"strings"
)

// 改动类型常量，供 engine 提取改动时填写 Change.Kind。
const (
	ChangeGo      = "go"
	ChangeDoc     = "doc"
	ChangeCommand = "command"
	ChangeAnswer  = "answer"
	ChangeBugfix  = "bugfix"
)

// changeRule 是数据化的证据规则，避免 if-else 阶梯。
type changeRule struct {
	name         string
	match        func(Change) bool
	kinds        []EvidenceKind
	needReadEcho bool
	needTestName bool
	noEvidence   bool
}

// changeRules 是默认证据要求策略表（T107）。顺序不影响结果，规则可叠加。
var changeRules = []changeRule{
	{
		name: "bugfix",
		match: func(c Change) bool {
			return c.Kind == ChangeBugfix || strings.Contains(strings.ToLower(c.Path), "bug")
		},
		kinds:        []EvidenceKind{EvTest},
		needTestName: true,
	},
	{
		name: "go-source",
		match: func(c Change) bool {
			return c.Kind == ChangeGo || strings.EqualFold(filepath.Ext(c.Path), ".go")
		},
		// EvDiff 是“改了什么”的证据，build/test 是“改对了没有”的证据，二者都要。
		kinds: []EvidenceKind{EvBuild, EvTest, EvDiff},
	},
	{
		name: "non-code-file",
		match: func(c Change) bool {
			ext := strings.ToLower(filepath.Ext(c.Path))
			return c.Kind == ChangeDoc || ext == ".md" || ext == ".json" || ext == ".yaml" || ext == ".yml"
		},
		kinds:        []EvidenceKind{EvFile, EvDiff},
		needReadEcho: true,
	},
	{
		name:  "command",
		match: func(c Change) bool { return c.Kind == ChangeCommand },
		kinds: []EvidenceKind{EvCommand},
	},
	{
		name:       "answer",
		match:      func(c Change) bool { return c.Kind == ChangeAnswer },
		noEvidence: true,
	},
}

// RequiredFor 根据改动类型推导默认证据要求。
// 规则以数据形式表达；模型自报的 Req 由 engine 用本函数结果覆盖。
func RequiredFor(changes []Change) Requirement {
	var r Requirement
	seenKinds := make(map[EvidenceKind]bool)
	seenTargets := make(map[string]bool)
	answerCount := 0
	effective := 0

	for _, c := range changes {
		if c.Path == "" && c.Kind == "" {
			continue // 空改动不产生证据要求
		}
		effective++
		for _, rule := range changeRules {
			if !rule.match(c) {
				continue
			}
			for _, k := range rule.kinds {
				if !seenKinds[k] {
					seenKinds[k] = true
					r.Kinds = append(r.Kinds, k)
				}
			}
			if rule.needReadEcho {
				r.NeedReadEcho = true
			}
			if rule.needTestName {
				r.NeedTestName = true
			}
			if rule.noEvidence {
				answerCount++
			}
			if c.Path != "" && !seenTargets[c.Path] {
				seenTargets[c.Path] = true
				r.Targets = append(r.Targets, c.Path)
			}
		}
	}

	if effective == 0 || answerCount == effective {
		r.NoEvidence = true
		r.Kinds = nil
		r.Targets = nil
	} else if len(r.Kinds) == 0 {
		// 未知类型的文件改动：至少要求变更证据（EvDiff），否则 Gate 的
		// 类型集合为空，任何证据都会被判 evidence_kind。
		r.Kinds = []EvidenceKind{EvDiff}
	}
	r.MinCount = len(r.Kinds)
	return r
}
