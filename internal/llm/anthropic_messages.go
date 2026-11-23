package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
)

// AnthropicClient 适配 Anthropic Messages API（/v1/messages）。
// 这是 2026 年除 Chat Completions 之外的事实第二标准。
// 流式协议采用带语义的 SSE 事件：message_start / content_block_start /
// content_block_delta / content_block_stop / message_delta / message_stop。
type AnthropicClient struct {
	BaseURL string
	APIKey  string
	Model   string
	Version string // anthropic-version，默认 2023-06-01
	HTTP    *http.Client
}

func NewAnthropicClient(baseURL, apiKey, model string) *AnthropicClient {
	return &AnthropicClient{
		BaseURL: strings.TrimRight(baseURL, "/"),
		APIKey:  apiKey,
		Model:   model,
		Version: "2023-06-01",
		HTTP:    NewHTTPClient(),
	}
}

func (c *AnthropicClient) Stream(ctx context.Context, req Request) (<-chan Chunk, <-chan error) {
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

type anthropicBlock struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
	ID   string `json:"id,omitempty"`
	Name string `json:"name,omitempty"`
	// tool_use 的 input 与 tool_result 的 content
	Input   json.RawMessage `json:"input,omitempty"`
	Content any             `json:"content,omitempty"`
	ToolID  string          `json:"tool_use_id,omitempty"`
	IsError bool            `json:"is_error,omitempty"`
}

type anthropicMessage struct {
	Role    string          `json:"role"`
	Content json.RawMessage `json:"content"`
}

// toAnthropicMessages 把统一的 Message 转换为 content-block 格式。
// 纯文本用字符串；含工具调用的 assistant 用块数组；
// 工具结果作为 user 消息里的 tool_result 块回传。
func toAnthropicMessages(msgs []Message) []anthropicMessage {
	out := make([]anthropicMessage, 0, len(msgs))
	for _, m := range msgs {
		if m.Role == "system" {
			continue // system 作为顶层字段
		}
		// 工具结果必须以 user 消息的 tool_result 块回传。
		if m.Role == "tool" {
			blocks := []anthropicBlock{{
				Type: "tool_result", ToolID: m.ToolID,
				Content: m.Content, IsError: false,
			}}
			b, _ := json.Marshal(blocks)
			out = append(out, anthropicMessage{Role: "user", Content: b})
			continue
		}
		if len(m.Calls) == 0 {
			b, _ := json.Marshal(m.Content)
			out = append(out, anthropicMessage{Role: normalizeRole(m.Role), Content: b})
			continue
		}
		var blocks []anthropicBlock
		if m.Content != "" {
			blocks = append(blocks, anthropicBlock{Type: "text", Text: m.Content})
		}
		for _, call := range m.Calls {
			blocks = append(blocks, anthropicBlock{
				Type: "tool_use", ID: call.ID, Name: call.Name,
				Input: json.RawMessage(call.Args),
			})
		}
		b, _ := json.Marshal(blocks)
		out = append(out, anthropicMessage{Role: "assistant", Content: b})
	}
	return out
}

func normalizeRole(role string) string {
	if role == "assistant" {
		return "assistant"
	}
	return "user"
}

// toAnthropicTools 把 ToolSpec 转为 input_schema 格式。
func toAnthropicTools(specs []ToolSpec) []map[string]any {
	out := make([]map[string]any, 0, len(specs))
	for _, s := range specs {
		schema := json.RawMessage(`{"type":"object"}`)
		if len(s.Schema) > 0 {
			schema = s.Schema
		}
		out = append(out, map[string]any{
			"name": s.Name, "description": s.Description, "input_schema": schema,
		})
	}
	return out
}

