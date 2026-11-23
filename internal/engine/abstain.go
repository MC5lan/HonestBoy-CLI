package engine

import "github.com/MC5lan/HonestBoy-CLI/internal/core"

// ApplyAbstain 校验并落盘弃权。T412 实现。
func ApplyAbstain(a core.Abstain) (core.Terminal, error) {
	if err := a.Valid(); err != nil {
		return core.TermFailed, err
	}
	return core.TermUntest, nil
}
