package config

import (
	"fmt"
	"os"
	"time"

	"github.com/spf13/viper"
)

// DBConfig holds structured PostgreSQL configuration.
// DBConfig 保存结构化的 PostgreSQL 配置。
type DBConfig struct {
	Host     string `mapstructure:"host"`      // e.g. "localhost" or "postgres"
	Port     int    `mapstructure:"port"`      // e.g. 5432
	User     string `mapstructure:"user"`      // database user
	Password string `mapstructure:"password"`  // database password
	DBName   string `mapstructure:"dbname"`    // database name
	SSLMode  string `mapstructure:"sslmode"`   // e.g. "disable", "require"
	MaxConns int32  `mapstructure:"max_conns"` // pgx pool max connections
}

// DSN constructs the PostgreSQL connection string from structured fields.
// DSN 从结构化字段构造 PostgreSQL 连接字符串。
func (d DBConfig) DSN() string {
	return fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=%s",
		d.User, d.Password, d.Host, d.Port, d.DBName, d.SSLMode)
}

// Config holds runtime settings loaded from configuration file or environment variables.
// Config 保存从配置文件或环境变量加载的运行时配置。
type Config struct {
	Addr            string        `mapstructure:"addr"`           // public HTTP listen address, e.g. ":8080"
	MetricsAddr     string        `mapstructure:"metrics_addr"`   // internal admin/metrics address, e.g. ":9090"
	Database        DBConfig      `mapstructure:"database"`       // structured PostgreSQL configuration
	ShutdownTimeout time.Duration `mapstructure:"-"`              // graceful shutdown budget (mapped manually from int)
	RunMigrations   bool          `mapstructure:"run_migrations"` // run migrations on startup
	LogLevel        string        `mapstructure:"log_level"`      // log level: debug, info, warn, error
	LogFile         string        `mapstructure:"log_file"`       // optional log file path
}

// Load reads configuration from a file (if CONFIG_FILE is set) and then from the environment, applying defaults.
// Load 先从配置文件读取（如果指定了 CONFIG_FILE），然后被环境变量覆盖，并套用默认值。
func Load() (Config, error) {
	v := viper.New()

	// 设置默认值 (Defaults)
	v.SetDefault("addr", ":8080")
	v.SetDefault("metrics_addr", ":9090")
	v.SetDefault("database.host", "localhost")
	v.SetDefault("database.port", 5432)
	v.SetDefault("database.user", "postgres")
	v.SetDefault("database.password", "")
	v.SetDefault("database.dbname", "transfers")
	v.SetDefault("database.sslmode", "disable")
	v.SetDefault("database.max_conns", 10)
	v.SetDefault("shutdown_timeout_seconds", 15)
	v.SetDefault("run_migrations", true)
	v.SetDefault("log_level", "info")
	v.SetDefault("log_file", "")

	// 绑定环境变量，确保向下兼容 (Bind Environment Variables)
	_ = v.BindEnv("addr", "APP_ADDR")
	_ = v.BindEnv("metrics_addr", "METRICS_ADDR")
	_ = v.BindEnv("database.host", "DB_HOST")
	_ = v.BindEnv("database.port", "DB_PORT")
	_ = v.BindEnv("database.user", "DB_USER")
	_ = v.BindEnv("database.password", "DB_PASSWORD")
	_ = v.BindEnv("database.dbname", "DB_NAME")
	_ = v.BindEnv("database.sslmode", "DB_SSLMODE")
	_ = v.BindEnv("database.max_conns", "DB_MAX_CONNS")
	_ = v.BindEnv("shutdown_timeout_seconds", "SHUTDOWN_TIMEOUT_SECONDS")
	_ = v.BindEnv("run_migrations", "RUN_MIGRATIONS")
	_ = v.BindEnv("log_level", "LOG_LEVEL")
	_ = v.BindEnv("log_file", "LOG_FILE")

	// 加载配置文件 (Load config file if specified)
	if configFile := os.Getenv("CONFIG_FILE"); configFile != "" {
		v.SetConfigFile(configFile)
		if err := v.ReadInConfig(); err != nil {
			return Config{}, fmt.Errorf("failed to read config file: %w", err)
		}
	} else {
		// 让 Viper 也能自动读取同名环境变量
		v.AutomaticEnv()
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return Config{}, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	// 特殊处理：将配置中的 int 转换为 time.Duration
	timeoutSec := v.GetInt("shutdown_timeout_seconds")
	if timeoutSec == 15 && v.GetInt("shutdown_timeout") != 0 {
		timeoutSec = v.GetInt("shutdown_timeout") // 兼容 YAML 里的名称
	}
	cfg.ShutdownTimeout = time.Duration(timeoutSec) * time.Second

	// 必填项校验
	if cfg.Database.Host == "" || cfg.Database.DBName == "" {
		return Config{}, fmt.Errorf("database host and dbname are required")
	}
	if cfg.Database.MaxConns <= 0 {
		return Config{}, fmt.Errorf("database max_conns must be positive")
	}
	return cfg, nil
}
