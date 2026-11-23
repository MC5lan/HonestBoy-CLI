package cli

import "errors"

// ExitCodeOf 把错误映射到稳定退出码（见 docs/exit-codes.md）。
func ExitCodeOf(err error) int {
	if err == nil {
		return 0
	}
	var te *TerminalError
	if errors.As(err, &te) {
		return te.Terminal.ExitCode()
	}
	switch {
	case errors.Is(err, ErrUsage):
		return 2
	case errors.Is(err, ErrEnvironment):
		return 6
	case errors.Is(err, ErrTimeout):
		return 7
	case errors.Is(err, ErrInterrupted):
		return 130
	case errors.Is(err, ErrChainBroken):
		return 5
	default:
		return 1
	}
}
