package llm

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
)

type anthropicServer struct {
	url      string
	mu       sync.Mutex
	requests [][]byte
	status   int
}

func newAnthropicServer(t *testing.T, events []string, status int) *anthropicServer {
	t.Helper()
	s := &anthropicServer{status: status}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		s.mu.Lock()
		s.requests = append(s.requests, body)
		s.mu.Unlock()
		if r.URL.Path != "/v1/messages" {
			http.Error(w, "wrong path", http.StatusNotFound)
			return
		}
		if r.Header.Get("x-api-key") == "" || r.Header.Get("anthropic-version") == "" {
			http.Error(w, "missing headers", http.StatusBadRequest)
			return
		}
		if status >= 400 {
			w.WriteHeader(status)
			fmt.Fprint(w, `{"error":"scripted"}`)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		for _, e := range events {
			fmt.Fprintf(w, "%s\n\n", e)
		}
	}))
	t.Cleanup(srv.Close)
	s.url = srv.URL
	return s
}

func anthropicEvent(name, payload string) string {
	return "event: " + name + "\ndata: " + payload
}

func TestAnthropicStreamTextAndUsage(t *testing.T) {
	events := []string{
		anthropicEvent("message_start", `{"type":"message_start","message":{"usage":{"input_tokens":11}}}`),
		anthropicEvent("content_block_start", `{"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}`),
		anthropicEvent("content_block_delta", `{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"你好"}}`),
		anthropicEvent("content_block_delta", `{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"世界"}}`),
		anthropicEvent("content_block_stop", `{"type":"content_block_stop","index":0}`),
		anthropicEvent("message_delta", `{"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":7}}`),
		anthropicEvent("message_stop", `{"type":"message_stop"}`),
		anthropicEvent("ping", `{"type":"ping"}`),
	}
	srv := newAnthropicServer(t, events, 0)
	client := NewAnthropicClient(srv.url, "sk-ant-test", "claude-test")
	chunks, errs := client.Stream(context.Background(), Request{
		Messages: []Message{{Role: "system", Content: "你是 Honest Boy"}, {Role: "user", Content: "hi"}},
	})
	var text string
	var usage *Usage
	done := false
	for c := range chunks {
		text += c.Text
		if c.Usage != nil {
			usage = c.Usage
		}
		if c.Done {
			done = true
		}
	}
	if err := <-errs; err != nil {
		t.Fatalf("stream error: %v", err)
	}
	if text != "你好世界" || !done {
		t.Fatalf("text=%q done=%v", text, done)
	}
	if usage == nil || usage.In != 11 || usage.Out != 7 {
		t.Fatalf("usage = %+v", usage)
	}

	// 断言请求体：system 顶层、tools input_schema、消息格式。
	srv.mu.Lock()
	body := srv.requests[0]
	srv.mu.Unlock()
	var reqBody map[string]any
	if err := json.Unmarshal(body, &reqBody); err != nil {
		t.Fatal(err)
	}
	if reqBody["system"] != "你是 Honest Boy" {
		t.Fatalf("system = %v", reqBody["system"])
	}
	if reqBody["stream"] != true || reqBody["max_tokens"] == nil {
		t.Fatalf("bad body: %v", reqBody)
	}
	msgs := reqBody["messages"].([]any)
	if len(msgs) != 1 || msgs[0].(map[string]any)["role"] != "user" {
		t.Fatalf("messages = %v", msgs)
	}
}

func TestAnthropicStreamToolUse(t *testing.T) {
	events := []string{
		anthropicEvent("message_start", `{"type":"message_start","message":{"usage":{"input_tokens":5}}}`),
		anthropicEvent("content_block_start", `{"type":"content_block_start","index":0,"content_block":{"type":"text","text":"我看一下"}}`),
		anthropicEvent("content_block_delta", `{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"我看一下"}}`),
		anthropicEvent("content_block_stop", `{"type":"content_block_stop","index":0}`),
		anthropicEvent("content_block_start", `{"type":"content_block_start","index":1,"content_block":{"type":"tool_use","id":"toolu_1","name":"read_file","input":{}}}`),
		anthropicEvent("content_block_delta", `{"type":"content_block_delta","index":1,"delta":{"type":"input_json_delta","partial_json":"{\"path\":"}}`),
		anthropicEvent("content_block_delta", `{"type":"content_block_delta","index":1,"delta":{"type":"input_json_delta","partial_json":"\"a.go\"}"}}`),
		anthropicEvent("content_block_stop", `{"type":"content_block_stop","index":1}`),
		anthropicEvent("message_delta", `{"type":"message_delta","delta":{"stop_reason":"tool_use"},"usage":{"output_tokens":9}}`),
		anthropicEvent("message_stop", `{"type":"message_stop"}`),
	}
	srv := newAnthropicServer(t, events, 0)
	client := NewAnthropicClient(srv.url, "sk-ant-test", "m")
	chunks, errs := client.Stream(context.Background(), Request{})
	var calls []*ToolCall
	var text string
	for c := range chunks {
		if c.Call != nil {
			calls = append(calls, c.Call)
		}
		text += c.Text
	}
	if err := <-errs; err != nil {
		t.Fatalf("stream error: %v", err)
	}
	if text != "我看一下" {
		t.Fatalf("text = %q", text)
	}
	if len(calls) != 1 || calls[0].ID != "toolu_1" || calls[0].Name != "read_file" || calls[0].Args != `{"path":"a.go"}` {
		t.Fatalf("calls = %+v", calls)
	}
}

