package engine

// Interrupt 管理中断流程。T803 实现取消与等待。
type Interrupt struct{ cancelled bool }

func (i *Interrupt) Cancel()         { i.cancelled = true }
func (i *Interrupt) Cancelled() bool { return i.cancelled }
