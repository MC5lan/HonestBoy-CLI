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

func TestChatReasoningContent(t *testing.T) {
	reasoning := map[string]any{
		"choices": []any{map[string]any{"delta": map[string]any{"reasoning_content": "先想一步"}, "finish_reason": nil}},
	}
	rb, _ := json.Marshal(reasoning)
	srv := testserver.New(t, testserver.Script{Responses: [][]string{
		{string(rb), chatChunk("答案")},
	}})
	c := NewChatClient(srv.URL, "sk-test", "deepseek-flash")
	chunks, errs := c.Stream(context.Background(), Request{Messages: []Message{{Role: "user", Content: "hi"}}})
	var think, text string
	for chunk := range chunks {
		think += chunk.Reasoning
		text += chunk.Text
	}
	if err := <-errs; err != nil {
		t.Fatal(err)
	}
	if think != "先想一步" || text != "答案" {
		t.Fatalf("reasoning=%q text=%q", think, text)
	}
}

func TestChatTruncatedFinish(t *testing.T) {
	stop := "length"
	raw, _ := json.Marshal(map[string]any{
		"choices": []any{map[string]any{"delta": map[string]any{}, "finish_reason": stop}},
	})
	srv := testserver.New(t, testserver.Script{Responses: [][]string{{string(raw)}}})
	c := NewChatClient(srv.URL, "sk-test", "deepseek-flash")
	chunks, errs := c.Stream(context.Background(), Request{Messages: []Message{{Role: "user", Content: "hi"}}})
	truncated := false
	for chunk := range chunks {
		if chunk.Truncated {
			truncated = true
		}
	}
	if err := <-errs; err != nil {
		t.Fatal(err)
	}
	if !truncated {
		t.Fatal("finish_reason=length should mark the chunk truncated")
	}
}

// 带 tools 的后续请求必须回传 reasoning_content（DeepSeek 思考模式要求）。
func TestWireMessagesCarryReasoning(t *testing.T) {
	wire := toWireMessages([]Message{{
		Role: "assistant", Content: "", Reasoning: "上一轮推理",
		Calls: []ToolCall{{ID: "c1", Name: "read_file", Args: `{"path":"a"}`}},
	}})
	if len(wire) != 1 {
		t.Fatalf("wire = %v", wire)
	}
	if got := wire[0]["reasoning_content"]; got != "上一轮推理" {
		t.Fatalf("reasoning_content = %v", got)
	}
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
