package core

import "errors"

var (
	// ErrIllegalTransition 表示回合状态机收到非法转移。
	ErrIllegalTransition = errors.New("illegal transition")
	// ErrUnknownProvenance 表示来源字符串不在四值集合内。
	ErrUnknownProvenance = errors.New("unknown provenance value")
	// ErrEmptySummary 表示证据缺少验证器原文摘要。
	ErrEmptySummary = errors.New("evidence summary is empty")
	// ErrChainBroken 表示 Receipt 哈希链断裂。
	ErrChainBroken = errors.New("receipt chain broken")
	// ErrInvalidAbstain 表示弃权声明不合法。
	ErrInvalidAbstain = errors.New("invalid abstain")
)
