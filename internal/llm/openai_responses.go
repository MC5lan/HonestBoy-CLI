package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
)

// ResponsesClient 适配 OpenAI Responses API（/responses）。
// 与 Chat Completions 的关键差异：
//   - 语义化 SSE 事件（response.output_text.delta 等），无 [DONE] 哨兵
//   - 工具定义扁平化（type/name/description/parameters 在同一层）
//   - input 是 typed items 数组，system 提示放顶层 instructions
//   - 工具结果用 function_call_output item 回传，用 call_id 关联
//
// 本实现显式设置 store:false，符合产品“数据不出机器”的定位（会话由本地 jsonl 管理）。
type ResponsesClient struct {
	BaseURL string
	APIKey  string
	Model   string
	HTTP    *http.Client
}

func NewResponsesClient(baseURL, apiKey, model string) *ResponsesClient {
	return &ResponsesClient{
		BaseURL: strings.TrimRight(baseURL, "/"),
		APIKey:  apiKey,
		Model:   model,
		HTTP:    NewHTTPClient(),
	}
}

func (c *ResponsesClient) Stream(ctx context.Context, req Request) (<-chan Chunk, <-chan error) {
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

// responsesEndpoint 拼出 /responses。约定 baseURL 已含版本段（如 .../v1），
// 与 ChatClient 的 /chat/completions 约定一致。
func responsesEndpoint(baseURL string) string {
	return strings.TrimRight(baseURL, "/") + "/responses"
}

// toResponsesInput 把统一 Message 转成 Responses 的 typed items。
// system 消息单独收集为 instructions，不进 input。
func toResponsesInput(msgs []Message) (items []any, instructions []string) {
	for _, m := range msgs {
		switch m.Role {
		case "system":
			instructions = append(instructions, m.Content)
		case "tool":
			items = append(items, map[string]any{
				"type": "function_call_output", "call_id": m.ToolID, "output": m.Content,
			})
		case "assistant":
			if len(m.Calls) == 0 {
				items = append(items, map[string]any{"role": "assistant", "content": m.Content})
				continue
			}
			for _, call := range m.Calls {
				items = append(items, map[string]any{
					"type": "function_call", "call_id": call.ID,
					"name": call.Name, "arguments": call.Args,
				})
			}
		default: // user
			items = append(items, map[string]any{"role": "user", "content": m.Content})
		}
	}
	return items, instructions
}

// toResponsesTools 生成扁平工具定义。
// 显式设 strict:false：工具注册表生成的 schema 未必满足严格模式要求
// （additionalProperties:false、全部字段 required），避免被服务端拒绝。
func toResponsesTools(specs []ToolSpec) []map[string]any {
	out := make([]map[string]any, 0, len(specs))
	for _, s := range specs {
		schema := json.RawMessage(`{"type":"object"}`)
		if len(s.Schema) > 0 {
			schema = s.Schema
		}
		out = append(out, map[string]any{
			"type": "function", "name": s.Name,
			"description": s.Description, "parameters": schema, "strict": false,
		})
	}
	return out
}

type responsesSSE struct {
	Type string `json:"type"`
	// output_text.delta
	Delta string `json:"delta"`
	// output_item.added / done
	Item struct {
		ID     string          `json:"id"`
		CallID string          `json:"call_id"`
		Type   string          `json:"type"`
		Name   string          `json:"name"`
		Args   json.RawMessage `json:"arguments"`
	} `json:"item"`
	// function_call_arguments.delta / done
	ItemID    string `json:"item_id"`
	Arguments string `json:"arguments"`
	// completed
	Response struct {
		Usage struct {
			InputTokens  int `json:"input_tokens"`
			OutputTokens int `json:"output_tokens"`
		} `json:"usage"`
	} `json:"response"`
	// error / failed
	Message string `json:"message"`
}

// decodeResponsesArgs 处理 arguments 的两种形态：JSON 字符串（标准）或裸对象（兼容端点）。
func decodeResponsesArgs(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s
	}
	return string(raw)
}

type responsesCallBuf struct {
	itemID    string // 事件里的 item_id（fc_...），用于索引分片
	callID    string // 回传工具结果用的 call_id（call_...）
	name      string
	args      strings.Builder
	finalArgs string
}

