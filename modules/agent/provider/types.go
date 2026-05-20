package provider

import "context"

// ChatMessage 与 OpenAI 风格兼容的多轮消息。
type ChatMessage struct {
	Role    string // system | user | assistant
	Content string
}

// TokenUsage token 用量统计。
type TokenUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// ChatResponse 模型回复。
type ChatResponse struct {
	Content string
	Usage   TokenUsage
}

// LLMProvider 多模型统一聊天接口（Gemini / 后续 OpenAI 等实现此接口）。
type LLMProvider interface {
	Name() string
	Chat(ctx context.Context, messages []ChatMessage) (*ChatResponse, error)
}

// StreamingLLMProvider 支持流式输出（用于终端展示思考过程）。
type StreamingLLMProvider interface {
	LLMProvider
	ChatStream(ctx context.Context, messages []ChatMessage, onChunk func(string)) (*ChatResponse, error)
}
