package main

import (
	"context"
	"flag"
	"log"
	"time"

	analyticsCLI "github.com/Caknoooo/go-gin-clean-starter/modules/analytics/cli"
	analyticsSvc "github.com/Caknoooo/go-gin-clean-starter/modules/analytics/service"
	gsheetSvc "github.com/Caknoooo/go-gin-clean-starter/modules/gsheet/service"
	"github.com/Caknoooo/go-gin-clean-starter/pkg/constants"
	"github.com/samber/do"
)

// dailyReportPresets 每日运营日报固定输出的 Analytics 场景。
var dailyReportPresets = []struct {
	title  string
	preset string
}{
	{"① 付费 vs 免费汇总（报告日）", "product-type-compare"},
	{"② 小组日表现（近 7 日）", "team-performance"},
	{"③ 产品健康度 Top 20", "product-health"},
}

type dailyReportArgs struct {
	dateFrom time.Time
	dateTo   time.Time
	reportOn string // 报告日 YYYY-MM-DD，Analytics 单日场景使用
	format   string
	skipSync bool
}

func runDailyReport(injector *do.Injector) {
	args := parseDailyReportArgs()

	loader := do.MustInvokeNamed[*gsheetSvc.ODSLoader](injector, constants.ODSLoader)
	etlRunner := do.MustInvokeNamed[*gsheetSvc.ETLRunner](injector, constants.ETLRunner)
	dqChecker := do.MustInvokeNamed[*gsheetSvc.DQChecker](injector, constants.DQChecker)
	analytics := do.MustInvoke[analyticsSvc.AnalyticsService](injector)

	ctx := context.Background()
	etlParams := gsheetSvc.ETLParams{DateFrom: args.dateFrom, DateTo: args.dateTo}

	log.Printf("[daily-report] sync_range=%s~%s report_date=%s",
		args.dateFrom.Format("2006-01-02"), args.dateTo.Format("2006-01-02"), args.reportOn)

	if !args.skipSync {
		// 付费 + 免费 ODS（各带 DQ）
		syncODS(ctx, loader, dqChecker, syncArgs{
			spreadsheetID: defaultSpreadsheetID,
			gid:           0,
			productType:   "paid",
			dateFrom:      args.dateFrom,
			dateTo:        args.dateTo,
		})
		syncODS(ctx, loader, dqChecker, syncArgs{
			spreadsheetID: defaultSpreadsheetID,
			gid:           469519483,
			productType:   "free",
			dateFrom:      args.dateFrom,
			dateTo:        args.dateTo,
		})

		if err := etlRunner.RunAll(ctx, etlParams); err != nil {
			log.Fatalf("[daily-report] ETL failed: %v", err)
		}
		log.Println("[daily-report] ETL done (DIM → DWD → DWS)")

		// 汇总 DQ（全区间）
		report, err := dqChecker.Check(ctx, args.dateFrom, args.dateTo)
		if err != nil {
			log.Fatalf("[daily-report] DQ check failed: %v", err)
		}
		gsheetSvc.PrintReport(report)
		if !report.OK {
			log.Printf("[daily-report] WARNING: %s", gsheetSvc.SummaryLine(report))
		}
	} else {
		log.Println("[daily-report] skip sync/ETL (-skip-sync)")
	}

	// Analytics 三段 preset
	trendFrom := args.dateTo.AddDate(0, 0, -6).Format("2006-01-02")
	trendTo := args.dateTo.Format("2006-01-02")

	for _, p := range dailyReportPresets {
		log.Printf("\n[daily-report] ===== %s =====", p.title)
		fn, ok := analyticsCLI.Presets[p.preset]
		if !ok {
			log.Fatalf("[daily-report] unknown preset %q", p.preset)
		}

		from, to := args.reportOn, args.reportOn
		if p.preset == "team-performance" {
			from, to = trendFrom, trendTo
		}

		req, err := fn(from, to)
		if err != nil {
			log.Fatalf("[daily-report] preset %s: %v", p.preset, err)
		}

		result, meta, err := analytics.Query(ctx, req)
		if err != nil {
			log.Fatalf("[daily-report] query %s: %v", p.preset, err)
		}
		if err := analyticsCLI.PrintResult(args.format, result, meta); err != nil {
			log.Fatalf("[daily-report] output: %v", err)
		}
	}

	log.Println("\n[daily-report] ===== DONE =====")
}

func parseDailyReportArgs() dailyReportArgs {
	fs := flag.NewFlagSet("--report:daily", flag.ExitOnError)
	from := fs.String("from", "", "同步起始日期 YYYY-MM-DD（默认：报告日）")
	to := fs.String("to", "", "同步结束日期 YYYY-MM-DD（默认：报告日）")
	reportDate := fs.String("date", "", "报告日 YYYY-MM-DD（Analytics 汇总日，默认昨天）")
	format := fs.String("format", "table", "Analytics 输出: table | json")
	skipSync := fs.Bool("skip-sync", false, "跳过 Sheet 同步与 ETL，仅跑 Analytics")

	rest := argsAfterCommand("--report:daily")
	_ = fs.Parse(rest)

	now := time.Now().Truncate(24 * time.Hour)
	reportOn := now.AddDate(0, 0, -1).Format("2006-01-02")
	if *reportDate != "" {
		reportOn = *reportDate
	}
	if _, err := time.Parse("2006-01-02", reportOn); err != nil {
		log.Fatalf("invalid -date: %v", err)
	}

	dateFrom := reportOn
	dateTo := reportOn
	if *from != "" {
		dateFrom = *from
	}
	if *to != "" {
		dateTo = *to
	}
	if *from == "" && *to != "" {
		dateFrom = dateTo
	}
	if *to == "" && *from != "" {
		dateTo = dateFrom
	}

	df, err := time.Parse("2006-01-02", dateFrom)
	if err != nil {
		log.Fatalf("invalid -from: %v", err)
	}
	dt, err := time.Parse("2006-01-02", dateTo)
	if err != nil {
		log.Fatalf("invalid -to: %v", err)
	}
	if df.After(dt) {
		log.Fatal("-from must be <= -to")
	}

	return dailyReportArgs{
		dateFrom: df,
		dateTo:   dt,
		reportOn: reportOn,
		format:   *format,
		skipSync: *skipSync,
	}
}
