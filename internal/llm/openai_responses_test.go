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

type responsesServer struct {
	url      string
	mu       sync.Mutex
	requests [][]byte
	status   int
}

func newResponsesServer(t *testing.T, events []string, status int) *responsesServer {
	t.Helper()
	s := &responsesServer{status: status}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		s.mu.Lock()
		s.requests = append(s.requests, body)
		s.mu.Unlock()
		if r.URL.Path != "/responses" {
			http.Error(w, "wrong path", http.StatusNotFound)
			return
		}
		if r.Header.Get("Authorization") == "" {
			http.Error(w, "missing auth", http.StatusBadRequest)
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

// rspEvent 生成 Responses API 的 SSE 事件（event: 行 + data: 行）。
func rspEvent(name, data string) string { return "event: " + name + "\ndata: " + data }

func TestResponsesStreamTextAndUsage(t *testing.T) {
	events := []string{
		rspEvent("response.created", `{"type":"response.created"}`),
		rspEvent("response.output_text.delta", `{"type":"response.output_text.delta","delta":"你好"}`),
		rspEvent("response.output_text.delta", `{"type":"response.output_text.delta","delta":"世界"}`),
		rspEvent("response.output_text.done", `{"type":"response.output_text.done","text":"你好世界"}`),
		rspEvent("response.completed", `{"type":"response.completed","response":{"usage":{"input_tokens":13,"output_tokens":8}}}`),
	}
	srv := newResponsesServer(t, events, 0)
	client := NewResponsesClient(srv.url, "sk-test", "gpt-test")
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
	if usage == nil || usage.In != 13 || usage.Out != 8 {
		t.Fatalf("usage = %+v", usage)
	}

	srv.mu.Lock()
	body := srv.requests[0]
	srv.mu.Unlock()
	var reqBody map[string]any
	if err := json.Unmarshal(body, &reqBody); err != nil {
		t.Fatal(err)
	}
	if reqBody["instructions"] != "你是 Honest Boy" || reqBody["store"] != false {
		t.Fatalf("bad body: %v", reqBody)
	}
	if reqBody["stream"] != true || reqBody["max_output_tokens"] == nil {
		t.Fatalf("bad body: %v", reqBody)
	}
	items := reqBody["input"].([]any)
	if len(items) != 1 || items[0].(map[string]any)["role"] != "user" {
		t.Fatalf("input = %v", items)
	}
}

func TestResponsesStreamFunctionCall(t *testing.T) {
	events := []string{
		rspEvent("response.output_item.added", `{"type":"response.output_item.added","item":{"type":"function_call","id":"fc_1","call_id":"call_1","name":"read_file","arguments":""}}`),
		// 只靠 delta 分片提供参数，验证 item_id(fc_1) 与 call_id(call_1) 分别索引。
		rspEvent("response.function_call_arguments.delta", `{"type":"response.function_call_arguments.delta","item_id":"fc_1","delta":"{\"path\":"}`),
		rspEvent("response.function_call_arguments.delta", `{"type":"response.function_call_arguments.delta","item_id":"fc_1","delta":"\"a.go\"}"}`),
		rspEvent("response.output_item.done", `{"type":"response.output_item.done","item":{"type":"function_call","id":"fc_1","call_id":"call_1","name":"read_file"}}`),
		rspEvent("response.completed", `{"type":"response.completed","response":{"usage":{"input_tokens":5,"output_tokens":9}}}`),
	}
	srv := newResponsesServer(t, events, 0)
	client := NewResponsesClient(srv.url, "sk-test", "m")
	chunks, errs := client.Stream(context.Background(), Request{})
	var calls []*ToolCall
	for c := range chunks {
		if c.Call != nil {
			calls = append(calls, c.Call)
		}
	}
	if err := <-errs; err != nil {
		t.Fatalf("stream error: %v", err)
	}
	if len(calls) != 1 || calls[0].ID != "call_1" || calls[0].Name != "read_file" ||
		calls[0].Args != `{"path":"a.go"}` {
		t.Fatalf("calls = %d, ID=%q Name=%q Args=%q", len(calls), calls[0].ID, calls[0].Name, calls[0].Args)
	}
}

func TestResponsesToolResultRoundTrip(t *testing.T) {
	srv := newResponsesServer(t, []string{
		rspEvent("response.completed", `{"type":"response.completed","response":{"usage":{}}}`),
	}, 0)
	client := NewResponsesClient(srv.url, "sk-test", "m")
	chunks, errs := client.Stream(context.Background(), Request{
		Messages: []Message{
			{Role: "assistant", Calls: []ToolCall{{ID: "call_1", Name: "read_file", Args: `{"path":"a.go"}`}}},
			{Role: "tool", ToolID: "call_1", Content: "file content"},
		},
		Tools: []ToolSpec{{Name: "read_file", Description: "read", Schema: []byte(`{"type":"object"}`)}},
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
		Input []struct {
			Type   string `json:"type"`
			CallID string `json:"call_id"`
			Role   string `json:"role"`
		} `json:"input"`
		Tools []struct {
			Type       string          `json:"type"`
			Name       string          `json:"name"`
			Parameters json.RawMessage `json:"parameters"`
			Strict     *bool           `json:"strict"`
		} `json:"tools"`
	}
	if err := json.Unmarshal(body, &reqBody); err != nil {
		t.Fatal(err)
	}
	if len(reqBody.Input) != 2 || reqBody.Input[0].Type != "function_call" ||
		reqBody.Input[0].CallID != "call_1" || reqBody.Input[1].Type != "function_call_output" ||
		reqBody.Input[1].CallID != "call_1" {
		t.Fatalf("input = %+v", reqBody.Input)
	}
	if len(reqBody.Tools) != 1 || reqBody.Tools[0].Type != "function" ||
		reqBody.Tools[0].Name != "read_file" || reqBody.Tools[0].Strict == nil || *reqBody.Tools[0].Strict {
		t.Fatalf("tools = %+v", reqBody.Tools)
	}
}

func TestResponsesErrorEvents(t *testing.T) {
	srv := newResponsesServer(t, []string{
		rspEvent("response.output_text.delta", `{"type":"response.output_text.delta","delta":"部分输出"}`),
		rspEvent("error", `{"type":"error","message":"rate limit"}`),
	}, 0)
	client := NewResponsesClient(srv.url, "sk-test", "m")
	chunks, errs := client.Stream(context.Background(), Request{})
	var text string
	for c := range chunks {
		text += c.Text
	}
	err := <-errs
	if err == nil || err.Error() != "rate limit" {
		t.Fatalf("error = %v", err)
	}
	if text != "部分输出" {
		t.Fatalf("partial text lost: %q", text)
	}
}

func TestResponsesHTTPError(t *testing.T) {
	srv := newResponsesServer(t, nil, 429)
	client := NewResponsesClient(srv.url, "sk-test", "m")
	chunks, errs := client.Stream(context.Background(), Request{})
	for range chunks {
	}
	err := <-errs
	var re *RetryableError
	if !errors.As(err, &re) || re.Status != 429 || !re.Retryable {
		t.Fatalf("want retryable 429, got %v", err)
	}
}

func TestResponsesFactoryAndDetection(t *testing.T) {
	if _, ok := NewClient("responses", "https://api.openai.com/v1", "k", "m").(*ResponsesClient); !ok {
		t.Fatal("responses protocol should build ResponsesClient")
	}
	if got := DetectProtocol("", "https://gateway.example/v1/responses"); got != ProtocolResponses {
		t.Fatalf("detect responses = %s", got)
	}
	if got := DetectProtocol(ProtocolResponses, "https://api.openai.com/v1"); got != ProtocolResponses {
		t.Fatalf("explicit responses must win: %s", got)
	}
}
