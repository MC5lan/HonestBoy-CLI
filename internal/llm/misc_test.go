package llm

import (
	"context"
	"strings"
	"testing"
	"unicode/utf8"

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
	if m.Role != "user" || !strings.Contains(m.Content, "继续") || !strings.Contains(m.Content, "已输出一半") {
		t.Fatalf("resume message = %+v", m)
	}
	if m := ResumePrompt(""); strings.Contains(m.Content, "已输出的内容") {
		t.Fatalf("empty partial should not be included: %+v", m)
	}
}

// TD05 E5：模型吐出非法 UTF-8 时适配器必须净化成合法文本，不崩、不丢正常内容。
func TestChatStreamInvalidUTF8(t *testing.T) {
	srv := testserver.New(t, testserver.Script{
		InvalidUTF8: true,
		Responses:   [][]string{{chatChunk("%BAD%乱码")}},
	})
	c := NewChatClient(srv.URL, "", "m")
	chunks, errs := c.Stream(context.Background(), Request{})
	var text string
	for ch := range chunks {
		text += ch.Text
	}
	if err := <-errs; err != nil {
		t.Fatal(err)
	}
	if !utf8.ValidString(text) || !strings.Contains(text, "乱码") {
		t.Fatalf("invalid utf8 not sanitized: %q", text)
	}
}

// TD04 F25：SSE 中途断连（没有 [DONE]）不能 panic 或报错，按已收到内容收尾。
func TestChatDisconnectNoPanic(t *testing.T) {
	srv := testserver.New(t, testserver.Script{
		Responses:    [][]string{{chatChunk("前半"), chatChunk("后半")}},
		DisconnectAt: 1, // 发完第一块就断开，也不会写 [DONE]
	})
	c := NewChatClient(srv.URL, "", "m")
	chunks, errs := c.Stream(context.Background(), Request{})
	var text string
	disconnected := false
	for ch := range chunks {
		text += ch.Text
		if ch.Disconnected {
			disconnected = true
		}
	}
	if err := <-errs; err != nil {
		t.Fatalf("disconnect should be tolerated: %v", err)
	}
	if !strings.Contains(text, "前半") || strings.Contains(text, "后半") {
		t.Fatalf("unexpected partial text: %q", text)
	}
	if !disconnected {
		t.Fatal("missing Disconnected signal for engine resume")
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
