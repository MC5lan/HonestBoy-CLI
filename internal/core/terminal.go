package core

// Terminal 表示回合终态。
type Terminal string

const (
	TermDone   Terminal = "done"
	TermUntest Terminal = "untested"
	TermDenied Terminal = "denied"
	TermHalt   Terminal = "halt"
	TermFailed Terminal = "failed"
)

func (t Terminal) ExitCode() int {
	switch t {
	case TermDone:
		return 0
	case TermUntest:
		return 3
	case TermDenied:
		return 4
	default:
		return 1
	}
}

func (t Terminal) IsSuccess() bool { return t == TermDone }

func (t Terminal) String() string { return string(t) }
