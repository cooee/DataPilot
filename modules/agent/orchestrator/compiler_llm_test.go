package orchestrator

import (
	"testing"

	"github.com/Caknoooo/go-gin-clean-starter/modules/agent/provider"
	analyticsdto "github.com/Caknoooo/go-gin-clean-starter/modules/analytics/dto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNormalizeLLMPlan_ValidPresetIgnoresBadDataset(t *testing.T) {
	result := &provider.PlanResult{
		Preset: "product-type-compare",
		Query: &analyticsdto.QueryRequest{
			Dataset: "产品数据",
			Metrics: []string{"dau"},
		},
	}
	q, err := normalizeLLMPlan(result, "2026-05-18")
	require.NoError(t, err)
	assert.Equal(t, "dws_product_daily", q.Dataset)
	assert.Contains(t, q.Dimensions, "product_type")
}

func TestNormalizeLLMPlan_ValidDataset(t *testing.T) {
	result := &provider.PlanResult{
		Query: &analyticsdto.QueryRequest{
			Dataset: "dws_product_daily",
			Metrics: []string{"dau"},
			Dimensions: []string{"product_type"},
			DateRange: &analyticsdto.DateRange{From: "2026-05-18", To: "2026-05-18"},
		},
	}
	q, err := normalizeLLMPlan(result, "")
	require.NoError(t, err)
	assert.Equal(t, "dws_product_daily", q.Dataset)
}

func TestNormalizeLLMPlan_Invalid(t *testing.T) {
	_, err := normalizeLLMPlan(&provider.PlanResult{
		Preset: "not-exist",
		Query:  &analyticsdto.QueryRequest{Dataset: "产品数据"},
	}, "2026-05-18")
	require.Error(t, err)
}
