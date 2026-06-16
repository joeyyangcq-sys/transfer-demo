package postgres

import (
	"context"
	"fmt"
	"time"

	pgxdecimal "github.com/jackc/pgx-shopspring-decimal"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// NewPool creates a pgx connection pool and verifies connectivity. The optional
// tracer (may be nil) records per-query latency.
// NewPool 创建 pgx 连接池并校验连通性。可选的 tracer（可为 nil）记录单条查询耗时。
func NewPool(ctx context.Context, dsn string, maxConns int32, tracer pgx.QueryTracer) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("parse dsn: %w", err)
	}
	cfg.MaxConns = maxConns
	// Recycle connections so stale ones (e.g. after an RDS failover) are dropped.
	// 回收连接，淘汰陈旧连接（例如 RDS 故障切换后的失效连接）。
	cfg.MaxConnLifetime = time.Hour
	cfg.MaxConnIdleTime = 30 * time.Minute
	if tracer != nil {
		cfg.ConnConfig.Tracer = tracer
	}

	// Teach pgx to map NUMERIC <-> shopspring/decimal.Decimal.
	// 让 pgx 在 NUMERIC 与 shopspring/decimal.Decimal 之间互相映射。
	cfg.AfterConnect = func(ctx context.Context, conn *pgx.Conn) error {
		pgxdecimal.Register(conn.TypeMap())
		return nil
	}

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("create pool: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping: %w", err)
	}
	return pool, nil
}