type anthropicSSE struct {
	Type string `json:"type"`
	// message_start
	Message struct {
		Usage struct {
			InputTokens int `json:"input_tokens"`
		} `json:"usage"`
	} `json:"message"`
	// content_block_start / content_block_delta
	Index        int `json:"index"`
	ContentBlock struct {
		Type  string          `json:"type"`
		Text  string          `json:"text"`
		ID    string          `json:"id"`
		Name  string          `json:"name"`
		Input json.RawMessage `json:"input"`
	} `json:"content_block"`
	Delta struct {
		Type        string `json:"type"`
		Text        string `json:"text"`
		PartialJSON string `json:"partial_json"`
		StopReason  string `json:"stop_reason"`
	} `json:"delta"`
	// message_delta
	Usage struct {
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
}

type anthropicToolBuf struct {
	id, name string
	args     strings.Builder
}

func (c *AnthropicClient) stream(ctx context.Context, req Request, out chan<- Chunk) error {
	model := req.Model
	if model == "" {
		model = c.Model
	}
	maxTokens := req.MaxTokens
	if maxTokens <= 0 {
		maxTokens = 4096
	}
	body := map[string]any{
		"model":      model,
		"max_tokens": maxTokens,
		"stream":     true,
		"messages":   toAnthropicMessages(req.Messages),
	}
	// system 作为顶层字段，不是 messages 里的一条。
	var systemParts []string
	for _, m := range req.Messages {
		if m.Role == "system" {
			systemParts = append(systemParts, m.Content)
		}
	}
	if len(systemParts) > 0 {
		body["system"] = strings.Join(systemParts, "\n\n")
	}
	if len(req.Tools) > 0 {
		body["tools"] = toAnthropicTools(req.Tools)
	}
	raw, err := json.Marshal(body)
	if err != nil {
		return err
	}

	endpoint := c.BaseURL + "/v1/messages"
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
		httpReq.Header.Set("x-api-key", c.APIKey)
	}
	version := c.Version
	if version == "" {
		version = "2023-06-01"
	}
	httpReq.Header.Set("anthropic-version", version)

	resp, err := httpClient.Do(httpReq)
	if err != nil {
		return &RetryableError{Status: 0, Retryable: true, Message: err.Error()}
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		msg, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		retryable, _ := ClassifyStatus(resp.StatusCode)
		return &RetryableError{Status: resp.StatusCode, Retryable: retryable, Message: string(msg)}
	}

	tools := make(map[int]*anthropicToolBuf)
	var order []int
	var inputTokens, outputTokens int

	err = ParseSSE(resp.Body, func(event, data []byte) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		var e anthropicSSE
		if uerr := json.Unmarshal([]byte(SanitizeUTF8(string(data))), &e); uerr != nil {
			return nil // 畸形块跳过，不 panic
		}
		if e.Type == "" && len(event) > 0 {
			e.Type = string(event)
		}
		switch e.Type {
		case "message_start":
			inputTokens = e.Message.Usage.InputTokens
		case "content_block_start":
			if e.ContentBlock.Type == "tool_use" {
				tools[e.Index] = &anthropicToolBuf{id: e.ContentBlock.ID, name: e.ContentBlock.Name}
				order = append(order, e.Index)
				// 有些兼容端点直接把完整 input 放在 start 里。
				if len(e.ContentBlock.Input) > 0 && string(e.ContentBlock.Input) != "{}" {
					tools[e.Index].args.Write(e.ContentBlock.Input)
				}
			}
		case "content_block_delta":
			switch e.Delta.Type {
			case "text_delta":
				if e.Delta.Text != "" {
					out <- Chunk{Text: SanitizeUTF8(e.Delta.Text)}
				}
			case "input_json_delta":
				if buf := tools[e.Index]; buf != nil {
					buf.args.WriteString(e.Delta.PartialJSON)
				}
			}
		case "message_delta":
			outputTokens = e.Usage.OutputTokens
		case "message_stop":
			// 流结束标志（没有 [DONE]）。
		case "ping":
			// 心跳，忽略。
		}
		return nil
	})
	if err != nil {
		return err
	}

	for _, idx := range order {
		buf := tools[idx]
		args := buf.args.String()
		if args == "" {
			args = "{}"
		}
		if !json.Valid([]byte(args)) {
			args = "{}"
		}
		out <- Chunk{Call: &ToolCall{ID: buf.id, Name: buf.name, Args: args}}
	}
	out <- Chunk{Usage: &Usage{In: inputTokens, Out: outputTokens}}
	out <- Chunk{Done: true}
	return nil
}

var _ Client = (*AnthropicClient)(nil)

// AnthropicEndpoint 判断 base URL 是否指向 Anthropic 兼容端点。
func AnthropicEndpoint(baseURL string) bool {
	lower := strings.ToLower(baseURL)
	return strings.Contains(lower, "anthropic") || strings.Contains(lower, "/v1/messages")
}
