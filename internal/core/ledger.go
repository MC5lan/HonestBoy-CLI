package core

import (
	"sort"
	"strings"
	"time"
)

// LedgerEntry 记录一次失败。Why 必须是验证器原文摘要，不许模型改写。
// Tags/Files/Kind 用于历史检索。
type LedgerEntry struct {
	TurnID  string    `json:"turn_id"`
	Attempt int       `json:"attempt"`
	What    string    `json:"what"`
	Why     string    `json:"why"`
	At      time.Time `json:"at"`
	Kind    string    `json:"kind,omitempty"`
	Tags    []string  `json:"tags,omitempty"`
	Files   []string  `json:"files,omitempty"`
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

func (l *Ledger) Entries() []LedgerEntry { return append([]LedgerEntry(nil), l.entries...) }

func (l *Ledger) Len() int { return len(l.entries) }

// Query 是账本检索条件。
type Query struct {
	Kind  string
	Tags  []string
	Files []string
}

// Score 按文档 §3.5.3 加权：标签 0.5、文件 0.3、类型 0.2。
func Score(q Query, e LedgerEntry) float64 {
	tagScore := jaccard(q.Tags, e.Tags)
	fileScore := pathOverlap(q.Files, e.Files)
	kindScore := 0.0
	if q.Kind != "" && q.Kind == e.Kind {
		kindScore = 1.0
	}
	return 0.5*tagScore + 0.3*fileScore + 0.2*kindScore
}

// Search 返回得分高于 minScore 的前 topK 条，按得分降序。
func (l *Ledger) Search(q Query, topK int, minScore float64) []LedgerEntry {
	type scored struct {
		e LedgerEntry
		s float64
	}
	var list []scored
	for _, e := range l.entries {
		if s := Score(q, e); s >= minScore {
			list = append(list, scored{e, s})
		}
	}
	sort.SliceStable(list, func(i, j int) bool { return list[i].s > list[j].s })
	if topK <= 0 || topK > len(list) {
		topK = len(list)
	}
	out := make([]LedgerEntry, 0, topK)
	for i := 0; i < topK; i++ {
		out = append(out, list[i].e)
	}
	return out
}

func jaccard(a, b []string) float64 {
	if len(a) == 0 || len(b) == 0 {
		return 0
	}
	set := make(map[string]struct{}, len(a))
	for _, x := range a {
		set[strings.ToLower(x)] = struct{}{}
	}
	hit := 0
	for _, y := range b {
		if _, ok := set[strings.ToLower(y)]; ok {
			hit++
		}
	}
	union := len(a) + len(b) - hit
	if union == 0 {
		return 0
	}
	return float64(hit) / float64(union)
}

func pathOverlap(a, b []string) float64 {
	if len(a) == 0 || len(b) == 0 {
		return 0
	}
	hit := 0
	for _, x := range a {
		for _, y := range b {
			if x == y {
				hit++
				break
			}
		}
	}
	return float64(hit) / float64(len(a))
}

// RenderLedgerContext 把检索结果渲染为注入上下文的文本块（标明来源是历史）。
func RenderLedgerContext(entries []LedgerEntry) string {
	if len(entries) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("[历史失败案例 · 来自本地账本，非本轮观测]\n")
	for i, e := range entries {
		b.WriteString(itoaSimple(i+1) + ". (" + e.What + ") " + e.Why + "\n")
	}
	return b.String()
}

func itoaSimple(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [12]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}
