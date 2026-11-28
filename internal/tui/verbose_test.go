package tui

import (
	"strings"
	"testing"

	"github.com/MC5lan/HonestBoy-CLI/internal/core"
)

func seedTwoTools(m *Model) {
	m.applyEvent(core.Event{Type: core.EvtToolRequest, Data: core.ToolRequestData{
		CallID: "c1", Tool: "read_file", Args: `{"path":"a.go"}`,
	}})
	m.applyEvent(core.Event{Type: core.EvtToolEnd, Data: core.ToolEndData{
		CallID: "c1", OK: true, Output: "a",
	}})
	m.applyEvent(core.Event{Type: core.EvtToolRequest, Data: core.ToolRequestData{
		CallID: "c2", Tool: "shell", Args: `{"command":"go test"}`,
	}})
	m.applyEvent(core.Event{Type: core.EvtToolEnd, Data: core.ToolEndData{
		CallID: "c2", OK: false, Output: "fail",
	}})
}

func toolBlocks(m *Model) []*block {
	var out []*block
	for i := range m.blocks {
		if m.blocks[i].kind == blkTool {
			out = append(out, &m.blocks[i])
		}
	}
	return out
}

func TestVerboseSelectCard(t *testing.T) {
	m := NewModel(Config{Theme: "none"})
	m.setSize(100, 40)
	seedTwoTools(m)
	blocks := toolBlocks(m)
	if len(blocks) != 2 {
		t.Fatalf("tools = %d, want 2", len(blocks))
	}
	if !blocks[0].collapsed {
		t.Fatal("successful #1 should start collapsed")
	}
	if blocks[1].collapsed {
		t.Fatal("failed #2 should start expanded")
	}
	m.runSlash("/verbose", "/verbose 1", []string{"1"})
	if blocks[0].collapsed {
		t.Fatal("#1 should be expanded")
	}
	if blocks[1].collapsed {
		t.Fatal("#2 should stay expanded")
	}
	m.runSlash("/verbose", "/verbose 1", []string{"1"})
	if !blocks[0].collapsed {
		t.Fatal("#1 should be collapsed again")
	}
}

func TestVerboseList(t *testing.T) {
	m := NewModel(Config{Theme: "none"})
	m.setSize(100, 40)
	seedTwoTools(m)
	m.runSlash("/verbose", "/verbose list", []string{"list"})
	out := m.blocks[len(m.blocks)-1].content
	for _, want := range []string{"#1", "#2", "读取文件", "执行命令"} {
		if !strings.Contains(out, want) {
			t.Fatalf("list missing %q:\n%s", want, out)
		}
	}
}

func TestVerboseInvalidIndex(t *testing.T) {
	m := NewModel(Config{Theme: "none"})
	m.setSize(100, 40)
	seedTwoTools(m)
	m.runSlash("/verbose", "/verbose 99", []string{"99"})
	if !strings.Contains(m.hint, "没有第 99") {
		t.Fatalf("hint = %q", m.hint)
	}
}
