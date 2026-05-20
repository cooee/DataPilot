package report

import "testing"

func TestBuildSegments_paidForecastMetrics(t *testing.T) {
	p := &DailyReportPayload{
		ReportDate: "2026-05-18",
		Compare: []TypeCompareRow{
			{ProductType: "paid", DAU: 1000, Recharge: 5000, NewUsers: 100, NewPaying: 20, ARPPU: 5},
			{ProductType: "free", DAU: 50000, RetentionD7: 0.35, NewUsers: 5000},
		},
		TrendHistory: []TrendPoint{
			{Date: "2026-05-17", ProductType: "paid", Recharge: 4000, NewUsers: 90, NewPaying: 18, ARPPU: 4.5},
			{Date: "2026-05-18", ProductType: "paid", Recharge: 5000, NewUsers: 100, NewPaying: 20, ARPPU: 5},
			{Date: "2026-05-17", ProductType: "free", DAU: 48000, RetentionD7: 0.34, NewUsers: 4800},
			{Date: "2026-05-18", ProductType: "free", DAU: 50000, RetentionD7: 0.35, NewUsers: 5000},
		},
	}
	segs := BuildSegments(p)
	if len(segs) != 2 {
		t.Fatalf("expected 2 segments, got %d", len(segs))
	}
	if segs[0].Key != "paid" || len(segs[0].Forecasts) == 0 {
		t.Fatalf("paid forecasts missing: %+v", segs[0])
	}
	if len(segs[1].Forecasts) == 0 {
		t.Fatal("free forecasts missing")
	}
	if len(segs[0].AnalystNotes) == 0 {
		t.Fatal("expected analyst notes")
	}
}
