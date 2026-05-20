package orchestrator

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRulePlanner_ProductTypeCompare(t *testing.T) {
	p := NewRulePlanner(nil)
	plan, err := p.Plan("2026-05-18 付费和免费产品日活充值对比", IntentMetricQA, "")
	require.NoError(t, err)
	assert.Equal(t, IntentMetricQA, plan.Intent)
	assert.Equal(t, "product-type-compare", plan.Preset)
	assert.Equal(t, "2026-05-18", plan.Query.DateRange.From)
}

func TestRulePlanner_AnomalyIntent(t *testing.T) {
	p := NewRulePlanner(nil)
	plan, err := p.Plan("昨天有哪些异常产品", "", "")
	require.NoError(t, err)
	assert.Equal(t, IntentAnomalyList, plan.Intent)
	assert.Equal(t, "anomaly-detection", plan.Preset)
}

func TestRulePlanner_HealthPreset(t *testing.T) {
	p := NewRulePlanner(nil)
	plan, err := p.Plan("产品健康度排名", IntentMetricQA, "2026-05-18")
	require.NoError(t, err)
	assert.Equal(t, "product-health", plan.Preset)
}

func TestRulePlanner_SiteProductSummary(t *testing.T) {
	p := NewRulePlanner(nil)
	plan, err := p.Plan("2026-05-18 站点产品日活跃和导量充值汇总", IntentMetricQA, "")
	require.NoError(t, err)
	assert.Equal(t, "site-product-summary", plan.Preset)
	assert.Equal(t, "dws_site_product_daily", plan.Query.Dataset)
}

func TestRulePlanner_SiteProductDetail(t *testing.T) {
	p := NewRulePlanner(nil)
	plan, err := p.Plan("站点产品明细排行", IntentMetricQA, "2026-05-18")
	require.NoError(t, err)
	assert.Equal(t, "site-product-detail", plan.Preset)
}

func TestRulePlanner_SiteNotConfusedWithPaidFree(t *testing.T) {
	p := NewRulePlanner(nil)
	plan, err := p.Plan("2026-05-18 付费和免费对比", IntentMetricQA, "")
	require.NoError(t, err)
	assert.Equal(t, "product-type-compare", plan.Preset)
}