func TestAnthropicToolResultRoundTrip(t *testing.T) {
	srv := newAnthropicServer(t, []string{anthropicEvent("message_stop", `{"type":"message_stop"}`)}, 0)
	client := NewAnthropicClient(srv.url, "sk-ant-test", "m")
	chunks, errs := client.Stream(context.Background(), Request{
		Messages: []Message{
			{Role: "assistant", Calls: []ToolCall{{ID: "toolu_1", Name: "read_file", Args: `{"path":"a.go"}`}}},
			{Role: "tool", ToolID: "toolu_1", Content: "file content"},
		},
		Tools: []ToolSpec{{Name: "read_file", Description: "read", Schema: []byte(`{"type":"object","properties":{"path":{"type":"string"}}}`)}},
	})
	for range chunks {
	}
	if err := <-errs; err != nil {
		t.Fatal(err)
	}
	srv.mu.Lock()
	body := srv.requests[0]
	srv.mu.Unlock()
	var reqBody struct {
		Messages []struct {
			Role    string `json:"role"`
			Content []struct {
				Type   string `json:"type"`
				ToolID string `json:"tool_use_id"`
			} `json:"content"`
		} `json:"messages"`
		Tools []struct {
			Name        string          `json:"name"`
			InputSchema json.RawMessage `json:"input_schema"`
		} `json:"tools"`
	}
	if err := json.Unmarshal(body, &reqBody); err != nil {
		t.Fatal(err)
	}
	if len(reqBody.Messages) != 2 || reqBody.Messages[1].Role != "user" ||
		reqBody.Messages[1].Content[0].Type != "tool_result" ||
		reqBody.Messages[1].Content[0].ToolID != "toolu_1" {
		t.Fatalf("messages = %+v", reqBody.Messages)
	}
	if len(reqBody.Tools) != 1 || reqBody.Tools[0].Name != "read_file" ||
		!json.Valid(reqBody.Tools[0].InputSchema) {
		t.Fatalf("tools = %+v", reqBody.Tools)
	}
}

func TestAnthropicHTTPError(t *testing.T) {
	srv := newAnthropicServer(t, nil, 401)
	client := NewAnthropicClient(srv.url, "sk-ant-test", "m")
	chunks, errs := client.Stream(context.Background(), Request{})
	for range chunks {
	}
	err := <-errs
	var re *RetryableError
	if !errors.As(err, &re) || re.Status != 401 || re.Retryable {
		t.Fatalf("want non-retryable 401, got %v", err)
	}
}

func TestClientFactoryAndDetection(t *testing.T) {
	if _, ok := NewClient("anthropic", "https://api.anthropic.com", "k", "m").(*AnthropicClient); !ok {
		t.Fatal("anthropic protocol should build AnthropicClient")
	}
	if _, ok := NewClient("openai-chat", "https://api.openai.com", "k", "m").(*ChatClient); !ok {
		t.Fatal("default protocol should build ChatClient")
	}
	if got := DetectProtocol("", "https://api.anthropic.com"); got != ProtocolAnthropic {
		t.Fatalf("detect anthropic = %s", got)
	}
	if got := DetectProtocol("", "https://api.openai.com/v1"); got != ProtocolOpenAIChat {
		t.Fatalf("detect openai = %s", got)
	}
	if got := DetectProtocol(ProtocolAnthropic, "https://api.openai.com/v1"); got != ProtocolAnthropic {
		t.Fatalf("explicit protocol must win: %s", got)
	}
}
