package report

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"time"

	"github.com/Caknoooo/go-gin-clean-starter/modules/agent/provider"
	"github.com/Caknoooo/go-gin-clean-starter/modules/agent/skill"
)

// LLMHTMLMeta 写入 HTML 的生成溯源信息。
type LLMHTMLMeta struct {
	Provider    string `json:"provider"`
	Model       string `json:"model"`
	SkillID     string `json:"skill_id"`
	SkillSHA256 string `json:"skill_sha256"`
	GeneratedAt string `json:"generated_at"`
}

// LLMHTMLOptions 大模型生成 HTML 选项。
type LLMHTMLOptions struct {
	Verbose      bool // 流水线步骤日志 → stderr
	ShowThinking bool // 先输出分析思考（流式 stdout）
}

// LLMHTMLResult 大模型生成的 HTML 及元数据。
type LLMHTMLResult struct {
	HTML     string
	Thinking string
	Meta     LLMHTMLMeta
}

// GenerateHTMLViaLLM 使用 Skill + 报表数据：可选先流式思考，再由大模型生成 HTML。
func GenerateHTMLViaLLM(
	ctx context.Context,
	llm provider.LLMProvider,
	skillDoc *skill.Document,
	payload *DailyReportPayload,
	opts LLMHTMLOptions,
) (*LLMHTMLResult, error) {
	if llm == nil {
		return nil, fmt.Errorf("LLM Provider 未配置")
	}
	if skillDoc == nil || !skillDoc.Loaded {
		return nil, fmt.Errorf("Skill 未加载：请设置 AGENT_SKILL_PATH=%s", skill.DefaultPath())
	}
	if payload == nil {
		return nil, fmt.Errorf("报表数据为空")
	}

	stream := provider.AsStreaming(llm)
	meta := LLMHTMLMeta{
		Provider:    llm.Name(),
		Model:       provider.ModelName(llm),
		SkillID:     skillDoc.ID,
		SkillSHA256: skillDoc.SHA256,
		GeneratedAt: time.Now().Format("2006-01-02 15:04:05"),
	}

	if opts.Verbose {
		log.Printf("[llm-html] skill=%s model=%s report=%s segments=%d trend_pts=%d",
			skillDoc.ID, meta.Model, payload.ReportDate, len(payload.Segments), len(payload.TrendHistory))
	}

	dataJSON, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("序列化报表数据: %w", err)
	}
	if opts.Verbose {
		log.Printf("[llm-html] 报表 JSON 大小: %d bytes", len(dataJSON))
	}

	var thinking string
	if opts.ShowThinking {
		if opts.Verbose {
			log.Println("[llm-html] [1/2] 流式输出数据分析思考过程 → stdout")
		}
		fmt.Fprintln(os.Stdout, "\n════════════════════════════════════════")
		fmt.Fprintln(os.Stdout, "  DataPilot · 数据分析思考过程（LLM）")
		fmt.Fprintf(os.Stdout, "  Model: %s · Skill: %s\n", meta.Model, meta.SkillID)
		fmt.Fprintln(os.Stdout, "════════════════════════════════════════")
		fmt.Println()

		thinkMsgs := []provider.ChatMessage{
			{Role: "system", Content: buildThinkingSystemPrompt(skillDoc, meta)},
			{Role: "user", Content: buildThinkingUserPrompt(string(dataJSON), meta)},
		}
		thinkResp, err := stream.ChatStream(ctx, thinkMsgs, func(chunk string) {
			_, _ = os.Stdout.WriteString(chunk)
		})
		fmt.Fprintln(os.Stdout, "\n\n════════════════════════════════════════")
		fmt.Fprintln(os.Stdout, "  思考完成 · 开始生成 HTML …")
		fmt.Fprintln(os.Stdout, "════════════════════════════════════════")
		fmt.Println()
		if err != nil {
			return nil, fmt.Errorf("思考阶段失败: %w", err)
		}
		thinking = strings.TrimSpace(thinkResp.Content)
		if opts.Verbose {
			log.Printf("[llm-html] 思考完成 tokens: prompt=%d completion=%d",
				thinkResp.Usage.PromptTokens, thinkResp.Usage.CompletionTokens)
		}
	}

	if opts.Verbose {
		log.Println("[llm-html] [2/2] 生成 HTML 文档…")
	}

	htmlMsgs := []provider.ChatMessage{
		{Role: "system", Content: buildLLMHTMLSystemPrompt(skillDoc, meta)},
		{Role: "user", Content: buildLLMHTMLUserPrompt(string(dataJSON), meta, thinking)},
	}
	var htmlWriter io.Writer
	if opts.Verbose {
		htmlWriter = os.Stderr
		_, _ = fmt.Fprintln(htmlWriter, "[llm-html] HTML 生成中（流式片段不打印，完成后写入文件）…")
	}
	htmlResp, err := stream.ChatStream(ctx, htmlMsgs, nil)
	if err != nil {
		return nil, fmt.Errorf("HTML 生成失败: %w", err)
	}
	if opts.Verbose {
		log.Printf("[llm-html] HTML 完成 tokens: prompt=%d completion=%d total=%d",
			htmlResp.Usage.PromptTokens, htmlResp.Usage.CompletionTokens, htmlResp.Usage.TotalTokens)
	}

	html := stripHTMLFences(htmlResp.Content)
	lower := strings.ToLower(html)
	if !strings.Contains(lower, "<!doctype") && !strings.Contains(lower, "<html") {
		return nil, fmt.Errorf("LLM 返回非 HTML 文档，请检查 prompt 或重试")
	}
	html = ensureModelBadge(html, meta)
	html = ensureReportTitle(html, payload.HTMLTitle)

	return &LLMHTMLResult{HTML: html, Thinking: thinking, Meta: meta}, nil
}

