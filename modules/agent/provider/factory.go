package provider

import (
	"context"
	"fmt"
	"os"
	"strings"
)

// NewLLMProviderFromEnv 按 AGENT_LLM_PROVIDER 创建 Provider。
// 支持：stub（默认）、gemini。
func NewLLMProviderFromEnv(ctx context.Context) (LLMProvider, error) {
	name := strings.ToLower(strings.TrimSpace(os.Getenv("AGENT_LLM_PROVIDER")))
	switch name {
	case "", "stub", "rule":
		return StubLLMProvider{}, nil
	case "gemini":
		return NewGeminiProvider(ctx, GeminiConfigFromEnv())
	default:
		return nil, fmt.Errorf("未知 AGENT_LLM_PROVIDER=%q，支持 stub|gemini", name)
	}
}
