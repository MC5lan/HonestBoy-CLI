package core

// Claim 是模型申请的完成声明。
// Targets 由 engine 从本回合工具调用自动提取，Req 由默认策略推导；
// 两者都不信任模型自报。
type Claim struct {
	ID           string
	Text         string
	Targets      []string
	Req          Requirement // 由默认策略推导，engine 覆盖模型自报值
	SelfReported Requirement // 模型自报要求，仅记录供审计，不参与判定
}

// NewClaim 构造完成申请：Req 一律由改动类型推导，模型无法降低要求。
func NewClaim(text string, changes []Change) Claim {
	return Claim{Text: text, Req: RequiredFor(changes)}
}

// WithTargets 返回附带目标的副本（engine 自动提取目标后调用）。
func (c Claim) WithTargets(targets []string) Claim {
	c.Targets = append([]string(nil), targets...)
	return c
}

// WithSelfReported 记录模型自报要求；Req 不会被它覆盖。
func (c Claim) WithSelfReported(req Requirement) Claim {
	c.SelfReported = req
	return c
}
