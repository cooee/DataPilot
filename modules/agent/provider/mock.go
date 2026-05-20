package provider

import "context"

// MockLLMProvider 测试用固定回复。
type MockLLMProvider struct {
	Reply string
	Err   error
}

func (m MockLLMProvider) Name() string { return "mock" }

func (m MockLLMProvider) Chat(_ context.Context, _ []ChatMessage) (*ChatResponse, error) {
	if m.Err != nil {
		return nil, m.Err
	}
	return &ChatResponse{Content: m.Reply}, nil
}
