package tools

import (
	"context"
	"encoding/json"
	"strconv"

	"github.com/MC5lan/HonestBoy-CLI/internal/core"
)

// RequestDoneTool 是申请完成的专用工具。自然语言里的“我完成了”不算申请。
type RequestDoneTool struct{}

func (t *RequestDoneTool) Name() string { return "request_done" }
func (t *RequestDoneTool) Description() string {
	return "申请完成；必须提供本回合完成摘要，由 Gate 判定"
}
func (t *RequestDoneTool) Schema() json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{"summary":{"type":"string"}},"required":["summary"]}`)
}
func (t *RequestDoneTool) Risk() Risk { return RiskRead }
func (t *RequestDoneTool) Run(ctx context.Context, args json.RawMessage) (Result, error) {
	return Result{OK: true, Output: "完成申请已提交", Provenance: core.ProvUnknown}, nil
}

// DeclareUntestedTool 是弃权专用工具，让弃权比假装完成更容易。
type DeclareUntestedTool struct{}

func (t *DeclareUntestedTool) Name() string { return "declare_untested" }
func (t *DeclareUntestedTool) Description() string {
	return "明确声明未验证/未完成，并列出已完成与未完成项"
}
func (t *DeclareUntestedTool) Schema() json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{"reason":{"type":"string"},"done":{"type":"array","items":{"type":"string"}},"not_done":{"type":"array","items":{"type":"string"}},"next":{"type":"string"}},"required":["reason","not_done"]}`)
}
func (t *DeclareUntestedTool) Risk() Risk { return RiskRead }
func (t *DeclareUntestedTool) Run(ctx context.Context, args json.RawMessage) (Result, error) {
	return Result{OK: true, Output: "弃权声明已提交", Provenance: core.ProvUnknown}, nil
}

// VerifyBuildTool / VerifyTestTool / VerifyVetTool 让验证器出现在工具声明里。
type verifyTool struct {
	name, desc string
	schema     json.RawMessage
	sb         *Sandbox
	kind       core.EvidenceKind
	fn         func(context.Context, *Sandbox, string) (core.Evidence, error)
}

func newVerifyTool(name, desc, schema string, sb *Sandbox, kind core.EvidenceKind,
	fn func(context.Context, *Sandbox, string) (core.Evidence, error)) *verifyTool {
	return &verifyTool{name: name, desc: desc, schema: json.RawMessage(schema), sb: sb, kind: kind, fn: fn}
}

func (t *verifyTool) Name() string            { return t.name }
func (t *verifyTool) Description() string     { return t.desc }
func (t *verifyTool) Schema() json.RawMessage { return t.schema }
func (t *verifyTool) Risk() Risk              { return RiskExec }
func (t *verifyTool) Run(ctx context.Context, args json.RawMessage) (Result, error) {
	var a struct {
		Target string `json:"target"`
	}
	_ = json.Unmarshal(args, &a)
	target := a.Target
	if target == "" {
		target = "."
	}
	ev, err := t.fn(ctx, t.sb, target)
	if err != nil {
		return Result{OK: false, Output: err.Error(), Provenance: core.ProvObserved}, nil
	}
	return Result{
		Output: ev.Summary, OK: ev.OK,
		Meta: map[string]string{
			"kind": string(t.kind), "summary": ev.Summary,
			"exit_code": fmtInt(ev.ExitCode), "evidence_id": ev.ID,
			"level": ev.Level.String(), "ran": strconv.FormatBool(ev.Ran),
			"raw_hash": ev.RawHash, "raw": ev.Raw,
		},
		Provenance: ev.Provenance,
	}, nil
}

func fmtInt(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}

// NewDefaultRegistry 注册全部工具；engine 直接用它装配。
func NewDefaultRegistry(sb *Sandbox, guard *ReadGuard, shell string) *Registry {
	r := NewRegistry()
	_ = r.Register(NewReadFileTool(sb, guard))
	_ = r.Register(NewWriteFileTool(sb, guard))
	_ = r.Register(NewListDirTool(sb))
	_ = r.Register(NewEditFileTool(sb, guard))
	_ = r.Register(NewApplyPatchTool(sb, guard))
	_ = r.Register(NewShellTool(sb, shell))
	_ = r.Register(NewSearchTool(sb))
	_ = r.Register(newVerifyTool("verify_build", "构建验证，产出 EvBuild",
		`{"type":"object","properties":{"target":{"type":"string"}}}`, sb, core.EvBuild, Build))
	_ = r.Register(newVerifyTool("verify_test", "测试验证（-count=1），产出 EvTest",
		`{"type":"object","properties":{"target":{"type":"string"}}}`, sb, core.EvTest, Test))
	_ = r.Register(newVerifyTool("verify_vet", "静态检查，产出 EvVet",
		`{"type":"object","properties":{"target":{"type":"string"}}}`, sb, core.EvVet, Vet))
	_ = r.Register(&RequestDoneTool{})
	_ = r.Register(&DeclareUntestedTool{})
	return r
}
