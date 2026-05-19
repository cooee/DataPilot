package main

import (
	"context"
	"log"

	gsheetRepo "github.com/Caknoooo/go-gin-clean-starter/modules/gsheet/repository"
	"github.com/Caknoooo/go-gin-clean-starter/pkg/constants"
	"github.com/samber/do"
)

const chMigrationsDir = "database/clickhouse/migrations"

func runCHMigrate(injector *do.Injector) {
	repo := do.MustInvokeNamed[*gsheetRepo.EtlRepo](injector, constants.EtlRepo)
	ctx := context.Background()
	if err := repo.RunMigrations(ctx, chMigrationsDir); err != nil {
		log.Fatalf("[ch-migrate] failed: %v", err)
	}
	log.Println("[ch-migrate] all migrations applied successfully")
}
