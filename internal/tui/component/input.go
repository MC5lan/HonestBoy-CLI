package component

// Input 是多行输入框。T607/T810 实现历史、粘贴折叠与按键。
type Input struct{ value string }

func (i *Input) Value() string     { return i.value }
func (i *Input) SetValue(v string) { i.value = v }
