package tui

import (
	"github.com/MC5lan/HonestBoy-CLI/internal/core"
	tea "github.com/charmbracelet/bubbletea"
)

// runState 是一次引擎运行的句柄。runID 用来丢弃已中断/过期回合的事件。
type runState struct {
	id   uint64
	errs <-chan error
}

type eventMsg struct {
	runID uint64
	ev    core.Event
	ch    <-chan core.Event
}

type streamClosedMsg struct {
	runID uint64
}

// waitForEvent 把事件 channel 转成 tea.Cmd；channel 关闭时发带 runID 的关闭消息。
func waitForEvent(runID uint64, ch <-chan core.Event) tea.Cmd {
	if ch == nil {
		return nil
	}
	return func() tea.Msg {
		ev, ok := <-ch
		if !ok {
			return streamClosedMsg{runID: runID}
		}
		return eventMsg{runID: runID, ev: ev, ch: ch}
	}
}
