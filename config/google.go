package config

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"

	"github.com/joho/godotenv"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/docs/v1"
	"google.golang.org/api/drive/v3"
	"google.golang.org/api/option"
	"google.golang.org/api/sheets/v4"
)

// 三个 Service 共享同一个 OAuth2 HTTP Client，避免重复读凭证 / 申请 token。
var (
	googleHTTPClientOnce sync.Once
	googleHTTPClient     *http.Client
	googleHTTPClientErr  error
)

func googleScopes() []string {
	readonly := strings.ToLower(strings.TrimSpace(os.Getenv("GOOGLE_READONLY"))) != "false"
	if readonly {
		return []string{
			docs.DocumentsReadonlyScope,
			sheets.SpreadsheetsReadonlyScope,
			drive.DriveReadonlyScope,
		}
	}
	return []string{
		docs.DocumentsScope,
		sheets.SpreadsheetsScope,
		drive.DriveScope,
	}
}

func loadGoogleCredentialsJSON() ([]byte, error) {
	if inline := strings.TrimSpace(os.Getenv("GOOGLE_CREDENTIALS_JSON")); inline != "" {
		return []byte(inline), nil
	}

	path := strings.TrimSpace(os.Getenv("GOOGLE_APPLICATION_CREDENTIALS"))
	if path == "" {
		return nil, fmt.Errorf("neither GOOGLE_APPLICATION_CREDENTIALS nor GOOGLE_CREDENTIALS_JSON is set")
	}
	return os.ReadFile(path)
}

func buildGoogleHTTPClient(ctx context.Context) (*http.Client, error) {
	// 跟 SetUpDatabaseConnection / SetUpClickHouseConnection 行为一致：
	// 这里也兜底 load 一次 .env，避免命令行子命令未先初始化 DB 时读不到环境变量
	_ = godotenv.Load(".env")

	scopes := googleScopes()

	jsonBytes, err := loadGoogleCredentialsJSON()
	if err != nil {
		return nil, err
	}

	// 优先按 service account JWT 解析；兼容用户授权类的 ADC JSON
	if jwtCfg, jwtErr := google.JWTConfigFromJSON(jsonBytes, scopes...); jwtErr == nil {
		if subject := strings.TrimSpace(os.Getenv("GOOGLE_IMPERSONATE_SUBJECT")); subject != "" {
			jwtCfg.Subject = subject
		}
		return jwtCfg.Client(ctx), nil
	}

	creds, err := google.CredentialsFromJSON(ctx, jsonBytes, scopes...)
	if err != nil {
		return nil, fmt.Errorf("invalid google credentials: %w", err)
	}
	return oauth2.NewClient(ctx, creds.TokenSource), nil
}

func getGoogleHTTPClient(ctx context.Context) (*http.Client, error) {
	googleHTTPClientOnce.Do(func() {
		googleHTTPClient, googleHTTPClientErr = buildGoogleHTTPClient(ctx)
	})
	return googleHTTPClient, googleHTTPClientErr
}

func SetUpGoogleDocsService() *docs.Service {
	ctx := context.Background()
	httpClient, err := getGoogleHTTPClient(ctx)
	if err != nil {
		panic(fmt.Errorf("failed to build google http client: %w", err))
	}
	svc, err := docs.NewService(ctx, option.WithHTTPClient(httpClient))
	if err != nil {
		panic(fmt.Errorf("failed to init google docs service: %w", err))
	}
	return svc
}

func SetUpGoogleSheetsService() *sheets.Service {
	ctx := context.Background()
	httpClient, err := getGoogleHTTPClient(ctx)
	if err != nil {
		panic(fmt.Errorf("failed to build google http client: %w", err))
	}
	svc, err := sheets.NewService(ctx, option.WithHTTPClient(httpClient))
	if err != nil {
		panic(fmt.Errorf("failed to init google sheets service: %w", err))
	}
	return svc
}

func SetUpGoogleDriveService() *drive.Service {
	ctx := context.Background()
	httpClient, err := getGoogleHTTPClient(ctx)
	if err != nil {
		panic(fmt.Errorf("failed to build google http client: %w", err))
	}
	svc, err := drive.NewService(ctx, option.WithHTTPClient(httpClient))
	if err != nil {
		panic(fmt.Errorf("failed to init google drive service: %w", err))
	}
	return svc
}
