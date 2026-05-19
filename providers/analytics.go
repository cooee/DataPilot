package providers

import (
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	analyticsCtrl "github.com/Caknoooo/go-gin-clean-starter/modules/analytics/controller"
	analyticsRepo "github.com/Caknoooo/go-gin-clean-starter/modules/analytics/repository"
	analyticsSvc "github.com/Caknoooo/go-gin-clean-starter/modules/analytics/service"
	"github.com/Caknoooo/go-gin-clean-starter/pkg/constants"
	"github.com/samber/do"
	"gorm.io/gorm"
)

func InitAnalyticsServices(injector *do.Injector) {
	do.Provide(injector, func(i *do.Injector) (analyticsSvc.AnalyticsService, error) {
		conn := do.MustInvokeNamed[driver.Conn](i, constants.ClickHouseDB)
		db := do.MustInvokeNamed[*gorm.DB](i, constants.DB)
		chRepo := analyticsRepo.NewCHQueryRepo(conn)
		metricRepo := analyticsRepo.NewMetricDictRepo(db)
		return analyticsSvc.NewAnalyticsService(chRepo, metricRepo), nil
	})

	do.Provide(injector, func(i *do.Injector) (analyticsCtrl.AnalyticsController, error) {
		svc := do.MustInvoke[analyticsSvc.AnalyticsService](i)
		return analyticsCtrl.NewAnalyticsController(svc), nil
	})
}
