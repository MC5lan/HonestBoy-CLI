package core

// TurnState 是回合状态机的状态。
type TurnState uint8

const (
	StIdle TurnState = iota
	StThinking
	StToolPending
	StToolRunning
	StJudging
	StClosed
)

func (s TurnState) String() string {
	switch s {
	case StThinking:
		return "thinking"
	case StToolPending:
		return "tool_pending"
	case StToolRunning:
		return "tool_running"
	case StJudging:
		return "judging"
	case StClosed:
		return "closed"
	default:
		return "idle"
	}
}

// Turn 表示一个回合。
type Turn struct {
	ID       string
	State    TurnState
	Terminal Terminal
	Claim    string
}

// 转移表（硬约定，写在代码里）：
//
//	Idle        -> Thinking
//	Thinking    -> ToolPending | Judging | Closed
//	ToolPending -> ToolRunning | Thinking
//	ToolRunning -> Thinking | Closed
//	Judging     -> Closed
//	Closed      -> (无)
//
// 没有 Thinking -> Closed(terminal=done) 的边：完成必须经过 Judging。
var allowedTransitions = map[TurnState]map[TurnState]bool{
	StIdle:        {StThinking: true},
	StThinking:    {StToolPending: true, StJudging: true, StClosed: true},
	StToolPending: {StToolRunning: true, StThinking: true},
	StToolRunning: {StThinking: true, StClosed: true},
	StJudging:     {StClosed: true},
	StClosed:      {},
}

// CanTransition 只查询转移表，不改变状态。
func (t *Turn) CanTransition(to TurnState) bool { return allowedTransitions[t.State][to] }

// Transition 按转移表推进状态；非法边返回 ErrIllegalTransition。
func (t *Turn) Transition(to TurnState) error {
	if !t.CanTransition(to) {
		return ErrIllegalTransition
	}
	if to == StClosed && t.State != StJudging && t.Terminal == TermDone {
		return ErrIllegalTransition
	}
	t.State = to
	return nil
}
