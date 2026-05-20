package constants

const (
	ENUM_ROLE_ADMIN = "admin"
	ENUM_ROLE_USER  = "user"

	ENUM_RUN_PRODUCTION = "production"
	ENUM_RUN_TESTING    = "testing"

	ENUM_PAGINATION_PER_PAGE = 10
	ENUM_PAGINATION_PAGE     = 1

	DB           = "db"
	ClickHouseDB = "clickhouseDB"
	JWTService   = "JWTService"

	GoogleDocsService   = "googleDocsService"
	GoogleSheetsService = "googleSheetsService"
	GoogleDriveService  = "googleDriveService"

	// GSheet pipeline DI keys
	ODSRepo    = "odsRepo"
	EtlRepo    = "etlRepo"
	CSVFetcher = "csvFetcher"
	ODSLoader  = "odsLoader"
	ETLRunner  = "etlRunner"
	DQChecker  = "dqChecker"

	// Site product pipeline (gid=553168897)
	SiteODSRepo   = "siteODSRepo"
	SiteODSLoader = "siteODSLoader"
	SiteDQChecker = "siteDQChecker"
)
