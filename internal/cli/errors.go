package cli

import (
	"errors"

	"github.com/MC5lan/HonestBoy-CLI/internal/core"
)

var (
	ErrUsage          = errors.New("usage error")
	ErrEnvironment    = errors.New("environment error")
	ErrTimeout        = errors.New("timeout")
	ErrInterrupted    = errors.New("interrupted")
	ErrChainBroken    = errors.New("evidence chain broken")
	ErrNotImplemented = errors.New("command not implemented yet")
	// ErrInteractive 由 cmd 层捕获并启动 TUI；cli 包本身不依赖 tui。
	ErrInteractive = errors.New("interactive mode requested")
)

// InteractiveRequest 携带恢复的会话 ID，cmd 层据此装配 TUI。
type InteractiveRequest struct{ ResumeID string }

func (e *InteractiveRequest) Error() string        { return "interactive mode requested" }
func (e *InteractiveRequest) Is(target error) bool { return target == ErrInteractive }

// TerminalError 把回合终态映射到退出码（0/3/4/1）。
type TerminalError struct{ Terminal core.Terminal }

func (e *TerminalError) Error() string { return "terminal: " + e.Terminal.String() }
