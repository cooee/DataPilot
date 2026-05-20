package service

import (
	"strings"
	"testing"

	"github.com/Caknoooo/go-gin-clean-starter/modules/analytics/catalog"
	"github.com/Caknoooo/go-gin-clean-starter/modules/analytics/dto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildQuery_ProductTypeComparison(t *testing.T) {
	req := &dto.QueryRequest{
		Dataset:    "dws_product_daily",
		Metrics:    []string{"dau", "recharge_total_amt", "retention_d7_ratio"},
		Dimensions: []string{"product_type"},
		DateRange: &dto.DateRange{
			From: "2026-05-18",
			To:   "2026-05-18",
		},
		Filters: []dto.Filter{
			{Field: "product_type", Operator: "in", Value: []string{"paid", "free"}},
		},
		Limit: 100,
	}

	built, cols, err := buildQuery(req)
	require.NoError(t, err)
	assert.Contains(t, built.SQL, "FROM dws_product_daily")
	assert.Contains(t, built.SQL, "GROUP BY product_type")
	assert.True(t, strings.Index(built.SQL, "WHERE") < strings.Index(built.SQL, "GROUP BY"), "WHERE must precede GROUP BY")
	assert.Contains(t, built.SQL, "sum(dau) AS dau")
	assert.Contains(t, built.SQL, "avg(retention_d7_ratio) AS retention_d7_ratio")
	assert.Contains(t, built.SQL, "product_type IN (?,?)")
	assert.Equal(t, []any{"2026-05-18", "2026-05-18", "paid", "free"}, built.Args)
	assert.Len(t, cols, 4)
}

func TestBuildQuery_DetailRows(t *testing.T) {
	req := &dto.QueryRequest{
		Dataset: "dws_product_daily",
		Metrics: []string{"dau", "recharge_total_amt"},
		Dimensions: []string{"date", "product_code", "product_name", "team"},
		DateRange: &dto.DateRange{From: "2026-05-18", To: "2026-05-18"},
		OrderBy: []dto.OrderBy{{Field: "recharge_total_amt", Desc: true}},
	}

	built, _, err := buildQuery(req)
	require.NoError(t, err)
	assert.Contains(t, built.SQL, "GROUP BY date, product_code, product_name, team")
	assert.Contains(t, built.SQL, "sum(recharge_total_amt) AS recharge_total_amt")
	assert.Contains(t, built.SQL, "ORDER BY recharge_total_amt DESC")
}

func TestBuildQuery_RejectUnknownDataset(t *testing.T) {
	_, _, err := buildQuery(&dto.QueryRequest{
		Dataset: "raw_sql_injection",
		Metrics: []string{"dau"},
	})
	require.Error(t, err)
}

func TestBuildQuery_RejectUnknownMetric(t *testing.T) {
	_, _, err := buildQuery(&dto.QueryRequest{
		Dataset: "dws_product_daily",
		Metrics: []string{"DROP TABLE"},
	})
	require.Error(t, err)
}

func TestBuildQuery_ADSView_NoGroupByWhenPreAggregated(t *testing.T) {
	req := &dto.QueryRequest{
		Dataset: "ads_product_health_overview",
		Metrics: []string{"health_score", "avg_dau_30d"},
		Dimensions: []string{"product_code", "product_name", "team"},
		OrderBy: []dto.OrderBy{{Field: "health_score", Desc: true}},
		Limit: 10,
	}
	built, _, err := buildQuery(req)
	require.NoError(t, err)
	assert.Contains(t, built.SQL, "FROM ads_product_health_overview")
	assert.Contains(t, built.SQL, "health_score")
	assert.NotContains(t, built.SQL, "GROUP BY")
	assert.NotContains(t, built.SQL, "sum(health_score)")
	assert.Contains(t, built.SQL, "ORDER BY health_score DESC")
}

func TestNeedsGroupBy(t *testing.T) {
	ds, _ := catalog.GetDataset("dws_product_daily")
	reqAgg := &dto.QueryRequest{
		Dataset:    "dws_product_daily",
		Metrics:    []string{"dau"},
		Dimensions: []string{"product_type"},
	}
	assert.True(t, needsGroupBy(reqAgg, ds))

	dsADS, _ := catalog.GetDataset("ads_product_health_overview")
	reqNoAgg := &dto.QueryRequest{
		Dataset:    "ads_product_health_overview",
		Metrics:    []string{"health_score"},
		Dimensions: []string{"team"},
	}
	assert.False(t, needsGroupBy(reqNoAgg, dsADS))
}

func TestBuildQuery_AnomalyDetection(t *testing.T) {
	req := &dto.QueryRequest{
		Dataset: "ads_product_anomaly_daily",
		Metrics: []string{"value", "upper_3sigma", "lower_3sigma", "is_anomaly"},
		Dimensions: []string{
			"date", "product_code", "product_name", "metric", "anomaly_direction",
		},
		DateRange: &dto.DateRange{From: "2026-05-18", To: "2026-05-18"},
		Filters:   []dto.Filter{{Field: "is_anomaly", Operator: "eq", Value: 1}},
		Limit:     50,
	}
	built, _, err := buildQuery(req)
	require.NoError(t, err)
	assert.Contains(t, built.SQL, "FROM ads_product_anomaly_daily")
	assert.Contains(t, built.SQL, "WHERE")
	assert.True(t, strings.Index(built.SQL, "WHERE") < strings.Index(built.SQL, "GROUP BY") || !strings.Contains(built.SQL, "GROUP BY"))
	assert.NotContains(t, built.SQL, "GROUP BY")
	assert.Contains(t, built.SQL, "is_anomaly = ?")
	assert.Equal(t, []any{"2026-05-18", "2026-05-18", 1}, built.Args)
}

func TestBuildQuery_MixedAggUsesAnyForPreAggregated(t *testing.T) {
	// 若未来 ADS 视图混合 AggNone 与 sum，GROUP BY 时 AggNone 应包 any()
	req := &dto.QueryRequest{
		Dataset:    "dws_product_daily",
		Metrics:    []string{"dau", "retention_d7_ratio"},
		Dimensions: []string{"team"},
		DateRange:  &dto.DateRange{From: "2026-05-18", To: "2026-05-18"},
	}
	built, _, err := buildQuery(req)
	require.NoError(t, err)
	assert.Contains(t, built.SQL, "sum(dau) AS dau")
	assert.Contains(t, built.SQL, "avg(retention_d7_ratio) AS retention_d7_ratio")
}
