package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

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

// Register 校验名称、重名与 JSON Schema；Schema 必须是 object。
func (r *Registry) Register(t Tool) error {
	if t == nil {
		return fmt.Errorf("%w: nil tool", ErrInvalidToolName)
	}
	name := t.Name()
	if strings.TrimSpace(name) == "" || name != strings.TrimSpace(name) {
		return fmt.Errorf("%w: %q", ErrInvalidToolName, name)
	}
	if _, exists := r.items[name]; exists {
		return fmt.Errorf("%w: %s", ErrDuplicateTool, name)
	}
	schema := t.Schema()
	if !json.Valid(schema) {
		return fmt.Errorf("%w: %s schema is not valid JSON", ErrInvalidToolSchema, name)
	}
	var obj map[string]any
	if err := json.Unmarshal(schema, &obj); err != nil {
		return fmt.Errorf("%w: %s: %v", ErrInvalidToolSchema, name, err)
	}
	if _, ok := obj["type"]; !ok {
		return fmt.Errorf("%w: %s missing type", ErrInvalidToolSchema, name)
	}
	if typ, _ := obj["type"].(string); typ != "object" {
		return fmt.Errorf("%w: %s type must be object", ErrInvalidToolSchema, name)
	}
	r.items[name] = t
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

// ToolDecl 是工具声明的稳定 JSON 形态（不绑定某个厂商协议）。
type ToolDecl struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Parameters  json.RawMessage `json:"parameters"`
}

// Declarations 输出按名称排序的工具声明，供协议层或外部审计使用。
func (r *Registry) Declarations() json.RawMessage {
	names := make([]string, 0, len(r.items))
	for name := range r.items {
		names = append(names, name)
	}
	sort.Strings(names)
	decls := make([]ToolDecl, 0, len(names))
	for _, name := range names {
		t := r.items[name]
		decls = append(decls, ToolDecl{Name: name, Description: t.Description(), Parameters: t.Schema()})
	}
	raw, err := json.Marshal(decls)
	if err != nil {
		return json.RawMessage("[]")
	}
	return raw
}
