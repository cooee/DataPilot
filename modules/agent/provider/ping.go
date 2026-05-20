package provider

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/Caknoooo/go-gin-clean-starter/modules/agent/skill"
)

// PingResult LLM 连通性检查结果。
type PingResult struct {
	OK           bool   `json:"ok"`
	Provider     string `json:"provider"`
	Model        string `json:"model,omitempty"`
	LatencyMs    int64  `json:"latency_ms"`
	ReplyPreview string `json:"reply_preview,omitempty"`
	Usage        TokenUsage `json:"usage,omitempty"`
	Error        string `json:"error,omitempty"`
	Hint         string `json:"hint,omitempty"`
	SkillLoaded  bool   `json:"skill_loaded"`
	SkillID      string `json:"skill_id,omitempty"`
	SkillPath    string `json:"skill_path,omitempty"`
	SkillSHA256  string `json:"skill_sha256,omitempty"`
}

// PingOptions 检查选项。
type PingOptions struct {
	TestOrchestration bool   // 是否额外测试 NL→Plan 编排
	SampleQuestion    string
}

// Ping 快速验证当前环境 LLM Provider 是否可用。
func Ping(ctx context.Context, opts PingOptions) PingResult {
	providerName := strings.ToLower(strings.TrimSpace(os.Getenv("AGENT_LLM_PROVIDER")))
	if providerName == "" || providerName == "stub" || providerName == "rule" {
		return PingResult{
			OK:       false,
			Provider: providerName,
			Error:    "未启用 LLM",
			Hint:     "在 .env 设置 AGENT_LLM_PROVIDER=gemini 并配置 GEMINI_API_KEY",
		}
	}

	llm, err := NewLLMProviderFromEnv(ctx)
	if err != nil {
		return PingResult{
			OK:       false,
			Provider: providerName,
			Error:    err.Error(),
			Hint:     "检查 GEMINI_API_KEY、GEMINI_MODEL 及 .env 格式（行尾勿写 # 注释）",
		}
	}

	res := PingResult{Provider: llm.Name()}
	if g, ok := llm.(*GeminiProvider); ok {
		res.Model = g.model
	}
	if doc, err := skill.LoadFromEnv(); err != nil {
		res.Error = "skill load: " + err.Error()
		res.OK = false
		return res
	} else if doc != nil {
		res.SkillLoaded = true
		res.SkillID = doc.ID
		res.SkillPath = doc.Path
		res.SkillSHA256 = doc.SHA256
	}

	start := time.Now()
	resp, err := llm.Chat(ctx, []ChatMessage{
		{Role: "user", Content: "Reply with exactly: pong"},
	})
	res.LatencyMs = time.Since(start).Milliseconds()

	if err != nil {
		res.OK = false
		res.Error = err.Error()
		res.Hint = pingErrorHint(err, providerName)
		return res
	}

	res.OK = true
	res.Usage = resp.Usage
	res.ReplyPreview = truncateStr(strings.TrimSpace(resp.Content), 120)

	if opts.TestOrchestration {
		if err := pingOrchestration(ctx, llm, opts); err != nil {
			res.OK = false
			res.Error = "Chat OK, orchestration failed: " + err.Error()
			res.Hint = "模型可连通，但 JSON 编排失败；可检查 prompt 或换 gemini-2.0-flash"
		}
	}

	return res
}

func pingOrchestration(ctx context.Context, llm LLMProvider, opts PingOptions) error {
	q := opts.SampleQuestion
	if q == "" {
		q = "2026-05-18 付费和免费日活充值对比"
	}
	orch := NewOrchestrator(llm)
	req := PlanRequest{
		Question: q,
		Date:     "2026-05-18",
		Intent:   "metric_qa",
		Presets:  []string{"product-type-compare", "anomaly-detection", "product-health"},
	}
	if doc, err := skill.LoadFromEnv(); err != nil {
		return err
	} else if doc != nil {
		req.SkillID = doc.ID
		req.SkillText = doc.Body
	}
	_, err := orch.Plan(ctx, req)
	return err
}

func pingErrorHint(err error, provider string) string {
	msg := strings.ToLower(err.Error())
	switch {
	case strings.Contains(msg, "api key"), strings.Contains(msg, "401"), strings.Contains(msg, "403"):
		return "API Key 无效或未授权，请在 Google AI Studio 检查 GEMINI_API_KEY"
	case strings.Contains(msg, "not found"), strings.Contains(msg, "404"):
		return "模型名不存在，尝试 GEMINI_MODEL=gemini-2.0-flash"
	case strings.Contains(msg, "timeout"), strings.Contains(msg, "deadline"):
		return "网络超时，检查代理或防火墙"
	default:
		return fmt.Sprintf("查看完整错误；当前 AGENT_LLM_PROVIDER=%s", provider)
	}
}

func truncateStr(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
