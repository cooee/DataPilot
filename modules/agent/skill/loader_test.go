package skill

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoad_ParseNameAndSHA(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "SKILL.md")
	err := os.WriteFile(path, []byte(`---
name: test-skill
description: x
---
# Body
line2
`), 0o644)
	require.NoError(t, err)

	doc, err := Load(path)
	require.NoError(t, err)
	assert.True(t, doc.Loaded)
	assert.Equal(t, "test-skill", doc.ID)
	assert.Contains(t, doc.Body, "# Body")
	assert.Len(t, doc.SHA256, 64)
}

func TestLoadFromEnv_Disabled(t *testing.T) {
	t.Setenv("AGENT_SKILL_ENABLED", "false")
	doc, err := LoadFromEnv()
	require.NoError(t, err)
	assert.Nil(t, doc)
}