func (c *ResponsesClient) stream(ctx context.Context, req Request, out chan<- Chunk) error {
	model := req.Model
	if model == "" {
		model = c.Model
	}
	maxTokens := req.MaxTokens
	if maxTokens < 16 {
		maxTokens = 4096
	}
	items, instructions := toResponsesInput(req.Messages)
	body := map[string]any{
		"model": model, "input": items, "stream": true,
		"store": false, "max_output_tokens": maxTokens,
	}
	if len(instructions) > 0 {
		body["instructions"] = strings.Join(instructions, "\n\n")
	}
	if len(req.Tools) > 0 {
		body["tools"] = toResponsesTools(req.Tools)
	}
	raw, err := json.Marshal(body)
	if err != nil {
		return err
	}

	httpClient := c.HTTP
	if httpClient == nil {
		httpClient = NewHTTPClient()
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, responsesEndpoint(c.BaseURL), bytes.NewReader(raw))
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

	calls := make(map[string]*responsesCallBuf)
	var order []string
	var usage *Usage
	var streamErr error
	sawCompleted := false

	err = ParseSSE(resp.Body, func(_, data []byte) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		var e responsesSSE
		if uerr := json.Unmarshal([]byte(SanitizeUTF8(string(data))), &e); uerr != nil {
			return nil // 畸形块跳过
		}
		switch e.Type {
		case "response.output_text.delta":
			if e.Delta != "" {
				out <- Chunk{Text: SanitizeUTF8(e.Delta)}
			}
		case "response.output_item.added":
			if e.Item.Type == "function_call" {
				// 分片事件用 item_id（fc_...）索引，回传结果才用 call_id（call_...）。
				itemID := e.Item.ID
				if itemID == "" {
					itemID = e.Item.CallID
				}
				if itemID != "" {
					calls[itemID] = &responsesCallBuf{itemID: itemID, callID: e.Item.CallID, name: e.Item.Name}
					order = append(order, itemID)
					if itemArgs := decodeResponsesArgs(e.Item.Args); itemArgs != "" && itemArgs != "{}" {
						calls[itemID].args.WriteString(itemArgs)
					}
				}
			}
		case "response.function_call_arguments.delta":
			if buf := calls[e.ItemID]; buf != nil {
				buf.args.WriteString(e.Delta)
			}
		case "response.function_call_arguments.done":
			if buf := calls[e.ItemID]; buf != nil {
				buf.finalArgs = e.Arguments
			}
		case "response.output_item.done":
			if e.Item.Type == "function_call" {
				itemID := e.Item.ID
				if itemID == "" {
					itemID = e.Item.CallID
				}
				if buf := calls[itemID]; buf != nil {
					buf.name = e.Item.Name
					if e.Item.CallID != "" {
						buf.callID = e.Item.CallID
					}
					if itemArgs := decodeResponsesArgs(e.Item.Args); itemArgs != "" {
						buf.finalArgs = itemArgs
					}
				} else if itemID != "" {
					calls[itemID] = &responsesCallBuf{itemID: itemID, callID: e.Item.CallID, name: e.Item.Name, finalArgs: decodeResponsesArgs(e.Item.Args)}
					order = append(order, itemID)
				}
			}
		case "response.completed":
			sawCompleted = true
			usage = &Usage{In: e.Response.Usage.InputTokens, Out: e.Response.Usage.OutputTokens}
		case "response.failed", "response.incomplete", "error":
			msg := e.Message
			if msg == "" {
				msg = "responses stream " + e.Type
			}
			streamErr = errors.New(msg)
		}
		return nil
	})
	if err != nil {
		return err
	}

	for _, id := range order {
		buf := calls[id]
		args := buf.finalArgs
		if args == "" {
			args = buf.args.String()
		}
		if args == "" {
			args = "{}"
		}
		if !json.Valid([]byte(args)) {
			args = "{}"
		}
		callID := buf.callID
		if callID == "" {
			callID = buf.itemID
		}
		out <- Chunk{Call: &ToolCall{ID: callID, Name: buf.name, Args: args}}
	}
	if usage != nil {
		out <- Chunk{Usage: usage}
	}
	if !sawCompleted && streamErr == nil {
		out <- Chunk{Disconnected: true}
	}
	out <- Chunk{Done: true}
	return streamErr
}

var _ Client = (*ResponsesClient)(nil)
