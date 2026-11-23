package engine

import (
	"strconv"
	"strings"

	"github.com/MC5lan/HonestBoy-CLI/internal/core"
	"github.com/MC5lan/HonestBoy-CLI/internal/tools"
)

// EvidenceHook 把工具结果转换为证据，并强制 Provenance。
type EvidenceHook struct{ bag *core.EvidenceBag }

func NewEvidenceHook() *EvidenceHook { return &EvidenceHook{bag: &core.EvidenceBag{}} }

// OnToolEnd 按工具类型产出证据：
// verify_* → EvBuild/EvTest/EvVet，edit/write → EvDiff，shell → EvCommand。
// 非 observed 的来源一律丢弃（T401）。
func (h *EvidenceHook) OnToolEnd(name, target string, r tools.Result) {
	if !r.Provenance.CanGround() {
		return
	}
	switch name {
	case "verify_build", "verify_test", "verify_vet":
		kinds := map[string]core.EvidenceKind{
			"verify_build": core.EvBuild, "verify_test": core.EvTest, "verify_vet": core.EvVet,
		}
		id := r.Meta["evidence_id"]
		if id == "" {
			id = "ev_" + shortID(name+target)
		}
		summary := r.Meta["summary"]
		if summary == "" {
			summary = strings.TrimSpace(firstLine(r.Output))
		}
		if summary == "" {
			summary = "验证器未返回摘要"
		}
		code, _ := strconv.Atoi(r.Meta["exit_code"])
		ran, _ := strconv.ParseBool(r.Meta["ran"])
		level := core.ParseEvidenceLevel(r.Meta["level"])
		h.bag.Add(core.Evidence{
			ID: id, Kind: kinds[name], Target: target, OK: r.OK, ExitCode: code,
			// Raw 必须与验证器原文逐字一致（可能为空），不能用摘要回退，否则哈希校验失败。
			Summary: summary, Raw: r.Meta["raw"], RawHash: r.Meta["raw_hash"],
			Level: level, Ran: ran, Provenance: core.ProvObserved,
		})
	case "edit_file", "write_file", "apply_patch":
		// 失败也必须留证：否则失败的工具不产生 change，Gate 会误判为纯问答而放行。
		if !r.OK {
			ev := core.Evidence{
				ID: "ev_fail_" + shortID(name+target+r.Output), Kind: core.EvDiff,
				Target: target, OK: false, ExitCode: -1,
				Summary: strings.TrimSpace(firstLine(r.Output)), Raw: r.Output,
				Level: core.EvidenceLevelVerifier, Ran: true, Provenance: core.ProvObserved,
			}
			ev.RawHash = ev.ComputeRawHash()
			h.bag.Add(ev)
			return
		}
		sha := r.Meta["sha256"]
		if sha == "" {
			return
		}
		ev := core.Evidence{
			ID: "ev_" + shortID(name+target), Kind: core.EvDiff, Target: target,
			OK: true, Summary: "变更后哈希 " + sha, Raw: r.Output,
			Level: core.EvidenceLevelVerifier, Ran: true, Provenance: core.ProvObserved,
		}
		ev.RawHash = ev.ComputeRawHash()
		h.bag.Add(ev)
	case "shell":
		code, _ := strconv.Atoi(r.Meta["exit_code"])
		summary := strings.TrimSpace(firstLine(r.Output))
		if summary == "" {
			summary = "命令退出码 " + r.Meta["exit_code"]
		}
		ev := core.Evidence{
			ID: "ev_" + shortID(name+target+summary), Kind: core.EvCommand, Target: "shell",
			OK: r.OK, ExitCode: code, Summary: summary, Raw: r.Output,
			Level: core.EvidenceLevelVerifier, Ran: true, Provenance: core.ProvObserved,
		}
		ev.RawHash = ev.ComputeRawHash()
		h.bag.Add(ev)
	}
}

func (h *EvidenceHook) Bag() *core.EvidenceBag { return h.bag }

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}

func shortID(s string) string {
	var sum uint64 = 14695981039346656037
	for _, b := range []byte(s) {
		sum ^= uint64(b)
		sum *= 1099511628211
	}
	const hex = "0123456789abcdef"
	var out [8]byte
	for i := 0; i < 8; i++ {
		out[i] = hex[(sum>>(i*4))&0xf]
	}
	return string(out[:])
}
