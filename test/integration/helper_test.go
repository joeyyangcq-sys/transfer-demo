//go:build integration

package integration

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/joeyyang/internal-transfers/internal/platform/postgres"
	"github.com/joeyyang/internal-transfers/internal/repository"
	"github.com/joeyyang/internal-transfers/internal/service"
)

// env holds the wired services and pool for an integration test.
type env struct {
	pool        *pgxpool.Pool
	accountSvc  *service.AccountService
	transferSvc *service.TransferService
}

// setup connects to TEST_DATABASE_URL, migrates, truncates, and wires services.
// The test is skipped when TEST_DATABASE_URL is not set.
func setup(t *testing.T) *env {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set; skipping integration test")
	}

	ctx := context.Background()
	pool, err := postgres.NewPool(ctx, dsn, 20)
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

	return &env{
		pool:        pool,
		accountSvc:  service.NewAccountService(pool, accountRepo),
		transferSvc: service.NewTransferService(pool, txm, accountRepo, transferRepo),
	}
}
