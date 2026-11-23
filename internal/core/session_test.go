package core

import (
	"testing"
	"time"
)

func TestSessionEmitSequence(t *testing.T) {
	s := &Session{ID: "s_test", CreatedAt: time.Now().UTC()}
	for i := 1; i <= 1000; i++ {
		e := s.Emit(EvtNotice, NoticeData{Level: "info", Text: "x"})
		if e.Seq != uint64(i) {
			t.Fatalf("seq = %d, want %d", e.Seq, i)
		}
		if e.SessionID != "s_test" || e.Type != EvtNotice {
			t.Fatalf("bad event fields: %+v", e)
		}
		if e.Time.Location() != time.UTC {
			t.Fatal("event time must be UTC")
		}
	}
	if s.Seq() != 1000 || len(s.Events) != 1000 {
		t.Fatalf("seq=%d events=%d", s.Seq(), len(s.Events))
	}
}

func TestSessionEmitTurnID(t *testing.T) {
	s := &Session{ID: "s"}
	if e := s.Emit(EvtTurnStart, nil); e.TurnID != "" {
		t.Fatal("no turn yet, TurnID should be empty")
	}
	s.Turns = append(s.Turns, &Turn{ID: "t1"})
	if e := s.Emit(EvtTextDelta, nil); e.TurnID != "t1" {
		t.Fatalf("TurnID = %q", e.TurnID)
	}
	if s.LastTurn() == nil || s.LastTurn().ID != "t1" {
		t.Fatal("LastTurn wrong")
	}
}

func TestSessionAppendReceiptChain(t *testing.T) {
	s := &Session{ID: "s"}
	at := time.Date(2026, 11, 23, 0, 0, 0, 0, time.UTC)
	r1 := NewReceipt("rc_1", "s", "t1", TermDone, "claim1", nil, ZeroHash, at)
	if err := s.AppendReceipt(r1); err != nil {
		t.Fatalf("append r1: %v", err)
	}
	if s.ChainTail() != r1.Hash {
		t.Fatal("chain tail mismatch")
	}
	r2 := NewReceipt("rc_2", "s", "t2", TermUntest, "claim2", nil, r1.Hash, at)
	if err := s.AppendReceipt(r2); err != nil {
		t.Fatalf("append r2: %v", err)
	}
	// 错误的 PrevHash（乱序/跳链）必须报错。
	bad := NewReceipt("rc_3", "s", "t3", TermDone, "x", nil, ZeroHash, at)
	if err := s.AppendReceipt(bad); err != ErrChainBroken {
		t.Fatalf("bad prev hash err = %v", err)
	}
	// 空 Hash 必须报错。
	bad.Hash = ""
	if err := s.AppendReceipt(bad); err != ErrChainBroken {
		t.Fatalf("empty hash err = %v", err)
	}
	if len(s.Receipts) != 2 {
		t.Fatalf("receipts = %d", len(s.Receipts))
	}
}
