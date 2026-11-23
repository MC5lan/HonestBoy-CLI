package engine

import (
	"os"
	"strings"
	"testing"

	"github.com/MC5lan/HonestBoy-CLI/internal/core"
	"github.com/MC5lan/HonestBoy-CLI/internal/llm"
)

func TestPlanModeFiltersWriteTools(t *testing.T) {
	registry := newTestEngine(t, &scriptedClient{}).registry
	all := toolSpecs(registry)
	plan := planToolSpecs(registry)
	if len(plan) >= len(all) {
		t.Fatalf("plan tools not filtered: %d/%d", len(plan), len(all))
	}
	allowed := map[string]bool{}
	for _, s := range plan {
		allowed[s.Name] = true
	}
	for _, name := range []string{"write_file", "edit_file", "apply_patch", "shell"} {
		if allowed[name] {
			t.Fatalf("%s should not be in plan mode", name)
		}
	}
	for _, name := range []string{"read_file", "list_dir", "search", "verify_build"} {
		if !allowed[name] {
			t.Fatalf("%s should be available in plan mode", name)
		}
	}
}

func TestPlanModeProducesHaltReceipt(t *testing.T) {
	client := &scriptedClient{responses: [][]llm.Chunk{
		{{Text: "计划：1. 读 a.go 2. 修改函数 3. 跑测试验证"}},
	}}
	eng := newTestEngine(t, client)
	eng.cfg.PlanMode = true
	rs := runEngine(t, eng, "重构")
	if rs.terminal != core.TermHalt {
		t.Fatalf("terminal = %s, want halt", rs.terminal)
	}
	if countEvents(rs, core.EvtGateAllow) != 0 || countEvents(rs, core.EvtGateDeny) != 0 {
		t.Fatal("plan mode should not invoke Gate")
	}
	// Receipt 里保存计划文本，供 hb approve 读取。
	ss := eng.store
	entries, _ := readSessionIDs(t, ss.Paths.Sessions)
	if len(entries) == 0 {
		t.Fatal("no session file")
	}
	sess, err := ss.Load(entries[0])
	if err != nil || len(sess.Receipts) != 1 {
		t.Fatalf("receipts = %d, %v", len(sess.Receipts), err)
	}
	plan, ok := PlanApproved(sess.Receipts[0])
	if !ok || !strings.Contains(plan, "计划") {
		t.Fatalf("plan receipt = %q", plan)
	}
}

func TestWithPlanRules(t *testing.T) {
	out := WithPlanRules("base")
	if !strings.Contains(out, "计划模式") || !strings.Contains(out, "base") {
		t.Fatalf("rules = %q", out)
	}
}

func readSessionIDs(t *testing.T, dir string) ([]string, error) {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var out []string
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".jsonl") {
			out = append(out, strings.TrimSuffix(e.Name(), ".jsonl"))
		}
	}
	return out, nil
}
