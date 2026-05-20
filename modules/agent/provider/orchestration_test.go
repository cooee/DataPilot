package provider

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParsePlanJSON(t *testing.T) {
	raw := `{"intent":"metric_qa","preset":"product-type-compare","interpretation":"对比","query":{"dataset":"dws_product_daily","metrics":["dau"],"dimensions":["product_type"],"date_range":{"from":"2026-05-18","to":"2026-05-18"},"limit":50}}`
	plan, err := parsePlanJSON(raw)
	require.NoError(t, err)
	assert.Equal(t, "metric_qa", plan.Intent)
	assert.Equal(t, "product-type-compare", plan.Preset)
	assert.Equal(t, "dws_product_daily", plan.Query.Dataset)
}

func TestParsePlanJSON_StripsMarkdownFence(t *testing.T) {
	raw := "```json\n{\"intent\":\"anomaly_list\",\"preset\":\"anomaly-detection\",\"interpretation\":\"x\",\"query\":{\"dataset\":\"ads_product_anomaly_daily\",\"metrics\":[\"value\"],\"limit\":10}}\n```"
	plan, err := parsePlanJSON(raw)
	require.NoError(t, err)
	assert.Equal(t, "anomaly-detection", plan.Preset)
}
