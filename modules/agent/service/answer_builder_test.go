package service

import (
	"testing"

	analyticsdto "github.com/Caknoooo/go-gin-clean-starter/modules/analytics/dto"
	"github.com/stretchr/testify/assert"
)

func TestBuildAnswer_SiteProductSummary(t *testing.T) {
	result := &analyticsdto.QueryResult{
		Rows: []map[string]any{
			{"date": "2026-05-18", "dau": uint64(8000), "lead_new_cnt": uint64(50), "lead_recharge_amt": 1200.0},
		},
	}
	ans := buildAnswer("site-product-summary", result, "2026-05-18")
	assert.Contains(t, ans, "站点产品汇总")
	assert.Contains(t, ans, "导量充值")
}

func TestBuildAnswer_SiteProductDetail(t *testing.T) {
	result := &analyticsdto.QueryResult{
		Rows: []map[string]any{
			{
				"product_code": "JHA-1006", "product_name": "91看片", "team": "增长1组",
				"dau": uint64(5162), "lead_new_cnt": uint64(21), "lead_recharge_amt": 200.0,
			},
		},
	}
	ans := buildAnswer("site-product-detail", result, "2026-05-18")
	assert.Contains(t, ans, "JHA-1006")
	assert.Contains(t, ans, "91看片")
}
