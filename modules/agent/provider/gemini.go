package provider

import (
	"context"
	"fmt"
	"os"
	"strings"

	"google.golang.org/genai"
)

// GeminiConfig Gemini Provider 配置。
type GeminiConfig struct {
	APIKey string
	Model  string
}

// GeminiProvider 基于 google.golang.org/genai 的 Gemini 实现。
type GeminiProvider struct {
	client *genai.Client
	model  string
}

// GeminiConfigFromEnv 从环境变量读取配置。
func GeminiConfigFromEnv() GeminiConfig {
	model := strings.TrimSpace(os.Getenv("GEMINI_MODEL"))
	if model == "" {
		model = "gemini-2.0-flash"
	}
	return GeminiConfig{
		APIKey: strings.TrimSpace(os.Getenv("GEMINI_API_KEY")),
		Model:  model,
	}
}

// NewGeminiProvider 创建 Gemini Provider。
func NewGeminiProvider(ctx context.Context, cfg GeminiConfig) (*GeminiProvider, error) {
	if cfg.APIKey == "" {
		return nil, fmt.Errorf("GEMINI_API_KEY 未设置")
	}
	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey:  cfg.APIKey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		return nil, fmt.Errorf("genai client: %w", err)
	}
	return &GeminiProvider{client: client, model: cfg.Model}, nil
}

func (g *GeminiProvider) Name() string { return "gemini" }

func (g *GeminiProvider) Chat(ctx context.Context, messages []ChatMessage) (*ChatResponse, error) {
	return g.ChatStream(ctx, messages, nil)
}

func (g *GeminiProvider) ChatStream(ctx context.Context, messages []ChatMessage, onChunk func(string)) (*ChatResponse, error) {
	if len(messages) == 0 {
		return nil, fmt.Errorf("messages 不能为空")
	}

	systemText, contents := buildGeminiMessages(messages)
	cfg := &genai.GenerateContentConfig{}
	if systemText != "" {
		cfg.SystemInstruction = genai.NewContentFromText(systemText, genai.RoleUser)
	}

	var full strings.Builder
	var usage TokenUsage

	for resp, err := range g.client.Models.GenerateContentStream(ctx, g.model, contents, cfg) {
		if err != nil {
			return nil, fmt.Errorf("gemini stream: %w", err)
		}
		chunk := resp.Text()
		if chunk == "" {
			continue
		}
		full.WriteString(chunk)
		if onChunk != nil {
			onChunk(chunk)
		}
		if resp.UsageMetadata != nil {
			usage.PromptTokens = int(resp.UsageMetadata.PromptTokenCount)
			usage.CompletionTokens = int(resp.UsageMetadata.CandidatesTokenCount)
			usage.TotalTokens = int(resp.UsageMetadata.TotalTokenCount)
		}
	}

	text := strings.TrimSpace(full.String())
	if text == "" {
		return nil, fmt.Errorf("gemini 返回空内容")
	}
	return &ChatResponse{Content: text, Usage: usage}, nil
}

func buildGeminiMessages(messages []ChatMessage) (string, []*genai.Content) {
	var systemText string
	var contents []*genai.Content
	for _, m := range messages {
		switch strings.ToLower(m.Role) {
		case "system":
			systemText += m.Content + "\n"
		case "assistant":
			contents = append(contents, genai.NewContentFromText(m.Content, genai.RoleModel))
		default:
			contents = append(contents, genai.NewContentFromText(m.Content, genai.RoleUser))
		}
	}
	return strings.TrimSpace(systemText), contents
}
