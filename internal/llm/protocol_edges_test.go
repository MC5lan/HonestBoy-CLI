package llm

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/MC5lan/HonestBoy-CLI/internal/llm/testserver"
)

func TestToolCallIndexVariants(t *testing.T) {
	cases := []struct {
		name string
		raw  json.RawMessage
		want int
		err  bool
	}{
		{"integer", json.RawMessage(`2`), 2, false},
		{"string", json.RawMessage(`"3"`), 3, false},
		{"missing", nil, 0, true},
		{"null", json.RawMessage(`null`), 0, true},
		{"invalid", json.RawMessage(`true`), 0, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := toolCallIndex(tc.raw)
			if (err != nil) != tc.err || (!tc.err && got != tc.want) {
				t.Fatalf("toolCallIndex(%s) = %d, %v", tc.raw, got, err)
			}
		})
	}
}

func TestChatWireMessagesWithToolHistory(t *testing.T) {
	srv := testserver.New(t, testserver.Script{Responses: [][]string{{chatChunk("ok")}}})
	c := NewChatClient(srv.URL, "", "m")
	chunks, errs := c.Stream(context.Background(), Request{Messages: []Message{
		{Role: "user", Content: "read a.go"},
		{Role: "assistant", Calls: []ToolCall{{ID: "call_1", Name: "read_file", Args: `{"path":"a.go"}`}}},
		{Role: "tool", ToolID: "call_1", Content: "file content"},
	}, MaxTokens: 100, Temperature: 0.2, Stop: []string{"END"}})
	for range chunks {
	}
	if err := <-errs; err != nil {
		t.Fatal(err)
	}
	body := srv.LastRequest()
	msgs := body["messages"].([]any)
	if len(msgs) != 3 {
		t.Fatalf("messages = %v", msgs)
	}
	assistant := msgs[1].(map[string]any)
	if assistant["role"] != "assistant" || assistant["content"] != "" || assistant["tool_calls"] == nil {
		t.Fatalf("assistant = %v", assistant)
	}
	toolMsg := msgs[2].(map[string]any)
	if toolMsg["role"] != "tool" || toolMsg["tool_call_id"] != "call_1" || toolMsg["content"] != "file content" {
		t.Fatalf("tool = %v", toolMsg)
	}
	if body["max_tokens"] != float64(100) || body["temperature"] != 0.2 {
		t.Fatalf("body = %v", body)
	}
	if body["stream_options"] == nil {
		t.Fatal("stream_options missing")
	}
}

func TestResponsesAssistantPlainAndRawArgs(t *testing.T) {
	items, instructions := toResponsesInput([]Message{
		{Role: "assistant", Content: "之前说过"},
		{Role: "system", Content: "规则"},
	})
	if len(items) != 1 || items[0].(map[string]any)["role"] != "assistant" {
		t.Fatalf("items = %v", items)
	}
	if len(instructions) != 1 || instructions[0] != "规则" {
		t.Fatalf("instructions = %v", instructions)
	}
	if got := decodeResponsesArgs(json.RawMessage(`{"path":"a.go"}`)); got != `{"path":"a.go"}` {
		t.Fatalf("raw object args = %q", got)
	}
	if got := decodeResponsesArgs(nil); got != "" {
		t.Fatalf("empty args = %q", got)
	}
}

func TestResponsesArgumentsDoneOnly(t *testing.T) {
	srv := newResponsesServer(t, []string{
		rspEvent("response.output_item.added", `{"type":"response.output_item.added","item":{"type":"function_call","id":"fc_9","call_id":"call_9","name":"search"}}`),
		rspEvent("response.function_call_arguments.done", `{"type":"response.function_call_arguments.done","item_id":"fc_9","arguments":"{\"q\":\"x\"}"}`),
	}, 0)
	c := NewResponsesClient(srv.url, "sk", "m")
	chunks, errs := c.Stream(context.Background(), Request{})
	var calls []*ToolCall
	for chunk := range chunks {
		if chunk.Call != nil {
			calls = append(calls, chunk.Call)
		}
	}
	if err := <-errs; err != nil {
		t.Fatal(err)
	}
	if len(calls) != 1 || calls[0].ID != "call_9" || calls[0].Args != `{"q":"x"}` {
		t.Fatalf("calls = %+v", calls)
	}
}

func TestRetryableErrorMessage(t *testing.T) {
	err := error(&RetryableError{Status: 429, Message: "slow down"})
	if err.Error() != "slow down" {
		t.Fatalf("Error() = %q", err.Error())
	}
	if errors.Is(err, err) == false {
		t.Fatal("errors.Is self mismatch")
	}
}
