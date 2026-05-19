package migrations

import (
	"github.com/Caknoooo/go-gin-clean-starter/database"
	"github.com/Caknoooo/go-gin-clean-starter/database/entities"
	"gorm.io/gorm"
)

func init() {
	database.RegisterMigration(
		"20260519000000_create_meta_metric_dict",
		Up20260519000000CreateMetaMetricDict,
		Down20260519000000CreateMetaMetricDict,
	)
}

func Up20260519000000CreateMetaMetricDict(db *gorm.DB) error {
	return db.AutoMigrate(&entities.MetaMetricDict{})
}

func Down20260519000000CreateMetaMetricDict(db *gorm.DB) error {
	return db.Migrator().DropTable(&entities.MetaMetricDict{})
}
