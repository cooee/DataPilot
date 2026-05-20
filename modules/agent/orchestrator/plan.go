package orchestrator

import (
	analyticsdto "github.com/Caknoooo/go-gin-clean-starter/modules/analytics/dto"
	"github.com/Caknoooo/go-gin-clean-starter/modules/agent/skill"
)

// QueryPlan NL 编排结果 → 语义层查询。
type QueryPlan struct {
	Intent         string
	Preset         string
	Query          *analyticsdto.QueryRequest
	Interpretation string
	Confidence     float64 // 规则引擎固定 1.0
	Source         string  // rule | llm
	Skill          skill.Meta
}
