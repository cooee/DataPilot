package provider

import "errors"

var ErrLLMNotConfigured = errors.New("LLM provider 未配置，请使用规则编排或设置 AGENT_LLM_PROVIDER")
