package report

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestStripHTMLFences(t *testing.T) {
	raw := "```html\n<!DOCTYPE html><html></html>\n```"
	out := stripHTMLFences(raw)
	assert.True(t, strings.HasPrefix(out, "<!DOCTYPE"))
}

func TestEnsureModelBadge_injects(t *testing.T) {
	html := "<!DOCTYPE html><html><body><h1>test</h1></body></html>"
	meta := LLMHTMLMeta{Provider: "gemini", Model: "gemini-2.0-flash", SkillID: "datapilot-daily-ops-report", GeneratedAt: "2026-05-20"}
	out := ensureModelBadge(html, meta)
	assert.Contains(t, out, "llm-generator-badge")
	assert.Contains(t, out, "gemini-2.0-flash")
	assert.Contains(t, out, "AI 生成")
}

func TestEnsureModelBadge_skipsIfPresent(t *testing.T) {
	meta := LLMHTMLMeta{Provider: "gemini", Model: "gemini-2.0-flash", SkillID: "x", GeneratedAt: "t"}
	html := "<html><body>AI 生成 gemini-2.0-flash</body></html>"
	out := ensureModelBadge(html, meta)
	assert.Equal(t, html, out)
}
