package llm

// Usage 是一次模型的 token 用量与成本（微美元）。
type Usage struct {
	In         int   `json:"tokens_in"`
	Out        int   `json:"tokens_out"`
	Cached     int   `json:"tokens_cached"`
	CostMicros int64 `json:"cost_micros"`
}

// Cost 按“每百万 token 的微美元”单价计算成本，全程整数避免浮点误差。
func (u Usage) Cost(pricePerMillionMicros int64) int64 {
	if pricePerMillionMicros <= 0 {
		return 0
	}
	billable := int64(u.In) + int64(u.Out)
	return billable * pricePerMillionMicros / 1_000_000
}

// Add 合并两次用量（用于断流续写拼接）。
func (u *Usage) Add(other Usage) {
	u.In += other.In
	u.Out += other.Out
	u.Cached += other.Cached
	u.CostMicros += other.CostMicros
}