func buildThinkingSystemPrompt(skillDoc *skill.Document, meta LLMHTMLMeta) string {
	return fmt.Sprintf(`你是 DataPilot 资深数据分析师。请根据 Skill 与 JSON 数据，用 **Markdown 中文** 输出「思考过程」，供运营与工程师审阅。

## Skill（id=%s）
%s

## 输出要求
1. 分节：## 数据概览、## 付费产品分析、## 免费产品分析、## 站点产品分析、## 异常与风险、## 7日预测解读、## 结论与建议
2. 引用 JSON 中的具体数字；说明 Forecasts 的 Method / SampleDays（若 carry_forward 需解释历史不足）
3. **不要输出 HTML**，不要代码块包裹整篇；可使用列表与表格
4. 文末一行：> 思考完成 · 将由 %s 生成 HTML 报表`, skillDoc.ID, skillDoc.Body, meta.Model)
}

func buildThinkingUserPrompt(dataJSON string, meta LLMHTMLMeta) string {
	return fmt.Sprintf("报告日数据 JSON（provider=%s model=%s）：\n\n%s", meta.Provider, meta.Model, dataJSON)
}

func buildLLMHTMLSystemPrompt(skillDoc *skill.Document, meta LLMHTMLMeta) string {
	badgeText := modelBadgeText(meta)
	return fmt.Sprintf(`你是 DataPilot 运营分析日报的 **HTML 生成器**。严格遵循 Skill，仅根据 JSON 与用户审阅过的分析思考生成报表。

## Skill（id=%s，sha256=%s）
%s

## HTML 硬性要求
1. 完整单文件 HTML，CSS 内联，禁止外链。
2. **主标题**：<title> 与 <h1> 必须完全一致，固定为 JSON 字段 html_title（当前为「%s」），禁止改为「学术」「排班」等任何其他字样。
3. **页眉副标题**（保持格式，勿删）：📅 报告日: … · ⏰ 生成时间: … · 🟢 数据质量 (DQ): PASS 或需关注文案。
4. **Tab 切换**：「付费产品」「免费产品」「站点产品」，JavaScript 切换 panel。
5. 付费 Tab：新增、新增付费、充值、ARPPU、7日预测（Segments key=paid）。
6. 免费 Tab：DAU、7日留存、新增、7日预测（Segments key=free）。
7. 站点 Tab：以**日导量新增**为核心 KPI；7日预测**仅** lead_new_cnt；无异常检测块；数据来自 Segments key=site。
8. 预测表须展示 MethodDetail、SampleDays；carry_forward 须注明样本不足。
9. 禁止编造数据。
10. 显眼展示 AI 标识：%s；<meta name="generator" content="%s"/>
11. 只输出 HTML，不要 markdown 或解释文字。`,
		DailyReportHTMLTitle, skillDoc.ID, skillDoc.SHA256, skillDoc.Body, badgeText, badgeText)
}

