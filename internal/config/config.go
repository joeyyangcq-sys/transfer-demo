package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

// Config holds runtime settings loaded from environment variables.
// Config 保存从环境变量加载的运行时配置。
type Config struct {
	Addr            string        // public HTTP listen address, e.g. ":8080" — 公开 HTTP 监听地址
	MetricsAddr     string        // internal admin/metrics address, e.g. ":9090" — 内部 admin/指标监听地址
	DatabaseURL     string        // PostgreSQL DSN — PostgreSQL 连接串
	DBMaxConns      int32         // pgx pool max connections — pgx 连接池最大连接数
	ShutdownTimeout time.Duration // graceful shutdown budget — 优雅关闭的超时预算
	RunMigrations   bool          // run migrations on startup — 启动时是否执行迁移
}

// Load reads configuration from the environment, applying defaults.
// Load 从环境变量读取配置，并套用默认值。
func Load() (Config, error) {
	cfg := Config{
		Addr:            getEnv("APP_ADDR", ":8080"),
		MetricsAddr:     getEnv("METRICS_ADDR", ":9090"),
		DatabaseURL:     os.Getenv("DATABASE_URL"),
		DBMaxConns:      int32(getEnvInt("DB_MAX_CONNS", 10)),
		ShutdownTimeout: time.Duration(getEnvInt("SHUTDOWN_TIMEOUT_SECONDS", 15)) * time.Second,
		RunMigrations:   getEnvBool("RUN_MIGRATIONS", true),
	}

	if cfg.DatabaseURL == "" {
		return Config{}, fmt.Errorf("DATABASE_URL is required")
	}
	if cfg.DBMaxConns <= 0 {
		return Config{}, fmt.Errorf("DB_MAX_CONNS must be positive")
	}
	return cfg, nil
}

// getEnv returns the env value for key, or fallback when unset/empty.
// getEnv 返回 key 对应的环境变量值，未设置或为空时返回 fallback。
func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// getEnvInt parses an int env value, falling back on missing/invalid input.
// getEnvInt 解析整型环境变量，缺失或非法时返回 fallback。
func getEnvInt(key string, fallback int) int {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return n
}

// getEnvBool parses a bool env value, falling back on missing/invalid input.
// getEnvBool 解析布尔型环境变量，缺失或非法时返回 fallback。
func getEnvBool(key string, fallback bool) bool {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return fallback
	}
	return b
}
