package provider

import "context"

// AsStreaming 若 Provider 支持流式则返回，否则用 Chat 模拟单次回调。
func AsStreaming(llm LLMProvider) StreamingLLMProvider {
	if s, ok := llm.(StreamingLLMProvider); ok {
		return s
	}
	return chatStreamAdapter{llm}
}

type chatStreamAdapter struct{ LLMProvider }

func (a chatStreamAdapter) ChatStream(ctx context.Context, messages []ChatMessage, onChunk func(string)) (*ChatResponse, error) {
	resp, err := a.Chat(ctx, messages)
	if err != nil {
		return nil, err
	}
	if onChunk != nil && resp.Content != "" {
		onChunk(resp.Content)
	}
	return resp, nil
}
