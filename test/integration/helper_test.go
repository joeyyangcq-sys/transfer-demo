//go:build integration

package integration

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"os"
	"testing"

	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/cloudwego/hertz/pkg/route"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/joeyyang/transfer-demo/internal/api"
	"github.com/joeyyang/transfer-demo/internal/observability"
	"github.com/joeyyang/transfer-demo/internal/platform/postgres"
	"github.com/joeyyang/transfer-demo/internal/repository"
	"github.com/joeyyang/transfer-demo/internal/service"
)

// env holds the wired services, pool, and HTTP engine for an integration test.
// env 持有集成测试装配好的 service、连接池和 HTTP 引擎。
type env struct {
	pool        *pgxpool.Pool
	accountSvc  *service.AccountService
	transferSvc *service.TransferService
	engine      *route.Engine // public router, for HTTP-level tests — 公开路由，用于 HTTP 层测试
	logs        *bytes.Buffer // captured JSON logs (access + error + panic) — 捕获的 JSON 日志
}

// setup connects to TEST_DB_*, migrates, truncates, and wires services.
// The test is skipped when TEST_DB_HOST is not set.
// setup 连接 TEST_DB_*，执行迁移、清空表并装配 service。
// 未设置 TEST_DB_HOST 时跳过测试。
func setup(t *testing.T) *env {
	t.Helper()

	host := os.Getenv("TEST_DB_HOST")
	if host == "" {
		t.Skip("TEST_DB_HOST not set; skipping integration test")
	}

	port := os.Getenv("TEST_DB_PORT")
	if port == "" {
		port = "5432"
	}
	user := os.Getenv("TEST_DB_USER")
	if user == "" {
		user = "postgres"
	}
	password := os.Getenv("TEST_DB_PASSWORD")
	dbname := os.Getenv("TEST_DB_NAME")
	if dbname == "" {
		dbname = "transfers_test"
	}
	sslmode := os.Getenv("TEST_DB_SSLMODE")
	if sslmode == "" {
		sslmode = "disable"
	}

	dsn := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=%s", user, password, host, port, dbname, sslmode)

	ctx := context.Background()
	pool, err := postgres.NewPool(ctx, dsn, 20, nil)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)

	if err := postgres.Migrate(ctx, pool); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if _, err := pool.Exec(ctx, `TRUNCATE ledger_entries, transfers, accounts RESTART IDENTITY CASCADE`); err != nil {
		t.Fatalf("truncate: %v", err)
	}

	accountRepo := repository.NewAccountRepository()
	transferRepo := repository.NewTransferRepository()
	txm := repository.NewTxManager(pool)

	accountSvc := service.NewAccountService(pool, accountRepo)
	transferSvc := service.NewTransferService(pool, txm, accountRepo, transferRepo)

	// Build the real public router so HTTP-level tests drive handlers,
	// DTO binding, middleware and error mapping end to end. The logger writes
	// to a buffer so tests can inspect the emitted logs.
	// 装配真实的公开路由，让 HTTP 层测试端到端覆盖 handler、DTO 绑定、中间件与错误映射。
	// logger 写入缓冲区，便于测试检查产生的日志。
	logs := &bytes.Buffer{}
	logger := slog.New(slog.NewJSONHandler(logs, &slog.HandlerOptions{Level: slog.LevelDebug}))
	metrics := observability.NewMetrics(prometheus.NewRegistry())
	resp := api.NewResponder(metrics, logger)
	handlers := api.Handlers{
		Account:     api.NewAccountHandler(accountSvc, resp, metrics),
		Transaction: api.NewTransactionHandler(transferSvc, resp, metrics),
		Health:      api.NewHealthHandler(pool),
	}
	h := server.New()
	api.RegisterPublicRoutes(h, metrics, logger, handlers)

	return &env{
		pool:        pool,
		accountSvc:  accountSvc,
		transferSvc: transferSvc,
		engine:      h.Engine,
		logs:        logs,
	}
}
