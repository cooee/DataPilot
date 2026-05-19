package main

import (
	"context"
	"flag"
	"log"
	"os"
	"time"

	gsheetSvc "github.com/Caknoooo/go-gin-clean-starter/modules/gsheet/service"
	"github.com/Caknoooo/go-gin-clean-starter/pkg/constants"
	"github.com/samber/do"
)

const (
	defaultSpreadsheetID = "1sJZBMAHBa3QtmSKQ8oLGaiC_w-8kloGJizRNHnbQ-jw"
	defaultGID           = 0
)

// syncArgs 解析 --sync:xxx 后面紧跟的 flag 参数。
type syncArgs struct {
	spreadsheetID string
	gid           int
	productType   string // 'paid' | 'free'
	dateFrom      time.Time
	dateTo        time.Time
}

// knownFreeGIDs 已知的免费产品 sheet gid（可在此追加）
var knownFreeGIDs = map[int]bool{
	469519483: true,
}

func parseSyncArgs(cmd string) syncArgs {
	fs := flag.NewFlagSet(cmd, flag.ExitOnError)
	id := fs.String("id", defaultSpreadsheetID, "Spreadsheet ID or URL")
	gid := fs.Int("gid", defaultGID, "Sheet tab gid")
	ptype := fs.String("type", "", "Product type: paid|free (auto-detected from gid if empty)")
	from := fs.String("from", "", "Start date YYYY-MM-DD (default: 90 days ago)")
	to := fs.String("to", "", "End date YYYY-MM-DD (default: today)")

	// 找到 --sync:xxx 后面的参数
	var rest []string
	for i, a := range os.Args[1:] {
		if a == cmd {
			rest = os.Args[i+2:]
			break
		}
	}
	_ = fs.Parse(rest)

	now := time.Now()
	dateFrom := now.AddDate(0, 0, -90).Truncate(24 * time.Hour)
	dateTo := now.Truncate(24 * time.Hour)

	if *from != "" {
		if t, err := time.Parse("2006-01-02", *from); err == nil {
			dateFrom = t
		} else {
			log.Fatalf("invalid -from date: %v", err)
		}
	}
	if *to != "" {
		if t, err := time.Parse("2006-01-02", *to); err == nil {
			dateTo = t
		} else {
			log.Fatalf("invalid -to date: %v", err)
		}
	}

	// 自动推断 productType
	productType := *ptype
	if productType == "" {
		if knownFreeGIDs[*gid] {
			productType = "free"
		} else {
			productType = "paid"
		}
	}

	sid := extractSpreadsheetID(*id)
	return syncArgs{spreadsheetID: sid, gid: *gid, productType: productType, dateFrom: dateFrom, dateTo: dateTo}
}

// runSyncGSheet 是 --sync:xxx 命令的总入口。
func runSyncGSheet(injector *do.Injector, cmd string) {
	args := parseSyncArgs(cmd)

	loader := do.MustInvokeNamed[*gsheetSvc.ODSLoader](injector, constants.ODSLoader)
	etlRunner := do.MustInvokeNamed[*gsheetSvc.ETLRunner](injector, constants.ETLRunner)
	dqChecker := do.MustInvokeNamed[*gsheetSvc.DQChecker](injector, constants.DQChecker)

	ctx := context.Background()
	etlParams := gsheetSvc.ETLParams{DateFrom: args.dateFrom, DateTo: args.dateTo}

	log.Printf("[sync] cmd=%s  id=%s  gid=%d  type=%s  from=%s  to=%s",
		cmd, args.spreadsheetID, args.gid, args.productType,
		args.dateFrom.Format("2006-01-02"), args.dateTo.Format("2006-01-02"))

	switch cmd {
	case "--sync:ods":
		syncODS(ctx, loader, dqChecker, args)
		log.Printf("[sync:ods] product_type=%s", args.productType)

	case "--sync:dim":
		syncETLStep(ctx, etlRunner, etlParams, "ODS→DIM", func(p gsheetSvc.ETLParams) error {
			return etlRunner.RunStep(ctx, "database/clickhouse/etl/01_ods_to_dim.sql", p)
		})

	case "--sync:dwd":
		syncETLStep(ctx, etlRunner, etlParams, "ODS→DWD", func(p gsheetSvc.ETLParams) error {
			return etlRunner.RunStep(ctx, "database/clickhouse/etl/02_ods_to_dwd.sql", p)
		})

	case "--sync:dws":
		syncETLStep(ctx, etlRunner, etlParams, "DWD→DWS", func(p gsheetSvc.ETLParams) error {
			return etlRunner.RunDWS(ctx, p)
		})

	case "--sync:all":
		syncODS(ctx, loader, dqChecker, args)
		if err := etlRunner.RunAll(ctx, etlParams); err != nil {
			log.Fatalf("[sync:all] ETL failed: %v", err)
		}
		log.Println("[sync:all] done")

	default:
		log.Fatalf("unknown sync command: %s", cmd)
	}
}

func syncODS(ctx context.Context, loader *gsheetSvc.ODSLoader, dq *gsheetSvc.DQChecker, args syncArgs) {
	result, err := loader.Load(ctx, args.spreadsheetID, args.gid, args.productType, "")
	if err != nil {
		log.Fatalf("[sync:ods] load failed: %v", err)
	}
	log.Printf("[sync:ods] total=%d inserted=%d skipped=%d warnings=%d",
		result.Total, result.Inserted, result.Skipped, len(result.Warnings))

	report, err := dq.Check(ctx, args.dateFrom, args.dateTo)
	if err != nil {
		log.Printf("[sync:ods] DQ check error (non-fatal): %v", err)
		return
	}
	gsheetSvc.PrintReport(report)
	if !report.OK {
		log.Printf("[sync:ods] WARNING: %s", gsheetSvc.SummaryLine(report))
	}
}

func syncETLStep(_ context.Context, _ *gsheetSvc.ETLRunner, p gsheetSvc.ETLParams, name string, fn func(gsheetSvc.ETLParams) error) {
	if err := fn(p); err != nil {
		log.Fatalf("[sync] %s failed: %v", name, err)
	}
	log.Printf("[sync] %s done", name)
}
