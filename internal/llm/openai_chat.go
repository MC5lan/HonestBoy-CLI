package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
)

// ChatClient 适配 OpenAI 兼容的 /chat/completions。
type ChatClient struct {
	BaseURL string
	APIKey  string
	Model   string
	HTTP    *http.Client
}

func NewChatClient(baseURL, apiKey, model string) *ChatClient {
	return &ChatClient{BaseURL: strings.TrimRight(baseURL, "/"), APIKey: apiKey, Model: model, HTTP: NewHTTPClient()}
}

func (c *ChatClient) Stream(ctx context.Context, req Request) (<-chan Chunk, <-chan error) {
	chunks := make(chan Chunk, 32)
	errs := make(chan error, 1)
	go func() {
		defer close(chunks)
		defer close(errs)
		if err := c.stream(ctx, req, chunks); err != nil {
			select {
			case errs <- err:
			default:
			}
		}
	}()
	return chunks, errs
}

type sseDelta struct {
	Choices []struct {
		Delta struct {
			Content          string `json:"content"`
			ReasoningContent string `json:"reasoning_content"`
			ToolCalls        []struct {
				Index    json.RawMessage `json:"index"`
				ID       string          `json:"id"`
				Type     string          `json:"type"`
				Function struct {
					Name      string `json:"name"`
					Arguments string `json:"arguments"`
				} `json:"function"`
			} `json:"tool_calls"`
		} `json:"delta"`
		FinishReason *string `json:"finish_reason"`
	} `json:"choices"`
	Usage *struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		CachedTokens     int `json:"cached_tokens"`
	} `json:"usage"`
}

type assembledCall struct {
	ID    string
	Name  string
	Args  strings.Builder
	index int
	bad   bool
}

func (c *ChatClient) stream(ctx context.Context, req Request, out chan<- Chunk) error {
	model := req.Model
	if model == "" {
		model = c.Model
	}
	body := map[string]any{
		"model":          model,
		"messages":       toWireMessages(req.Messages),
		"stream":         true,
		"stream_options": map[string]any{"include_usage": true},
	}
	if len(req.Tools) > 0 {
		body["tools"] = ToWireTools(req.Tools)
	}
	if req.MaxTokens > 0 {
		body["max_tokens"] = req.MaxTokens
	}
	if req.Temperature != 0 {
		body["temperature"] = req.Temperature
	}
	if len(req.Stop) > 0 {
		body["stop"] = req.Stop
	}
	if req.ReasoningEffort != "" {
		body["reasoning_effort"] = req.ReasoningEffort
	}
	raw, err := json.Marshal(body)
	if err != nil {
		return err
	}

	endpoint := c.BaseURL + "/chat/completions"
	httpClient := c.HTTP
	if httpClient == nil {
		httpClient = NewHTTPClient()
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(raw))
	if err != nil {
		return err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if c.APIKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+c.APIKey)
	}
	resp, err := httpClient.Do(httpReq)
	if err != nil {
		return &RetryableError{Status: 0, Retryable: true, Message: err.Error()}
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode >= 400 {
		msg, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		retryable, _ := ClassifyStatus(resp.StatusCode)
		return &RetryableError{Status: resp.StatusCode, Retryable: retryable, Message: string(msg)}
	}

	calls := make(map[int]*assembledCall)
	var order []int
	var finalUsage *Usage
	sawDone := false

	err = ParseSSE(resp.Body, func(_, data []byte) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if bytes.Equal(bytes.TrimSpace(data), []byte("[DONE]")) {
			sawDone = true
			return nil
		}
		var d sseDelta
		if uerr := json.Unmarshal([]byte(SanitizeUTF8(string(data))), &d); uerr != nil {
			// 非法 JSON 不 panic：跳过该块，记一条可读错误由上层处理。
			return nil
		}
		for _, ch := range d.Choices {
			if ch.Delta.ReasoningContent != "" {
				out <- Chunk{Reasoning: SanitizeUTF8(ch.Delta.ReasoningContent)}
			}
			if ch.Delta.Content != "" {
				out <- Chunk{Text: SanitizeUTF8(ch.Delta.Content)}
			}
			if ch.FinishReason != nil && *ch.FinishReason == "length" {
				out <- Chunk{Truncated: true}
			}
			for _, tc := range ch.Delta.ToolCalls {
				idx, perr := toolCallIndex(tc.Index)
				if perr != nil {
					idx = len(order) // 缺省时按出现顺序编号
				}
				ac, ok := calls[idx]
				if !ok {
					ac = &assembledCall{index: idx}
					calls[idx] = ac
					order = append(order, idx)
				}
				if tc.ID != "" && ac.ID == "" {
					ac.ID = tc.ID
				}
				if tc.Function.Name != "" && ac.Name == "" {
					ac.Name = tc.Function.Name
				}
				if tc.Function.Arguments != "" {
					ac.Args.WriteString(tc.Function.Arguments)
				}
			}
		}
		if d.Usage != nil {
			finalUsage = &Usage{In: d.Usage.PromptTokens, Out: d.Usage.CompletionTokens, Cached: d.Usage.CachedTokens}
		}
		return nil
	})
	if err != nil {
		return err
	}

	for _, idx := range order {
		ac := calls[idx]
		args := ac.Args.String()
		if args == "" {
			args = "{}"
		}
		if !json.Valid([]byte(args)) {
			ac.bad = true
			args = "{}"
		}
		call := &ToolCall{ID: ac.ID, Name: ac.Name, Args: args}
		out <- Chunk{Call: call}
	}
	if finalUsage != nil {
		out <- Chunk{Usage: finalUsage}
	}
	if !sawDone {
		out <- Chunk{Disconnected: true}
	}
	out <- Chunk{Done: true}
	return nil
}

func toolCallIndex(raw json.RawMessage) (int, error) {
	if len(raw) == 0 || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return 0, fmt.Errorf("missing index")
	}
	var n int
	if err := json.Unmarshal(raw, &n); err == nil {
		return n, nil
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return strconv.Atoi(s)
	}
	return 0, fmt.Errorf("bad index: %s", raw)
}

func toWireMessages(msgs []Message) []map[string]any {
	out := make([]map[string]any, 0, len(msgs))
	for _, m := range msgs {
		item := map[string]any{"role": m.Role, "content": m.Content}
		switch m.Role {
		case "assistant":
			if m.Reasoning != "" {
				item["reasoning_content"] = m.Reasoning
			}
			if len(m.Calls) > 0 {
				calls := make([]map[string]any, 0, len(m.Calls))
				for _, c := range m.Calls {
					calls = append(calls, map[string]any{
						"id": c.ID, "type": "function",
						"function": map[string]any{"name": c.Name, "arguments": c.Args},
					})
				}
				item["tool_calls"] = calls
			}
		case "tool":
			if m.ToolID != "" {
				item["tool_call_id"] = m.ToolID
			}
		}
		out = append(out, item)
	}
	return out
}
