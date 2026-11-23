package llm

import (
	"context"
	"strings"
	"testing"

	"github.com/MC5lan/HonestBoy-CLI/internal/llm/testserver"
)

func TestSanitizeUTF8(t *testing.T) {
	bad := string([]byte{0xff, 0xfe, 'a'})
	got := SanitizeUTF8(bad)
	if !ValidDelta(got) || !strings.Contains(got, "a") {
		t.Fatalf("sanitized = %q", got)
	}
	if !ValidDelta("正常中文") {
		t.Fatal("valid utf8 rejected")
	}
	var b strings.Builder
	AppendDelta(&b, bad)
	if !ValidDelta(b.String()) {
		t.Fatalf("AppendDelta produced invalid text: %q", b.String())
	}
}

func TestUsageCost(t *testing.T) {
	u := Usage{In: 1_000_000, Out: 500_000}
	if got := u.Cost(3_000_000); got != 4_500_000 {
		t.Fatalf("Cost = %d", got)
	}
	if got := (Usage{}).Cost(3_000_000); got != 0 {
		t.Fatalf("zero usage cost = %d", got)
	}
	u.Add(Usage{In: 10, Out: 20, Cached: 5, CostMicros: 7})
	if u.In != 1_000_010 || u.Out != 500_020 || u.Cached != 5 || u.CostMicros != 7 {
		t.Fatalf("Add = %+v", u)
	}
}

func TestToWireTools(t *testing.T) {
	wire := ToWireTools([]ToolSpec{{Name: "read_file", Description: "read", Schema: []byte(`{"type":"object"}`)}})
	if len(wire) != 1 || wire[0]["type"] != "function" {
		t.Fatalf("wire = %+v", wire)
	}
	fn := wire[0]["function"].(map[string]any)
	if fn["name"] != "read_file" || fn["description"] != "read" {
		t.Fatalf("function = %+v", fn)
	}
	empty := ToWireTools(nil)
	if len(empty) != 0 {
		t.Fatal("empty tools should stay empty")
	}
}

func TestResumePrompt(t *testing.T) {
	m := ResumePrompt("已输出一半")
	if m.Role != "user" || !strings.Contains(m.Content, "继续") {
		t.Fatalf("resume message = %+v", m)
	}
}

func TestChatClientContextCancel(t *testing.T) {
	srv := testserver.New(t, testserver.Script{Responses: [][]string{{chatChunk("x")}}})
	c := NewChatClient(srv.URL, "", "m")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	chunks, errs := c.Stream(ctx, Request{})
	for range chunks {
	}
	if err := <-errs; err == nil {
		t.Fatal("expected context error")
	}
}
