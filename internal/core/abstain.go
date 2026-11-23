package core

import "errors"

// Abstain 是弃权声明：明确列出已完成与未完成，是一等公民终态。
type Abstain struct {
	Reason  string
	Done    []string
	NotDone []string
	Next    string
}

func (a Abstain) Valid() error {
	if a.Reason == "" {
		return errors.New("reason is required")
	}
	if len(a.NotDone) == 0 {
		return ErrInvalidAbstain
	}
	return nil
}

// Terminal 返回弃权对应的终态。
func (a Abstain) Terminal() Terminal { return TermUntest }

// ToAbstainData 转换为事件数据。
func (a Abstain) ToAbstainData() AbstainData {
	return AbstainData{Reason: a.Reason, Done: a.Done, NotDone: a.NotDone}
}
