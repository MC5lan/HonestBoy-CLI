package engine

// OutsideAccess 区分“用户要求”与“模型自己想”的沙箱外访问。T807 实现。
type OutsideAccess struct{ userInput string }

func NewOutsideAccess(input string) *OutsideAccess { return &OutsideAccess{userInput: input} }

func (o *OutsideAccess) Allowed(path string) bool { return false }
