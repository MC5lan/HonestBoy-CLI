package tui

import (
	"strings"
	"testing"

	"github.com/MC5lan/HonestBoy-CLI/internal/core"
)

// View 的行数绝不能超过终端高度，否则顶部会被滚出屏幕（用户实测过的问题）。
func TestViewFitsHeight(t *testing.T) {
	for _, h := range []int{20, 24, 30, 34, 40} {
		m := NewModel(Config{Workdir: `C:\Users\dev\proj`, Model: "deepseek-chat", Theme: "none"})
		m.setSize(100, h)
		got := len(strings.Split(m.View(), "\n"))
		if got > h {
			t.Fatalf("height=%d: view has %d lines", h, got)
		}
		// 有内容和 done 终态下同样不能超。
		m.applyEvent(core.Event{Seq: 1, Type: core.EvtTurnStart, Data: core.TurnStartData{Input: "任务"}})
		m.term = core.TermDone
		got = len(strings.Split(m.View(), "\n"))
		if got > h {
			t.Fatalf("height=%d done: view has %d lines", h, got)
		}
	}
}
