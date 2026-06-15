package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

// Config holds runtime settings loaded from environment variables.
type Config struct {
	Addr            string        // HTTP listen address, e.g. ":8080"
	DatabaseURL     string        // PostgreSQL DSN
	DBMaxConns      int32         // pgx pool max connections
	ShutdownTimeout time.Duration // graceful shutdown budget
	RunMigrations   bool          // run migrations on startup
}

// Load reads configuration from the environment, applying defaults.
func Load() (Config, error) {
	cfg := Config{
		Addr:            getEnv("APP_ADDR", ":8080"),
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

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

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
