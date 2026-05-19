package providers

import (
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/Caknoooo/go-gin-clean-starter/config"
	authController "github.com/Caknoooo/go-gin-clean-starter/modules/auth/controller"
	authRepo "github.com/Caknoooo/go-gin-clean-starter/modules/auth/repository"
	authService "github.com/Caknoooo/go-gin-clean-starter/modules/auth/service"
	userController "github.com/Caknoooo/go-gin-clean-starter/modules/user/controller"
	"github.com/Caknoooo/go-gin-clean-starter/modules/user/repository"
	userService "github.com/Caknoooo/go-gin-clean-starter/modules/user/service"
	"github.com/Caknoooo/go-gin-clean-starter/pkg/constants"
	"github.com/samber/do"
	"google.golang.org/api/docs/v1"
	"google.golang.org/api/drive/v3"
	"google.golang.org/api/sheets/v4"
	"gorm.io/gorm"
)

func InitDatabase(injector *do.Injector) {
	do.ProvideNamed(injector, constants.DB, func(i *do.Injector) (*gorm.DB, error) {
		return config.SetUpDatabaseConnection(), nil
	})

	do.ProvideNamed(injector, constants.ClickHouseDB, func(i *do.Injector) (driver.Conn, error) {
		return config.SetUpClickHouseConnection(), nil
	})
}

func InitGoogleServices(injector *do.Injector) {
	do.ProvideNamed(injector, constants.GoogleDocsService, func(i *do.Injector) (*docs.Service, error) {
		return config.SetUpGoogleDocsService(), nil
	})

	do.ProvideNamed(injector, constants.GoogleSheetsService, func(i *do.Injector) (*sheets.Service, error) {
		return config.SetUpGoogleSheetsService(), nil
	})

	do.ProvideNamed(injector, constants.GoogleDriveService, func(i *do.Injector) (*drive.Service, error) {
		return config.SetUpGoogleDriveService(), nil
	})
}

func RegisterDependencies(injector *do.Injector) {
	InitDatabase(injector)
	InitGoogleServices(injector)

	do.ProvideNamed(injector, constants.JWTService, func(i *do.Injector) (authService.JWTService, error) {
		return authService.NewJWTService(), nil
	})

	db := do.MustInvokeNamed[*gorm.DB](injector, constants.DB)
	jwtService := do.MustInvokeNamed[authService.JWTService](injector, constants.JWTService)

	userRepository := repository.NewUserRepository(db)
	refreshTokenRepository := authRepo.NewRefreshTokenRepository(db)

	userService := userService.NewUserService(userRepository, db)
	authService := authService.NewAuthService(userRepository, refreshTokenRepository, jwtService, db)

	do.Provide(
		injector, func(i *do.Injector) (userController.UserController, error) {
			return userController.NewUserController(i, userService), nil
		},
	)

	do.Provide(
		injector, func(i *do.Injector) (authController.AuthController, error) {
			return authController.NewAuthController(i, authService), nil
		},
	)
}
