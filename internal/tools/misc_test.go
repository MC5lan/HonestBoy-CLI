package tools

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func TestCompressPreservesCriticalLines(t *testing.T) {
	input := strings.Join([]string{
		"a", "a", "a", "a", "a",
		"", "", "",
		"ERROR: something broke",
		"--- FAIL: TestA",
		"panic: boom",
	}, "\n")
	out := Compress(input)
	for _, key := range []string{"ERROR", "FAIL", "panic"} {
		if !strings.Contains(out, key) {
			t.Fatalf("critical line %q removed: %q", key, out)
		}
	}
	if !strings.Contains(out, "[x5]") {
		t.Fatalf("repeat compression failed: %q", out)
	}
	if len(out) >= len(input) {
		t.Fatalf("no compression: %d -> %d", len(input), len(out))
	}
}

func TestCompressProgressBar(t *testing.T) {
	out := Compress("10%\r50%\r100% done")
	if strings.Contains(out, "\r") || !strings.Contains(out, "100%") {
		t.Fatalf("progress compression failed: %q", out)
	}
}

func TestGuard(t *testing.T) {
	g := NewReadGuard()
	if err := g.Check("a", []byte("x")); !errors.Is(err, ErrNeedsReadFirst) {
		t.Fatalf("unread check = %v", err)
	}
	g.MarkRead("a", []byte("x"))
	if !g.HasRead("a") {
		t.Fatal("HasRead false")
	}
	if err := g.Check("a", []byte("x")); err != nil {
		t.Fatalf("valid check: %v", err)
	}
	if err := g.Check("a", []byte("changed")); !errors.Is(err, ErrChangedSinceRead) {
		t.Fatalf("external change check = %v", err)
	}
}

func TestRegistry(t *testing.T) {
	sb, _ := newTempSandbox(t)
	r := NewRegistry()
	if err := r.Register(NewReadFileTool(sb, NewReadGuard())); err != nil {
		t.Fatal(err)
	}
	if err := r.Register(NewReadFileTool(sb, NewReadGuard())); err == nil {
		t.Fatal("duplicate name should fail")
	}
	if got, ok := r.Get("read_file"); !ok || got.Name() != "read_file" {
		t.Fatalf("Get = %v", got)
	}
	if _, ok := r.Get("missing"); ok {
		t.Fatal("missing tool found")
	}
	if len(r.List()) != 1 || len(r.ByRisk(RiskRead)) != 1 || len(r.ByRisk(RiskExec)) != 0 {
		t.Fatal("risk/list filtering failed")
	}
	var decls []ToolDecl
	if err := json.Unmarshal(r.Declarations(), &decls); err != nil {
		t.Fatalf("declarations: %v", err)
	}
	if len(decls) != 1 || decls[0].Name != "read_file" || len(decls[0].Parameters) == 0 {
		t.Fatalf("declarations = %+v", decls)
	}
}

func TestRegistryRejectsInvalidSchema(t *testing.T) {
	r := NewRegistry()
	bad := &stubTool{name: "bad", schema: json.RawMessage(`{"type":"array"}`)}
	if err := r.Register(bad); !errors.Is(err, ErrInvalidToolSchema) {
		t.Fatalf("bad schema err = %v", err)
	}
	dup := &stubTool{name: "x", schema: json.RawMessage(`{"type":"object"}`)}
	if err := r.Register(dup); err != nil {
		t.Fatal(err)
	}
	if err := r.Register(dup); !errors.Is(err, ErrDuplicateTool) {
		t.Fatalf("duplicate err = %v", err)
	}
}

type stubTool struct {
	name   string
	schema json.RawMessage
}

func (s *stubTool) Name() string            { return s.name }
func (s *stubTool) Description() string     { return "stub" }
func (s *stubTool) Schema() json.RawMessage { return s.schema }
func (s *stubTool) Risk() Risk              { return RiskRead }
func (s *stubTool) Run(context.Context, json.RawMessage) (Result, error) {
	return Result{OK: true}, nil
}

func TestToolSchemasAndRisk(t *testing.T) {
	sb, _ := newTempSandbox(t)
	guard := NewReadGuard()
	tools := []Tool{
		NewReadFileTool(sb, NewReadGuard()), NewWriteFileTool(sb, guard), NewListDirTool(sb),
		NewEditFileTool(sb, guard), NewShellTool(sb, "cmd"), NewSearchTool(sb),
	}
	for _, tool := range tools {
		if tool.Name() == "" || tool.Description() == "" {
			t.Fatalf("incomplete tool: %+v", tool)
		}
		var schema map[string]any
		if err := json.Unmarshal(tool.Schema(), &schema); err != nil {
			t.Fatalf("%s bad schema: %v", tool.Name(), err)
		}
		if schema["type"] != "object" {
			t.Fatalf("%s schema type = %v", tool.Name(), schema["type"])
		}
	}
	if tools[4].Risk() != RiskExec || tools[1].Risk() != RiskWrite || tools[5].Risk() != RiskRead {
		t.Fatal("risk labels wrong")
	}
}

func TestSearchTool(t *testing.T) {
	sb, _ := newTempSandbox(t)
	ctx := context.Background()
	if w, err := (NewWriteFileTool(sb, NewReadGuard())).Run(ctx, mustArgs(t, map[string]any{
		"path": "a.go", "content": "package pkg\nfunc Hello() {}\n",
	})); err != nil || !w.OK {
		t.Fatalf("write: %+v %v", w, err)
	}
	r, err := NewSearchTool(sb).Run(ctx, mustArgs(t, map[string]any{"pattern": "Hello"}))
	if err != nil || !r.OK || !strings.Contains(r.Output, "a.go") || !strings.Contains(r.Output, "Hello") {
		t.Fatalf("search: %+v %v", r, err)
	}
	if r.Provenance.String() != "inferred" {
		t.Fatalf("search provenance = %s", r.Provenance)
	}
	r, _ = NewSearchTool(sb).Run(ctx, mustArgs(t, map[string]any{"pattern": "[invalid"}))
	if r.OK {
		t.Fatal("invalid regex should fail")
	}
}
