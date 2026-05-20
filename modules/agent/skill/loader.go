package skill

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// Meta 注入 Gemini 的 Skill 元数据（用于 API/CLI 验收）。
type Meta struct {
	Loaded bool   `json:"skill_loaded"`
	ID     string `json:"skill_id,omitempty"`
	Path   string `json:"skill_path,omitempty"`
	SHA256 string `json:"skill_sha256,omitempty"`
}

// Document 已加载的 Skill 文件。
type Document struct {
	Meta
	Body string // 注入 prompt 的正文（不含 YAML frontmatter）
}

const defaultRelPath = "skills/datapilot-daily-ops-report/SKILL.md"

var nameRe = regexp.MustCompile(`(?m)^name:\s*([a-z0-9][a-z0-9-]{0,63})\s*$`)

// DefaultPath 默认 Skill 路径（相对进程 cwd，通常为项目根）。
func DefaultPath() string {
	if p := strings.TrimSpace(os.Getenv("AGENT_SKILL_PATH")); p != "" {
		return p
	}
	return defaultRelPath
}

// IsEnabled 是否启用 Skill 注入（默认 true，AGENT_SKILL_ENABLED=false 关闭）。
func IsEnabled() bool {
	v := strings.ToLower(strings.TrimSpace(os.Getenv("AGENT_SKILL_ENABLED")))
	if v == "0" || v == "false" || v == "no" || v == "off" {
		return false
	}
	return true
}

// LoadFromEnv 按环境变量加载；未启用或文件不存在时返回 (nil, nil)。
func LoadFromEnv() (*Document, error) {
	if !IsEnabled() {
		return nil, nil
	}
	path := DefaultPath()
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	if _, err := os.Stat(abs); os.IsNotExist(err) {
		return nil, nil
	}
	return Load(abs)
}

// Load 读取 SKILL.md 并解析 frontmatter name。
func Load(path string) (*Document, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read skill %s: %w", path, err)
	}
	content := string(raw)
	sum := sha256.Sum256(raw)
	id := parseName(content)
	if id == "" {
		id = strings.TrimSuffix(filepath.Base(filepath.Dir(path)), "/")
	}
	body := stripFrontmatter(content)
	return &Document{
		Meta: Meta{
			Loaded: true,
			ID:     id,
			Path:   path,
			SHA256: hex.EncodeToString(sum[:]),
		},
		Body: strings.TrimSpace(body),
	}, nil
}

func parseName(content string) string {
	m := nameRe.FindStringSubmatch(content)
	if len(m) < 2 {
		return ""
	}
	return m[1]
}

func stripFrontmatter(content string) string {
	content = strings.TrimPrefix(content, "\ufeff")
	if !strings.HasPrefix(content, "---") {
		return content
	}
	rest := content[3:]
	if i := strings.Index(rest, "\n---"); i >= 0 {
		return strings.TrimSpace(rest[i+4:])
	}
	return content
}
