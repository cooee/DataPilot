package config

import (
	"os"

	"github.com/joho/godotenv"
)

// LoadEnv 加载项目根目录 .env（CLI / HTTP 入口统一调用）。
func LoadEnv() {
	_ = godotenv.Load(".env")
}

// MustGetenv 读取环境变量，空则 panic（仅用于启动期强依赖项）。
func MustGetenv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		panic("missing env: " + key)
	}
	return v
}
