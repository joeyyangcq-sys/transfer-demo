package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoad_ConfigFileOnly(t *testing.T) {
	// Create a temporary YAML config file
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "test_config.yaml")
	yamlContent := []byte(`
addr: ":8088"
metrics_addr: ":9099"
database:
  host: "test-db-host"
  port: 5433
  user: "testuser"
  password: "testpassword"
  dbname: "testdb"
  sslmode: "require"
  max_conns: 25
shutdown_timeout: 30
run_migrations: false
log_level: "debug"
log_file: "/tmp/test.log"
`)
	if err := os.WriteFile(configPath, yamlContent, 0644); err != nil {
		t.Fatalf("failed to write temp config file: %v", err)
	}

	// Ensure NO environment variables are influencing the test
	envVarsToClear := []string{
		"APP_ADDR", "METRICS_ADDR", "DB_HOST", "DB_PORT", "DB_USER",
		"DB_PASSWORD", "DB_NAME", "DB_SSLMODE", "DB_MAX_CONNS",
		"SHUTDOWN_TIMEOUT_SECONDS", "RUN_MIGRATIONS", "LOG_LEVEL", "LOG_FILE",
	}
	for _, env := range envVarsToClear {
		t.Setenv(env, "")
	}

	// Set ONLY the CONFIG_FILE environment variable
	t.Setenv("CONFIG_FILE", configPath)

	// Load configuration
	cfg, err := Load()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	// Verify that the config was loaded entirely from the file
	if cfg.Addr != ":8088" {
		t.Errorf("expected Addr ':8088', got %q", cfg.Addr)
	}
	if cfg.MetricsAddr != ":9099" {
		t.Errorf("expected MetricsAddr ':9099', got %q", cfg.MetricsAddr)
	}
	if cfg.Database.Host != "test-db-host" {
		t.Errorf("expected DB Host 'test-db-host', got %q", cfg.Database.Host)
	}
	if cfg.Database.Port != 5433 {
		t.Errorf("expected DB Port 5433, got %d", cfg.Database.Port)
	}
	if cfg.Database.User != "testuser" {
		t.Errorf("expected DB User 'testuser', got %q", cfg.Database.User)
	}
	if cfg.Database.DBName != "testdb" {
		t.Errorf("expected DB Name 'testdb', got %q", cfg.Database.DBName)
	}
	if cfg.Database.SSLMode != "require" {
		t.Errorf("expected SSLMode 'require', got %q", cfg.Database.SSLMode)
	}
	if cfg.Database.MaxConns != 25 {
		t.Errorf("expected MaxConns 25, got %d", cfg.Database.MaxConns)
	}
	if cfg.ShutdownTimeout != 30*time.Second {
		t.Errorf("expected ShutdownTimeout 30s, got %v", cfg.ShutdownTimeout)
	}
	if cfg.RunMigrations != false {
		t.Errorf("expected RunMigrations false, got %v", cfg.RunMigrations)
	}
	if cfg.LogLevel != "debug" {
		t.Errorf("expected LogLevel 'debug', got %q", cfg.LogLevel)
	}
	if cfg.LogFile != "/tmp/test.log" {
		t.Errorf("expected LogFile '/tmp/test.log', got %q", cfg.LogFile)
	}
}

func TestLoad_EnvOverridesConfigFile(t *testing.T) {
	// Create a temporary YAML config file
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "test_config_override.yaml")
	yamlContent := []byte(`
addr: ":8088"
database:
  host: "file-host"
`)
	if err := os.WriteFile(configPath, yamlContent, 0644); err != nil {
		t.Fatalf("failed to write temp config file: %v", err)
	}

	t.Setenv("CONFIG_FILE", configPath)
	
	// Override via environment variables
	t.Setenv("APP_ADDR", ":9999")
	t.Setenv("DB_HOST", "env-host")
	t.Setenv("DB_NAME", "env-db") // Satisfy required field

	cfg, err := Load()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	// Verify environment variables took precedence
	if cfg.Addr != ":9999" {
		t.Errorf("expected Addr ':9999' (from env), got %q", cfg.Addr)
	}
	if cfg.Database.Host != "env-host" {
		t.Errorf("expected DB Host 'env-host' (from env), got %q", cfg.Database.Host)
	}
}
