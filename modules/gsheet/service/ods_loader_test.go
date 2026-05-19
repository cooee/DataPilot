package service_test

import (
	"strings"
	"testing"

	"github.com/Caknoooo/go-gin-clean-starter/modules/gsheet/mapper"
	"github.com/Caknoooo/go-gin-clean-starter/modules/gsheet/service"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateHeaders_OK(t *testing.T) {
	// 直接访问 validateHeaders 的公开包装
	err := service.ValidateHeadersExported(mapper.ExpectedHeaders)
	require.NoError(t, err)
}

func TestValidateHeaders_TooFewCols(t *testing.T) {
	err := service.ValidateHeadersExported([]string{"日期", "产品名称"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cols")
}

func TestValidateHeaders_Mismatch(t *testing.T) {
	header := make([]string, len(mapper.ExpectedHeaders))
	copy(header, mapper.ExpectedHeaders)
	header[2] = "WRONG"
	err := service.ValidateHeadersExported(header)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "mismatch")
}

func TestDirtyRatioHelper(t *testing.T) {
	assert.InDelta(t, 0.1, service.DirtyRatioExported(1, 10), 1e-9)
	assert.Equal(t, 0.0, service.DirtyRatioExported(0, 0))
}

func TestMinHelper(t *testing.T) {
	assert.Equal(t, 3, service.MinExported(3, 8))
	assert.Equal(t, 3, service.MinExported(8, 3))
	assert.Equal(t, 5, service.MinExported(5, 5))
}

// TestSourceDefault 验证 source 自动推断格式
func TestSourceDefault(t *testing.T) {
	id := "1sJZBMAHBa3QtmSKQ8oLGaiC_w-8kloGJizRNHnbQ-jw"
	src := service.BuildSource(id, 0)
	assert.True(t, strings.HasPrefix(src, "gsheet:"))
	assert.Contains(t, src, "gid0")
}
