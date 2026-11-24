package store

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/MC5lan/HonestBoy-CLI/internal/core"
)

func tempStore(t *testing.T) *SessionStore {
	t.Helper()
	root := t.TempDir()
	ss := NewSessionStore(Paths{Root: root, Sessions: filepath.Join(root, "sessions"), Logs: filepath.Join(root, "logs")})
	t.Cleanup(func() { _ = ss.Unlock() })
	return ss
}

func TestSessionStoreRoundTrip(t *testing.T) {
	ss := tempStore(t)
	at := time.Now().UTC()
	s := core.Session{ID: "s_test", Cwd: "D:\\work", CreatedAt: at}
	if err := ss.Create(s); err != nil {
		t.Fatal(err)
	}
	e1 := core.Event{Seq: 1, SessionID: "s_test", Type: core.EvtTurnStart,
		Time: at, Data: core.TurnStartData{Input: "改个 bug", Model: "m"}}
	if err := ss.AppendEvent(e1); err != nil {
		t.Fatal(err)
	}
	r := core.NewReceipt("rc_1", "s_test", "t1", core.TermDone, "完成", nil, core.ZeroHash, at)
	if err := ss.AppendReceipt(r); err != nil {
		t.Fatal(err)
	}
	loaded, err := ss.Load("s_test")
	if err != nil {
		t.Fatal(err)
	}
	if loaded.ID != "s_test" || loaded.Cwd != s.Cwd || len(loaded.Events) != 1 || len(loaded.Receipts) != 1 {
		t.Fatalf("loaded = %+v", loaded)
	}
	if loaded.Seq() != 1 {
		t.Fatalf("seq = %d", loaded.Seq())
	}
	d, ok := loaded.Events[0].Data.(core.TurnStartData)
	if !ok || d.Input != "改个 bug" {
		t.Fatalf("event data = %+v", loaded.Events[0].Data)
	}
	if at2, err := core.VerifyChain(loaded.Receipts); at2 != -1 || err != nil {
		t.Fatalf("chain broken: %d %v", at2, err)
	}
}

func TestSessionStoreListAndReceipts(t *testing.T) {
	ss := tempStore(t)
	at := time.Now().UTC()
	for _, id := range []string{"s_a", "s_b"} {
		if err := ss.Create(core.Session{ID: id, Cwd: "x", CreatedAt: at}); err != nil {
			t.Fatal(err)
		}
		_ = ss.AppendEvent(core.Event{Seq: 1, SessionID: id, Type: core.EvtTurnStart,
			Time: at, Data: core.TurnStartData{Input: "任务" + id, Model: "m"}})
		term := core.TermDone
		if id == "s_b" {
			term = core.TermDenied
		}
		r := core.NewReceipt("rc_"+id, id, "t1", term, "claim", nil, core.ZeroHash, at)
		if err := ss.AppendReceipt(r); err != nil {
			t.Fatal(err)
		}
	}
	metas, err := ss.List()
	if err != nil || len(metas) != 2 {
		t.Fatalf("list = %+v, %v", metas, err)
	}
	if metas[0].Terminal == "" || metas[0].InputSummary == "" {
		t.Fatalf("meta missing fields: %+v", metas[0])
	}
	all, err := ss.AllReceipts()
	if err != nil || len(all) != 2 {
		t.Fatalf("all receipts = %d, %v", len(all), err)
	}
}

func TestConfigRoundTrip(t *testing.T) {
	root := t.TempDir()
	t.Setenv("LOCALAPPDATA", root)
	c := DefaultConfig()
	c.Protocol = "responses"
	c.Model = "gpt-test"
	if err := Save(c); err != nil {
		t.Fatal(err)
	}
	got, err := Load()
	if err != nil || got.Protocol != "responses" || got.Model != "gpt-test" {
		t.Fatalf("config = %+v, %v", got, err)
	}
	if got.APIKeyEnv != "HB_API_KEY" || got.Context.BudgetRatio != 0.7 {
		t.Fatal("defaults not merged")
	}
	if got.Context.Window != 960000 {
		t.Fatalf("context window = %d, want 960000", got.Context.Window)
	}
	if got.Rigor != "strict" {
		t.Fatalf("rigor = %q, want strict (always on)", got.Rigor)
	}
}
