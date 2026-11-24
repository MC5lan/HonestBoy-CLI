package engine

import (
	"testing"

	"github.com/MC5lan/HonestBoy-CLI/internal/core"
	"github.com/MC5lan/HonestBoy-CLI/internal/llm"
)

// 200 个连续回合不 panic、状态机始终回到终态。长跑只覆盖纯问答路径，
// 工具/沙箱的 30 场景演练见 docs 验收表。
func TestTwoHundredTurnsNoPanic(t *testing.T) {
	const turns = 200
	responses := make([][]llm.Chunk, turns)
	for i := range responses {
		responses[i] = []llm.Chunk{{Text: "只回答"}}
	}
	eng := newTestEngine(t, &scriptedClient{responses: responses})
	for i := 0; i < turns; i++ {
		rs := runEngine(t, eng, "连续回合")
		if rs.terminal != core.TermUntest {
			t.Fatalf("turn %d terminal = %s", i, rs.terminal)
		}
	}
}
