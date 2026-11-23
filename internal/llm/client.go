package llm

import (
	"context"
	"encoding/json"
	"strings"
)

type Message struct {
	Role    string
	Content string
	ToolID  string
	Calls   []ToolCall
}

type ToolCall struct{ ID, Name, Args string }

type Request struct {
	Model       string
	Messages    []Message
	Tools       []ToolSpec
	MaxTokens   int
	Temperature float64
	Stop        []string
}

type Chunk struct {
	Text  string
	Call  *ToolCall
	Usage *Usage
	Done  bool
}

type ToolSpec struct {
	Name        string
	Description string
	Schema      json.RawMessage
}

// Client 是统一模型接口。生产方负责关闭两个 channel。
type Client interface {
	Stream(ctx context.Context, req Request) (<-chan Chunk, <-chan error)
}

// 支持的协议。Chat Completions 是默认，Anthropic Messages 是第二标准。
const (
	ProtocolOpenAIChat = "openai-chat"
	ProtocolAnthropic  = "anthropic"
	ProtocolResponses  = "responses"
)

// NewClient 按协议构造适配器；未知协议回落到 Chat Completions。
func NewClient(protocol, baseURL, apiKey, model string) Client {
	switch strings.ToLower(protocol) {
	case ProtocolAnthropic, "anthropic-messages", "messages":
		return NewAnthropicClient(baseURL, apiKey, model)
	case ProtocolResponses, "openai-responses", "responses-api":
		return NewResponsesClient(baseURL, apiKey, model)
	default:
		return NewChatClient(baseURL, apiKey, model)
	}
}

// DetectProtocol 显式配置优先，否则从端点 URL 推断。
func DetectProtocol(explicit, baseURL string) string {
	if explicit != "" {
		return strings.ToLower(explicit)
	}
	if AnthropicEndpoint(baseURL) {
		return ProtocolAnthropic
	}
	// 部分网关直接把 Responses 端点写进 base_url。
	// 其余情况（base_url 只含 /v1）需显式指定 --protocol responses。
	if strings.HasSuffix(strings.TrimRight(baseURL, "/"), "/responses") {
		return ProtocolResponses
	}
	return ProtocolOpenAIChat
}
