package provider

// ModelName 返回底层模型名（若 Provider 未暴露则返回 unknown）。
func ModelName(llm LLMProvider) string {
	if llm == nil {
		return "unknown"
	}
	if g, ok := llm.(*GeminiProvider); ok {
		return g.Model()
	}
	return "unknown"
}

// Model 返回 Gemini 模型 ID。
func (g *GeminiProvider) Model() string {
	if g == nil {
		return ""
	}
	return g.model
}
