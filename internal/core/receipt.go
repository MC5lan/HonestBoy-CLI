package core

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"
	"time"
)

// ZeroHash 是哈希链首条的 PrevHash。
const ZeroHash = "0000000000000000000000000000000000000000000000000000000000000000"

// Receipt 是一次回合的证据记录，带哈希链。
type Receipt struct {
	ID        string     `json:"id"`
	SessionID string     `json:"session_id"`
	TurnID    string     `json:"turn_id"`
	Terminal  Terminal   `json:"terminal"`
	Claim     string     `json:"claim"`
	Evidence  []Evidence `json:"evidence"`
	PrevHash  string     `json:"prev_hash"`
	Hash      string     `json:"hash"`
	CreatedAt time.Time  `json:"created_at"`
}

// ComputeHash 用规范化 JSON 计算哈希：
// 字段顺序由结构体声明固定，时间用 RFC3339Nano UTC，Evidence 按 ID 排序，
// 且排除 Hash 字段自身。
func (r *Receipt) ComputeHash() string {
	copyReceipt := *r
	copyReceipt.Hash = ""
	if r.Evidence != nil {
		ev := append([]Evidence(nil), r.Evidence...)
		sort.Slice(ev, func(i, j int) bool { return ev[i].ID < ev[j].ID })
		copyReceipt.Evidence = ev
	}
	b, err := json.Marshal(&copyReceipt)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// VerifyChain 逐条重算哈希链，返回首个断裂位置（无断裂返回 -1）。
func VerifyChain(rs []Receipt) (brokenAt int, err error) {
	for i := range rs {
		wantPrev := ZeroHash
		if i > 0 {
			wantPrev = rs[i-1].Hash
		}
		if rs[i].PrevHash != wantPrev || rs[i].Hash != rs[i].ComputeHash() {
			return i, ErrChainBroken
		}
		// 证据摘要被事后替换（原文与哈希不符）同样算链损坏。
		for _, e := range rs[i].Evidence {
			if !e.VerifyRawHash() {
				return i, ErrChainBroken
			}
		}
	}
	return -1, nil
}

// NewReceipt 创建一条记录并计算哈希；caller 需设置 PrevHash。
func NewReceipt(id, sessionID, turnID string, term Terminal, claim string, ev []Evidence, prev string, at time.Time) Receipt {
	if at.IsZero() {
		at = time.Now().UTC()
	} else {
		at = at.UTC()
	}
	r := Receipt{
		ID: id, SessionID: sessionID, TurnID: turnID, Terminal: term,
		Claim: claim, Evidence: append([]Evidence(nil), ev...),
		PrevHash: prev, CreatedAt: at,
	}
	r.Hash = r.ComputeHash()
	return r
}