func buildLLMHTMLUserPrompt(dataJSON string, meta LLMHTMLMeta, thinking string) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("报告元信息：provider=%s model=%s skill_id=%s\n\n", meta.Provider, meta.Model, meta.SkillID))
	if strings.TrimSpace(thinking) != "" {
		b.WriteString("## 已审阅的数据分析思考\n")
		b.WriteString(thinking)
		b.WriteString("\n\n")
	}
	b.WriteString("## JSON 数据\n")
	b.WriteString(dataJSON)
	return b.String()
}

func modelBadgeText(meta LLMHTMLMeta) string {
	return fmt.Sprintf("🤖 本报告由 AI 生成 · Provider: %s · Model: %s · Skill: %s · %s",
		meta.Provider, meta.Model, meta.SkillID, meta.GeneratedAt)
}

func stripHTMLFences(raw string) string {
	raw = strings.TrimSpace(raw)
	raw = strings.TrimPrefix(raw, "```html")
	raw = strings.TrimPrefix(raw, "```HTML")
	raw = strings.TrimPrefix(raw, "```")
	raw = strings.TrimSuffix(raw, "```")
	return strings.TrimSpace(raw)
}

func ensureModelBadge(html string, meta LLMHTMLMeta) string {
	badge := modelBadgeText(meta)
	if strings.Contains(html, meta.Model) && strings.Contains(html, "AI 生成") {
		return html
	}
	snippet := fmt.Sprintf(
		`<div id="llm-generator-badge" style="position:fixed;bottom:0;left:0;right:0;background:#1e3a5f;color:#e8edf4;text-align:center;padding:8px 12px;font-size:12px;z-index:9999;border-top:1px solid #3b82f6;">%s</div>`,
		badge)
	return injectBeforeBodyClose(html, snippet)
}

// ensureReportTitle 强制主标题为固定文案（防止 LLM 自造「学术/排班」等变体）。
func ensureReportTitle(html, title string) string {
	if title == "" {
		title = DailyReportHTMLTitle
	}
	// 简单替换常见错误 h1 片段
	for _, bad := range []string{"学术", "排班", "运营分析日报"} {
		if strings.Contains(html, bad) && !strings.Contains(title, bad) {
			html = strings.ReplaceAll(html, "<h1>DataPilot 每日运营学术/排班分析报告</h1>", "<h1>"+title+"</h1>")
			html = strings.ReplaceAll(html, "<h1>DataPilot 运营分析日报</h1>", "<h1>"+title+"</h1>")
		}
	}
	if !strings.Contains(html, "<h1>"+title+"</h1>") {
		// 在 <body> 后注入标准 header 块较复杂，仅修补 title 标签
		if idx := strings.Index(strings.ToLower(html), "<title>"); idx >= 0 {
			end := strings.Index(html[idx:], "</title>")
			if end > 0 {
				html = html[:idx] + "<title>" + title + "</title>" + html[idx+end+8:]
			}
		}
	}
	return html
}

func injectBeforeBodyClose(html, snippet string) string {
	lower := strings.ToLower(html)
	idx := strings.LastIndex(lower, "</body>")
	if idx < 0 {
		return html + "\n" + snippet
	}
	return html[:idx] + snippet + "\n" + html[idx:]
}

// LLMVerboseFromEnv 是否输出流水线日志（AGENT_LLM_VERBOSE / AGENT_LLM_DEBUG）。
func LLMVerboseFromEnv() bool {
	v := strings.ToLower(strings.TrimSpace(os.Getenv("AGENT_LLM_VERBOSE")))
	if v == "1" || v == "true" || v == "yes" {
		return true
	}
	v = strings.ToLower(strings.TrimSpace(os.Getenv("AGENT_LLM_DEBUG")))
	return v == "1" || v == "true" || v == "yes"
}
