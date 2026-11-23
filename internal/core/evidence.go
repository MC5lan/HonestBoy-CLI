package core

import (
	"crypto/sha256"
	"encoding/hex"
	"time"
)

// EvidenceKind 是证据的稳定分类。
type EvidenceKind string

const (
	EvBuild   EvidenceKind = "build"
	EvTest    EvidenceKind = "test"
	EvVet     EvidenceKind = "vet"
	EvCommand EvidenceKind = "command"
	EvFile    EvidenceKind = "file"
	EvDiff    EvidenceKind = "diff"
)

// EvidenceLevel 是验证器级别（论文 §4.3）：
// 1=确定性验证器（编译/静态检查/测试），2=运行产物/冒烟，3=独立评审。
// 只有 1 级可以接地完成声明。
type EvidenceLevel uint8

const (
	EvidenceLevelNone     EvidenceLevel = 0 // 未标注，永不接地
	EvidenceLevelVerifier EvidenceLevel = 1
	EvidenceLevelRuntime  EvidenceLevel = 2
	EvidenceLevelReview   EvidenceLevel = 3
)

func (l EvidenceLevel) String() string {
	switch l {
	case EvidenceLevelVerifier:
		return "verifier"
	case EvidenceLevelRuntime:
		return "runtime"
	case EvidenceLevelReview:
		return "review"
	default:
		return "none"
	}
}

// ParseEvidenceLevel 解析级别字符串，未知值返回未标注（不可接地）。
func ParseEvidenceLevel(s string) EvidenceLevel {
	switch s {
	case "verifier", "1":
		return EvidenceLevelVerifier
	case "runtime", "2":
		return EvidenceLevelRuntime
	case "review", "3":
		return EvidenceLevelReview
	default:
		return EvidenceLevelNone
	}
}

// Evidence 是一条可被第三方复核的确定性验证结果。
// Summary 必须是验证器输出的原文片段，禁止模型改写。
type Evidence struct {
	ID         string        `json:"id"`
	Kind       EvidenceKind  `json:"kind"`
	Target     string        `json:"target"`
	OK         bool          `json:"ok"`
	ExitCode   int           `json:"exit_code"`
	Summary    string        `json:"summary"`
	Raw        string        `json:"raw,omitempty"`
	RawHash    string        `json:"raw_hash,omitempty"` // 验证器原始输出哈希，防事后改写
	StartedAt  time.Time     `json:"started_at"`
	Duration   int64         `json:"duration_ms"`
	Level      EvidenceLevel `json:"level"`
	Ran        bool          `json:"ran"` // 与“通过”分离：未运行是证据缺失，不是失败
	Provenance Provenance    `json:"provenance"`
	Superseded bool          `json:"superseded,omitempty"`
}

// CanGround 是可判定的证据接地谓词（论文定义 3）：
// 来源必须是验证器、必须实际运行过、退出码非负、必须通过、且为一级证据。
func (e Evidence) CanGround() bool {
	return e.Provenance.CanGround() && e.Ran && e.Level == EvidenceLevelVerifier &&
		e.ExitCode >= 0 && e.OK && e.VerifyRawHash()
}

// ComputeRawHash 计算验证器原始输出的哈希。
func (e Evidence) ComputeRawHash() string {
	sum := sha256.Sum256([]byte(e.Raw))
	return hex.EncodeToString(sum[:])
}

// VerifyRawHash 校验摘要未被替换；老证据（无 raw_hash）视为通过。
func (e Evidence) VerifyRawHash() bool {
	if e.RawHash == "" {
		return true
	}
	return e.RawHash == e.ComputeRawHash()
}

// Requirement 描述一次完成申请必须满足的证据集合。
type Requirement struct {
	Kinds        []EvidenceKind
	Targets      []string
	MinCount     int
	NeedReadEcho bool
	NeedTestName bool
	NoEvidence   bool
}

// Change 是一处改动，由 engine 从工具调用中自动提取。
type Change struct {
	Path string
	Kind string
}

// EvidenceBag 收集并去重证据，保留被取代的旧证据供审计。
type EvidenceBag struct{ items []Evidence }

// Add 按 ID 去重；空 Summary 的证据被拒绝。
// 同一 Kind+Target 再次出现时，旧条目标记 superseded，新条目追加。
func (b *EvidenceBag) Add(e Evidence) {
	if e.Summary == "" {
		return
	}
	for i := range b.items {
		if b.items[i].ID == e.ID {
			return
		}
	}
	for i := range b.items {
		if b.items[i].Kind == e.Kind && b.items[i].Target == e.Target {
			b.items[i].Superseded = true
		}
	}
	b.items = append(b.items, e)
}

// For 返回目标对应的有效证据（不含 superseded）。
func (b *EvidenceBag) For(target string) []Evidence {
	var out []Evidence
	for _, e := range b.items {
		if !e.Superseded && e.Target == target {
			out = append(out, e)
		}
	}
	return out
}

// Satisfies 判断当前有效证据是否满足要求。
func (b *EvidenceBag) Satisfies(req Requirement) bool {
	active := b.active()
	if req.NoEvidence {
		return true
	}
	if len(active) < req.MinCount {
		return false
	}
	for _, k := range req.Kinds {
		found := false
		for _, e := range active {
			if e.Kind == k && e.OK && e.CanGround() {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

// All 返回全部证据（含 superseded），供审计与哈希链使用。
func (b *EvidenceBag) All() []Evidence { return append([]Evidence(nil), b.items...) }

func (b *EvidenceBag) active() []Evidence {
	var out []Evidence
	for _, e := range b.items {
		if !e.Superseded {
			out = append(out, e)
		}
	}
	return out
}

// Count 返回有效证据条数。
func (b *EvidenceBag) Count() int { return len(b.active()) }

// GateContext 保存本轮任务的证据集合（文档 §3.1.2）。
type GateContext struct{ Evidences []Evidence }

// LastPass 返回最近一次可作依据的通过证据（指定级别）；没有则返回 nil。
// 调用方必须处理 nil 并阻断动作（规则 R4）。
func (g *GateContext) LastPass(level EvidenceLevel) *Evidence {
	for i := len(g.Evidences) - 1; i >= 0; i-- {
		e := g.Evidences[i]
		if e.Level == level && e.CanGround() {
			return &g.Evidences[i]
		}
	}
	return nil
}
