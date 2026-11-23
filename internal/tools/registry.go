package tools

import (
	"context"
	"encoding/json"

	"github.com/MC5lan/HonestBoy-CLI/internal/core"
)

type Risk uint8

const (
	RiskRead Risk = iota
	RiskWrite
	RiskExec
	RiskNet
)

func (r Risk) String() string {
	switch r {
	case RiskRead:
		return "read"
	case RiskWrite:
		return "write"
	case RiskExec:
		return "exec"
	case RiskNet:
		return "net"
	default:
		return "unknown"
	}
}

type Tool interface {
	Name() string
	Description() string
	Schema() json.RawMessage
	Risk() Risk
	Run(ctx context.Context, args json.RawMessage) (Result, error)
}

type Result struct {
	Output     string
	OK         bool
	Meta       map[string]string
	Provenance core.Provenance
}

// Registry 保存全局唯一名称的工具。
type Registry struct{ items map[string]Tool }

func NewRegistry() *Registry { return &Registry{items: make(map[string]Tool)} }

// Register 在重名时报错；T301 增加 schema 合法校验。
func (r *Registry) Register(t Tool) error {
	if _, exists := r.items[t.Name()]; exists {
		return ErrNotImplemented
	}
	r.items[t.Name()] = t
	return nil
}

func (r *Registry) Get(name string) (Tool, bool) {
	t, ok := r.items[name]
	return t, ok
}

func (r *Registry) List() []Tool {
	out := make([]Tool, 0, len(r.items))
	for _, t := range r.items {
		out = append(out, t)
	}
	return out
}

func (r *Registry) ByRisk(risk Risk) []Tool {
	var out []Tool
	for _, t := range r.items {
		if t.Risk() == risk {
			out = append(out, t)
		}
	}
	return out
}

// Declarations 输出可直接喂给模型的工具 JSON；T301 完善。
func (r *Registry) Declarations() json.RawMessage { return json.RawMessage("[]") }
