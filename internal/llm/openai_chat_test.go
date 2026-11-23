package llm

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/MC5lan/HonestBoy-CLI/internal/llm/testserver"
)

func chatChunk(text string) string {
	b, _ := json.Marshal(map[string]any{
		"choices": []any{map[string]any{"delta": map[string]any{"content": text}, "finish_reason": nil}},
	})
	return string(b)
}

func toolChunk(index int, id, name, args string) string {
	tc := map[string]any{"index": index}
	if id != "" {
		tc["id"] = id
	}
	if name != "" {
		tc["function"] = map[string]any{"name": name, "arguments": args}
	} else if args != "" {
		tc["function"] = map[string]any{"arguments": args}
	}
	b, _ := json.Marshal(map[string]any{
		"choices": []any{map[string]any{"delta": map[string]any{"tool_calls": []any{tc}}}},
	})
	return string(b)
}

func usageChunk(in, out int) string {
	b, _ := json.Marshal(map[string]any{
		"choices": []any{},
		"usage":   map[string]any{"prompt_tokens": in, "completion_tokens": out},
	})
	return string(b)
}

func TestChatStreamText(t *testing.T) {
	srv := testserver.New(t, testserver.Script{Responses: [][]string{
		{chatChunk("你好"), chatChunk("，世界"), usageChunk(10, 3)},
	}})
	c := NewChatClient(srv.URL, "sk-test", "gpt-test")
	chunks, errs := c.Stream(context.Background(), Request{Messages: []Message{{Role: "user", Content: "hi"}}})

	var text string
	var usage *Usage
	done := false
	for chunk := range chunks {
		text += chunk.Text
		if chunk.Usage != nil {
			usage = chunk.Usage
		}
		if chunk.Done {
			done = true
		}
	}
	if err := <-errs; err != nil {
		t.Fatalf("stream error: %v", err)
	}
	if text != "你好，世界" || !done {
		t.Fatalf("text=%q done=%v", text, done)
	}
	if usage == nil || usage.In != 10 || usage.Out != 3 {
		t.Fatalf("usage = %+v", usage)
	}
}

func TestChatStreamToolCallAssembly(t *testing.T) {
	srv := testserver.New(t, testserver.Script{Responses: [][]string{
		{
			toolChunk(0, "call_0", "read_file", `{"path":`),
			toolChunk(1, "call_1", "search", `{"pattern":`),
			toolChunk(2, "call_2", "shell", `{"q":`),
			toolChunk(0, "", "", `"a.go"}`),
			toolChunk(1, "", "", `"x"}`),
			toolChunk(2, "", "", `1}`),
		},
	}})
	c := NewChatClient(srv.URL, "", "m")
	chunks, errs := c.Stream(context.Background(), Request{})
	var calls []*ToolCall
	for chunk := range chunks {
		if chunk.Call != nil {
			calls = append(calls, chunk.Call)
		}
	}
	if err := <-errs; err != nil {
		t.Fatalf("stream error: %v", err)
	}
	if len(calls) != 3 {
		t.Fatalf("calls = %d", len(calls))
	}
	wantNames := []string{"read_file", "search", "shell"}
	wantArgs := []string{`{"path":"a.go"}`, `{"pattern":"x"}`, `{"q":1}`}
	for i, call := range calls {
		if call.Name != wantNames[i] || call.Args != wantArgs[i] || call.ID != "call_"+string(rune('0'+i)) {
			t.Fatalf("call %d = %+v", i, call)
		}
	}
}

func TestChatStreamHTTPError(t *testing.T) {
	srv := testserver.New(t, testserver.Script{StatusCodes: []int{429}, Responses: [][]string{{chatChunk("x")}}})
	c := NewChatClient(srv.URL, "", "m")
	chunks, errs := c.Stream(context.Background(), Request{})
	for range chunks {
	}
	err := <-errs
	var re *RetryableError
	if !errors.As(err, &re) || re.Status != 429 || !re.Retryable {
		t.Fatalf("want retryable 429, got %v", err)
	}
}

func TestChatStreamInvalidJSONSkipped(t *testing.T) {
	srv := testserver.New(t, testserver.Script{Responses: [][]string{{"not-json", chatChunk("ok")}}})
	c := NewChatClient(srv.URL, "", "m")
	chunks, errs := c.Stream(context.Background(), Request{})
	var text string
	for chunk := range chunks {
		text += chunk.Text
	}
	if err := <-errs; err != nil {
		t.Fatalf("stream error: %v", err)
	}
	if text != "ok" {
		t.Fatalf("text = %q", text)
	}
}
