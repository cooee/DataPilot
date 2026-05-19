package providers

import (
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	gsheetRepo "github.com/Caknoooo/go-gin-clean-starter/modules/gsheet/repository"
	gsheetSvc "github.com/Caknoooo/go-gin-clean-starter/modules/gsheet/service"
	"github.com/Caknoooo/go-gin-clean-starter/pkg/constants"
	"github.com/samber/do"
)

func InitGSheetServices(injector *do.Injector) {
	// ---- repositories ----
	do.ProvideNamed(injector, constants.ODSRepo, func(i *do.Injector) (*gsheetRepo.ODSRepo, error) {
		conn := do.MustInvokeNamed[driver.Conn](i, constants.ClickHouseDB)
		return gsheetRepo.NewODSRepo(conn), nil
	})

	do.ProvideNamed(injector, constants.EtlRepo, func(i *do.Injector) (*gsheetRepo.EtlRepo, error) {
		conn := do.MustInvokeNamed[driver.Conn](i, constants.ClickHouseDB)
		return gsheetRepo.NewEtlRepo(conn), nil
	})

	// ---- services ----
	do.ProvideNamed(injector, constants.CSVFetcher, func(i *do.Injector) (*gsheetSvc.CSVFetcher, error) {
		return gsheetSvc.NewCSVFetcher(), nil
	})

	do.ProvideNamed(injector, constants.ODSLoader, func(i *do.Injector) (*gsheetSvc.ODSLoader, error) {
		fetcher := do.MustInvokeNamed[*gsheetSvc.CSVFetcher](i, constants.CSVFetcher)
		repo := do.MustInvokeNamed[*gsheetRepo.ODSRepo](i, constants.ODSRepo)
		return gsheetSvc.NewODSLoader(fetcher, repo), nil
	})

	do.ProvideNamed(injector, constants.ETLRunner, func(i *do.Injector) (*gsheetSvc.ETLRunner, error) {
		repo := do.MustInvokeNamed[*gsheetRepo.EtlRepo](i, constants.EtlRepo)
		return gsheetSvc.NewETLRunner(repo), nil
	})

	do.ProvideNamed(injector, constants.DQChecker, func(i *do.Injector) (*gsheetSvc.DQChecker, error) {
		conn := do.MustInvokeNamed[driver.Conn](i, constants.ClickHouseDB)
		return gsheetSvc.NewDQChecker(conn), nil
	})
}
