package tui

import (
	"github.com/MC5lan/HonestBoy-CLI/internal/core"
	tea "github.com/charmbracelet/bubbletea"
)

type eventMsg core.Event
type streamClosedMsg struct{}
type tickRenderMsg struct{}

// waitForEvent 把事件 channel 转成 tea.Cmd；channel 关闭时发 streamClosedMsg。
func waitForEvent(ch <-chan core.Event) tea.Cmd {
	if ch == nil {
		return nil
	}
	return func() tea.Msg {
		ev, ok := <-ch
		if !ok {
			return streamClosedMsg{}
		}
		return eventMsg(ev)
	}
}
