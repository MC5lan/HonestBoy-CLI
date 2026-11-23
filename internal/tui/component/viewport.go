package component

// Viewport 是滚动视口。T606 实现不抢滚动。
type Viewport struct{ offset, height int }

func (v *Viewport) Offset() int { return v.offset }
func (v *Viewport) GotoBottom(lineCount int) {
	if lineCount > v.height {
		v.offset = lineCount - v.height
	}
}
