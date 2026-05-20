package main

import (
	"context"
	"log"

	gsheetSvc "github.com/Caknoooo/go-gin-clean-starter/modules/gsheet/service"
	"github.com/Caknoooo/go-gin-clean-starter/pkg/constants"
	"github.com/samber/do"
)

// runSyncSiteGSheet 站点产品同步（gid=553168897，product_type=site）。
func runSyncSiteGSheet(injector *do.Injector, cmd string) {
	args := parseSyncArgs(cmd)

	siteLoader := do.MustInvokeNamed[*gsheetSvc.SiteODSLoader](injector, constants.SiteODSLoader)
	siteDQ := do.MustInvokeNamed[*gsheetSvc.SiteDQChecker](injector, constants.SiteDQChecker)
	etlRunner := do.MustInvokeNamed[*gsheetSvc.ETLRunner](injector, constants.ETLRunner)

	ctx := context.Background()
	etlParams := gsheetSvc.ETLParams{DateFrom: args.dateFrom, DateTo: args.dateTo}
	gid := gsheetSvc.DefaultSiteGID()

	log.Printf("[sync:site] cmd=%s  id=%s  gid=%d  type=site  from=%s  to=%s",
		cmd, args.spreadsheetID, gid,
		args.dateFrom.Format("2006-01-02"), args.dateTo.Format("2006-01-02"))

	switch cmd {
	case "--sync:ods:site":
		syncSiteODS(ctx, siteLoader, siteDQ, args)

	case "--sync:site:all":
		syncSiteODS(ctx, siteLoader, siteDQ, args)
		if err := etlRunner.RunSiteAll(ctx, etlParams); err != nil {
			log.Fatalf("[sync:site:all] ETL failed: %v", err)
		}
		log.Println("[sync:site:all] done (site ODS → DIM → DWD → DWS)")

	default:
		log.Fatalf("unknown site sync command: %s", cmd)
	}
}

func syncSiteODS(ctx context.Context, loader *gsheetSvc.SiteODSLoader, dq *gsheetSvc.SiteDQChecker, args syncArgs) {
	log.Printf("[sync:ods:site] gid=%d product_type=site", gsheetSvc.DefaultSiteGID())
	result, err := loader.Load(ctx, args.spreadsheetID, gsheetSvc.DefaultSiteGID(), "")
	if err != nil {
		log.Fatalf("[sync:ods:site] load failed: %v", err)
	}
	log.Printf("[sync:ods:site] total=%d inserted=%d skipped=%d warnings=%d",
		result.Total, result.Inserted, result.Skipped, len(result.Warnings))

	report, err := dq.Check(ctx, args.dateFrom, args.dateTo)
	if err != nil {
		log.Printf("[sync:ods:site] DQ check error (non-fatal): %v", err)
		return
	}
	gsheetSvc.PrintReport(report)
	if !report.OK {
		log.Printf("[sync:ods:site] WARNING: %s", gsheetSvc.SummaryLine(report))
	}
}
