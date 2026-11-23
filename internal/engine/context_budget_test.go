package engine

import (
	"strings"
	"testing"

	"github.com/MC5lan/HonestBoy-CLI/internal/core"
	"github.com/MC5lan/HonestBoy-CLI/internal/llm"
)

func TestAssembleContextOrder(t *testing.T) {
	s := AssembleContext("SYSTEM", "HB内容", "ENV", "HIST", "TASK")
	parts := []string{"SYSTEM", "HB内容", "ENV", "HIST", "TASK"}
	prev := -1
	for _, p := range parts {
		i := strings.Index(s, p)
		if i < 0 || i <= prev {
			t.Fatalf("order wrong: %q in %q", p, s)
		}
		prev = i
	}
}

func TestBudgetTrimKeepsSystemRecentAndToolSummary(t *testing.T) {
	msgs := []llm.Message{{Role: "system", Content: "system prompt"}}
	for i := 0; i < 20; i++ {
		msgs = append(msgs, llm.Message{Role: "user", Content: "旧问题" + strings.Repeat("中", 1000)})
		msgs = append(msgs, llm.Message{Role: "tool", Content: strings.Repeat("工", 1000)})
	}
	msgs = append(msgs, llm.Message{Role: "user", Content: "最近问题"})
	msgs = append(msgs, llm.Message{Role: "assistant", Content: "最近回答"})

	b := Budget{Ratio: 0.7, KeepRecent: 2, WindowSize: 200}
	trimmed, removed := b.Trim(msgs, b.Limit())
	if removed == 0 {
		t.Fatal("expected compaction")
	}
	if trimmed[0].Role != "system" || trimmed[0].Content != "system prompt" {
		t.Fatal("system message lost")
	}
	if trimmed[len(trimmed)-1].Content != "最近回答" {
		t.Fatal("recent messages lost")
	}
	toolKept, truncated := false, false
	for _, m := range trimmed {
		if m.Role == "tool" {
			toolKept = true
			if len([]rune(m.Content)) <= 250 {
				truncated = true // 旧工具输出被压缩
			}
		}
	}
	if !toolKept || !truncated {
		t.Fatal("tool summaries should be kept and old ones truncated")
	}
	if !strings.Contains(WithCompactMark("sys", true), CompactMark) {
		t.Fatal("compact mark missing")
	}
}

func TestCompactionInLoop(t *testing.T) {
	// 极小窗口强制每轮裁剪，确认主循环仍能正常走到 done。
	client := &scriptedClient{responses: [][]llm.Chunk{
		{call("s1", "shell", `{"command":"echo a"}`)},
		{call("s2", "shell", `{"command":"echo b"}`)},
		{call("s3", "shell", `{"command":"echo c"}`)},
		{doneCall("d", "完成")},
	}}
	eng := newTestEngine(t, client)
	eng.cfg.ContextWindow = 120
	rs := runEngine(t, eng, "跑命令")
	if rs.terminal != "done" {
		t.Fatalf("terminal = %s", rs.terminal)
	}
	compacted := false
	for _, ev := range rs.events {
		if ev.Type == core.EvtNotice {
			if d, ok := ev.Data.(core.NoticeData); ok && strings.Contains(d.Text, "裁剪") {
				compacted = true
			}
		}
	}
	if !compacted {
		t.Fatal("no compaction notice emitted")
	}
}
