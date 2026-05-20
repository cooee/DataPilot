package dto

import (
	analyticsdto "github.com/Caknoooo/go-gin-clean-starter/modules/analytics/dto"
)

const (
	MESSAGE_SUCCESS_ASK      = "指标问答成功"
	MESSAGE_SUCCESS_ANOMALY  = "异常产品列举成功"
	MESSAGE_FAILED_ASK       = "指标问答失败"
	MESSAGE_FAILED_ANOMALY   = "异常产品列举失败"
	MESSAGE_FAILED_BIND      = "请求参数无效"
)

// AskRequest 指标问答（NL → 语义层 → 数据 + 自然语言摘要）。
type AskRequest struct {
	Question string `json:"question"`          // 自然语言问题
	Date     string `json:"date"`              // 可选 YYYY-MM-DD，覆盖问题中的日期
	Preset   string `json:"preset"`            // 可选，直接指定 preset 跳过 NL
}

// AnomalyListRequest 异常产品列举。
type AnomalyListRequest struct {
	Question string `json:"question"` // 可选 NL
	Date     string `json:"date"`     // 报告日 YYYY-MM-DD
}

// AgentResponse 统一 Agent 输出，便于 LLM 二次加工或前端展示。
type AgentResponse struct {
	Intent         string                      `json:"intent"`
	Preset         string                      `json:"preset,omitempty"`
	Source         string                      `json:"source"` // rule | llm
	SkillLoaded    bool                        `json:"skill_loaded"`
	SkillID        string                      `json:"skill_id,omitempty"`
	SkillPath      string                      `json:"skill_path,omitempty"`
	SkillSHA256    string                      `json:"skill_sha256,omitempty"`
	Interpretation string                      `json:"interpretation"`
	Answer         string                      `json:"answer"`
	Query          *analyticsdto.QueryRequest  `json:"query"`
	Data           *analyticsdto.QueryResult   `json:"data"`
	Meta           *analyticsdto.QueryMeta     `json:"meta,omitempty"`
}
