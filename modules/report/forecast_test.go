package report

import "testing"

func TestBuildForecast7d_linear(t *testing.T) {
	history := []TrendPoint{
		{Date: "2026-05-10", ProductType: "paid", DAU: 100, Recharge: 1000},
		{Date: "2026-05-11", ProductType: "paid", DAU: 110, Recharge: 1100},
		{Date: "2026-05-12", ProductType: "paid", DAU: 120, Recharge: 1200},
	}
	fc := BuildForecast7d(history, "2026-05-12")
	if len(fc) != 7 {
		t.Fatalf("expected 7 forecast rows, got %d", len(fc))
	}
	if fc[0].Recharge <= 1200 {
		t.Fatalf("expected upward recharge forecast, got %.2f", fc[0].Recharge)
	}
}

func TestBuildMetricForecasts_7dRegression(t *testing.T) {
	history := []TrendPoint{
		{Date: "2026-05-11", ProductType: "paid", Recharge: 500000},
		{Date: "2026-05-12", ProductType: "paid", Recharge: 520000},
		{Date: "2026-05-13", ProductType: "paid", Recharge: 540000},
		{Date: "2026-05-14", ProductType: "paid", Recharge: 560000},
		{Date: "2026-05-15", ProductType: "paid", Recharge: 580000},
		{Date: "2026-05-16", ProductType: "paid", Recharge: 600000},
		{Date: "2026-05-17", ProductType: "paid", Recharge: 610000},
		{Date: "2026-05-18", ProductType: "paid", Recharge: 622150},
	}
	fc := BuildMetricForecasts(history, "2026-05-18", "paid", paidForecastMetrics)
	var recharge []MetricForecast
	for _, r := range fc {
		if r.MetricKey == "recharge_total_amt" {
			recharge = append(recharge, r)
		}
	}
	if len(recharge) != 7 {
		t.Fatalf("expected 7 recharge forecasts, got %d", len(recharge))
	}
	if recharge[0].Method != "linear_regression_7d" {
		t.Fatalf("expected linear_regression_7d, got %s", recharge[0].Method)
	}
	if recharge[0].Value == recharge[6].Value {
		t.Fatalf("expected varying forecast values, got flat %.2f", recharge[0].Value)
	}
}

func TestBuildMetricForecasts_carryForwardOneDay(t *testing.T) {
	history := []TrendPoint{
		{Date: "2026-05-18", ProductType: "paid", Recharge: 622150},
	}
	defs := []forecastMetricDef{paidForecastMetrics[2]} // recharge only
	fc := BuildMetricForecasts(history, "2026-05-18", "paid", defs)
	if fc[0].Method != "carry_forward" {
		t.Fatalf("expected carry_forward, got %s", fc[0].Method)
	}
	if fc[0].Value != 622150 || fc[6].Value != 622150 {
		t.Fatalf("carry forward should be flat, got %.0f and %.0f", fc[0].Value, fc[6].Value)
	}
}

func TestBuildInsights_dqFail(t *testing.T) {
	p := DailyReportPayload{DQOK: false, Anomalies: nil}
	ins := BuildInsights(p)
	if len(ins) == 0 {
		t.Fatal("expected insights")
	}
}
