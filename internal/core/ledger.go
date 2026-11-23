package core

import "time"

// LedgerEntry 记录一次失败；Why 必须是验证器原文摘要，不许模型改写。
type LedgerEntry struct {
	TurnID  string    `json:"turn_id"`
	Attempt int       `json:"attempt"`
	What    string    `json:"what"`
	Why     string    `json:"why"`
	At      time.Time `json:"at"`
}

// Ledger 是失败账本，可序列化进会话文件并回放重建。
type Ledger struct{ entries []LedgerEntry }

func (l *Ledger) Record(e LedgerEntry) {
	if e.At.IsZero() {
		e.At = time.Now().UTC()
	}
	if e.Attempt == 0 {
		e.Attempt = len(l.entries) + 1
	}
	l.entries = append(l.entries, e)
}

// SameFailureRepeated 统计末尾连续的相同 (What, Why) 次数。
func (l *Ledger) SameFailureRepeated(what, why string) int {
	count := 0
	for i := len(l.entries) - 1; i >= 0; i-- {
		if l.entries[i].What == what && l.entries[i].Why == why {
			count++
			continue
		}
		break
	}
	return count
}

// Entries 返回账本副本（含时间），供序列化与回放。
func (l *Ledger) Entries() []LedgerEntry { return append([]LedgerEntry(nil), l.entries...) }

// Len 返回账本记录数。
func (l *Ledger) Len() int { return len(l.entries) }
