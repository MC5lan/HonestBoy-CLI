package tui

// pitchFrame 返回当前球场动画帧：射门时用 kickFrame，否则用递减的 pitchTick。
func (m *Model) pitchFrame() int {
	if m.kickFrame > 0 {
		return m.kickFrame
	}
	return m.pitchTick
}

// startKick 在任务成功时触发射门动画（球从脚下飞向右门）。
func (m *Model) startKick() {
	m.kickFrame = 1
	m.pitchTick = 0
}
