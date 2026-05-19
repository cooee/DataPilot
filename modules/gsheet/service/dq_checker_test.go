package service_test

import (
	"testing"
	"time"

	"github.com/Caknoooo/go-gin-clean-starter/modules/gsheet/service"
	"github.com/stretchr/testify/assert"
)

func makeDQReport(rules []string, passed []bool) *service.DQReport {
	r := &service.DQReport{
		DateFrom: time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC),
		DateTo:   time.Date(2026, 3, 31, 0, 0, 0, 0, time.UTC),
		OK:       true,
	}
	for i, rule := range rules {
		cnt := uint64(0)
		if !passed[i] {
			cnt = 1
			r.OK = false
		}
		r.Results = append(r.Results, service.DQResult{
			Rule:    rule,
			Passed:  passed[i],
			Detail:  "test",
			FailCnt: cnt,
		})
	}
	return r
}

func TestSummaryLine_AllPass(t *testing.T) {
	r := makeDQReport([]string{"rule_a", "rule_b"}, []bool{true, true})
	assert.Equal(t, "DQ OK", service.SummaryLine(r))
}

func TestSummaryLine_SomeFail(t *testing.T) {
	r := makeDQReport([]string{"rule_a", "rule_b", "rule_c"}, []bool{true, false, false})
	line := service.SummaryLine(r)
	assert.Contains(t, line, "DQ FAIL")
	assert.Contains(t, line, "rule_b")
	assert.Contains(t, line, "rule_c")
	assert.NotContains(t, line, "rule_a")
}

func TestPrintReport_NoError(t *testing.T) {
	r := makeDQReport([]string{"duplicate", "negative_dau"}, []bool{true, false})
	// 只验证不 panic
	service.PrintReport(r)
}
