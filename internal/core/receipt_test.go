package core

import (
	"testing"
	"time"
)

func TestReceiptHashStable(t *testing.T) {
	at := time.Date(2026, 11, 23, 1, 2, 3, 456789000, time.UTC)
	ev := []Evidence{
		{ID: "e2", Kind: EvTest, Summary: "ok", Provenance: ProvObserved},
		{ID: "e1", Kind: EvBuild, Summary: "ok", Provenance: ProvObserved},
	}
	r1 := NewReceipt("rc_1", "s", "t", TermDone, "完成", ev, ZeroHash, at)
	r2 := NewReceipt("rc_1", "s", "t", TermDone, "完成", ev, ZeroHash, at)
	if r1.Hash != r2.Hash {
		t.Fatalf("hash unstable: %s vs %s", r1.Hash, r2.Hash)
	}
	// 顺序不同的 Evidence 应得到相同哈希（规范序按 ID 排）。
	evReversed := []Evidence{ev[1], ev[0]}
	r3 := NewReceipt("rc_1", "s", "t", TermDone, "完成", evReversed, ZeroHash, at)
	if r1.Hash != r3.Hash {
		t.Fatal("evidence order must not change hash")
	}
	r4 := NewReceipt("rc_1", "s", "t", TermDone, "完成改动", ev, ZeroHash, at)
	if r1.Hash == r4.Hash {
		t.Fatal("one byte change must change hash")
	}
}

func TestVerifyChain(t *testing.T) {
	at := time.Now().UTC()
	r1 := NewReceipt("1", "s", "t1", TermDone, "a", nil, ZeroHash, at)
	r2 := NewReceipt("2", "s", "t2", TermDone, "b", nil, r1.Hash, at)
	r3 := NewReceipt("3", "s", "t3", TermDone, "c", nil, r2.Hash, at)
	rs := []Receipt{r1, r2, r3}
	if at, err := VerifyChain(rs); at != -1 || err != nil {
		t.Fatalf("valid chain broken at %d: %v", at, err)
	}
	tampered := append([]Receipt(nil), rs...)
	tampered[1].Claim = "篡改"
	if at, err := VerifyChain(tampered); at != 1 || err != ErrChainBroken {
		t.Fatalf("tamper not detected: at=%d err=%v", at, err)
	}
	tampered = append([]Receipt(nil), rs...)
	tampered[2].PrevHash = ZeroHash
	if at, err := VerifyChain(tampered); at != 2 || err != ErrChainBroken {
		t.Fatalf("broken prev not detected: at=%d err=%v", at, err)
	}
}
