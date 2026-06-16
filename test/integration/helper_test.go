//go:build integration

package integration

import (
	"context"
	"os"
	"testing"

	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/cloudwego/hertz/pkg/route"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/joeyyang/internal-transfers/internal/api"
	"github.com/joeyyang/internal-transfers/internal/observability"
	"github.com/joeyyang/internal-transfers/internal/platform/postgres"
	"github.com/joeyyang/internal-transfers/internal/repository"
	"github.com/joeyyang/internal-transfers/internal/service"
)

// env holds the wired services, pool, and HTTP engine for an integration test.
// env 持有集成测试装配好的 service、连接池和 HTTP 引擎。
type env struct {
	pool        *pgxpool.Pool
	accountSvc  *service.AccountService
	transferSvc *service.TransferService
	engine      *route.Engine // public router, for HTTP-level tests — 公开路由，用于 HTTP 层测试
}

// setup connects to TEST_DATABASE_URL, migrates, truncates, and wires services.
// The test is skipped when TEST_DATABASE_URL is not set.
// setup 连接 TEST_DATABASE_URL，执行迁移、清空表并装配 service。
// 未设置 TEST_DATABASE_URL 时跳过测试。
func setup(t *testing.T) *env {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set; skipping integration test")
	}

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

	// Build the real public router so HTTP-level tests exercise handlers,
	// DTO binding, middleware and error mapping end to end.
	// 装配真实的公开路由，让 HTTP 层测试端到端覆盖 handler、DTO 绑定、中间件与错误映射。
	metrics := observability.NewMetrics(prometheus.NewRegistry())
	logger := observability.NewLogger("error")
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
	}
}
