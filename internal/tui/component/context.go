package component

// ContextMeter 展示上下文占用。T806 实现。
type ContextMeter struct{ Percent int }

func (c *ContextMeter) Render() string { return "" }
