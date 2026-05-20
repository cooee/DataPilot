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
		SiteSummary: SiteSummaryRow{DAU: 8000, LeadNewCnt: 50, LeadRechargeAmt: 1200},
		SiteTrendHistory: []SiteTrendPoint{
			{Date: "2026-05-17", DAU: 7500, LeadNewCnt: 45, LeadRechargeAmt: 1000},
			{Date: "2026-05-18", DAU: 8000, LeadNewCnt: 50, LeadRechargeAmt: 1200},
		},
	}
	segs := BuildSegments(p)
	if len(segs) != 3 {
		t.Fatalf("expected 3 segments (paid/free/site), got %d", len(segs))
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
