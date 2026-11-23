package core

import "time"

// Session 落盘前在内存中的容器。
type Session struct {
	ID        string
	Cwd       string
	CreatedAt time.Time
	Events    []Event
	Turns     []*Turn
	Receipts  []Receipt
	seq       uint64
}

// Emit 是唯一产生事件的地方；seq 从 1 起单调，不经 Emit 无法前进。
func (s *Session) Emit(t EventType, data any) Event {
	s.seq++
	e := Event{Seq: s.seq, SessionID: s.ID, Type: t, Time: time.Now().UTC(), Data: data}
	if last := s.LastTurn(); last != nil {
		e.TurnID = last.ID
	}
	s.Events = append(s.Events, e)
	return e
}

func (s *Session) LastTurn() *Turn {
	if len(s.Turns) == 0 {
		return nil
	}
	return s.Turns[len(s.Turns)-1]
}

// AppendReceipt 校验 PrevHash 等于链尾 Hash 后追加。
func (s *Session) AppendReceipt(r Receipt) error {
	tail := ZeroHash
	if n := len(s.Receipts); n > 0 {
		tail = s.Receipts[n-1].Hash
	}
	if r.PrevHash != tail {
		return ErrChainBroken
	}
	if r.Hash == "" {
		return ErrChainBroken
	}
	s.Receipts = append(s.Receipts, r)
	return nil
}

// Seq 返回当前序号（供会话恢复与测试使用）。
func (s *Session) Seq() uint64 { return s.seq }

// SetSeq 从落盘事件恢复序号，保证回放后继续单调递增。
func (s *Session) SetSeq(seq uint64) { s.seq = seq }

// ChainTail 返回当前哈希链尾部。
func (s *Session) ChainTail() string {
	if n := len(s.Receipts); n > 0 {
		return s.Receipts[n-1].Hash
	}
	return ZeroHash
}
