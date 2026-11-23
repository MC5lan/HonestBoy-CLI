package core

// LoopGuard 防止模型原地打转。
type LoopGuard struct{}

const (
	warnRepeatedTwice = "同一失败重复两次，请注意"
	haltRepeated3x    = "同一失败重复三次"
	haltTooManyCalls  = "单回合工具调用超过 100 次"
	maxToolCalls      = 100
)

// Check 检查失败重复与工具调用次数：
// 同错 2 次返回警告（halt=false），3 次强制 halt；
// 单回合工具调用超过 100 次强制 halt。
// turnID 非空时只统计当前回合的失败，历史账本不阻断新回合。
func (g *LoopGuard) Check(led *Ledger, toolCalls int, turnID string) (halt bool, reason string) {
	if toolCalls > maxToolCalls {
		return true, haltTooManyCalls
	}
	if led == nil {
		return false, ""
	}
	for i := len(led.entries) - 1; i >= 0; i-- {
		e := led.entries[i]
		n := led.FailureRepeatedInTurn(e.What, e.Why, turnID)
		switch {
		case n >= 3:
			return true, haltRepeated3x
		case n == 2:
			return false, warnRepeatedTwice
		default:
			return false, ""
		}
	}
	return false, ""
}
