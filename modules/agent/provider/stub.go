package provider

import "context"

// StubLLMProvider 未配置 LLM 时的占位实现。
type StubLLMProvider struct{}

func (StubLLMProvider) Name() string { return "stub" }

func (StubLLMProvider) Chat(_ context.Context, _ []ChatMessage) (*ChatResponse, error) {
	return nil, ErrLLMNotConfigured
}
