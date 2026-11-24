package tui

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// brailleFrames 是 Codex 同款的 braille spinner 帧序列。
var brailleFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

type spinnerTickMsg time.Time
type noticeTickMsg struct{}

// spinnerTick 每 100ms 触发一帧；只在忙碌时调度，空闲时不空转。
func spinnerTick() tea.Cmd {
	return tea.Tick(100*time.Millisecond, func(t time.Time) tea.Msg {
		return spinnerTickMsg(t)
	})
}

func noticeTick() tea.Cmd {
	return tea.Tick(3*time.Second, func(time.Time) tea.Msg { return noticeTickMsg{} })
}

func (m *Model) spinnerFrame() string {
	if m.state != stStreaming && m.state != stToolRunning {
		return "•"
	}
	return brailleFrames[m.spinnerIdx%len(brailleFrames)]
}
