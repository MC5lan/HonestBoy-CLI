package engine

import "github.com/MC5lan/HonestBoy-CLI/internal/core"

// ExtractClaim 从 request_done 调用与工具记录提取 Claim。T402 实现。
func ExtractClaim(text string, changes []core.Change) core.Claim {
	return core.Claim{Text: text, Req: core.RequiredFor(changes)}
}
