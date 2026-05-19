package config

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/joho/godotenv"
)

func SetUpClickHouseConnection() driver.Conn {
	if err := godotenv.Load(".env"); err != nil {
		// 复用 SetUpDatabaseConnection 的行为：找不到 .env 时直接 panic
		// 如果项目里希望 .env 可选，可改为仅在 os.IsNotExist 时忽略
		panic(err)
	}

	host := os.Getenv("CLICKHOUSE_HOST")
	user := os.Getenv("CLICKHOUSE_USER")
	password := os.Getenv("CLICKHOUSE_PASS")
	database := os.Getenv("CLICKHOUSE_DB")

	port, err := strconv.Atoi(os.Getenv("CLICKHOUSE_PORT"))
	if err != nil || port == 0 {
		port = 9000
	}

	conn, err := clickhouse.Open(&clickhouse.Options{
		Addr: []string{fmt.Sprintf("%s:%d", host, port)},
		Auth: clickhouse.Auth{
			Database: database,
			Username: user,
			Password: password,
		},
		Compression: &clickhouse.Compression{
			Method: clickhouse.CompressionLZ4,
		},
		DialTimeout:     30 * time.Second,
		MaxOpenConns:    10,
		MaxIdleConns:    5,
		ConnMaxLifetime: time.Hour,
		ClientInfo: clickhouse.ClientInfo{
			Products: []struct {
				Name    string
				Version string
			}{
				{Name: os.Getenv("APP_NAME"), Version: "1.0.0"},
			},
		},
	})
	if err != nil {
		panic(fmt.Errorf("failed to open clickhouse connection: %w", err))
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := conn.Ping(ctx); err != nil {
		panic(fmt.Errorf("failed to ping clickhouse: %w", err))
	}

	return conn
}

func CloseClickHouseConnection(conn driver.Conn) {
	if conn == nil {
		return
	}
	_ = conn.Close()
}
