package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Querier is the subset of pgx used by repositories. Both *pgxpool.Pool and
// pgx.Tx satisfy it, so repo methods work inside or outside a transaction.
// Querier 是 repository 用到的 pgx 子集。*pgxpool.Pool 和 pgx.Tx 都满足它，
// 因此 repo 方法在事务内外都能使用。
type Querier interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// maxTxRetries bounds automatic retries on transient serialization failures.
// maxTxRetries 限制对瞬时序列化冲突的自动重试次数。
const maxTxRetries = 3

// TxManager runs functions inside a database transaction.
// TxManager 负责在数据库事务中运行函数。
type TxManager struct {
	pool *pgxpool.Pool
}

// NewTxManager creates a TxManager backed by the given pool.
// NewTxManager 基于给定连接池创建 TxManager。
func NewTxManager(pool *pgxpool.Pool) *TxManager {
	return &TxManager{pool: pool}
}

// WithTx runs fn inside a transaction, committing on success and rolling back
// on error. It retries on deadlock (40P01) and serialization (40001) failures.
// WithTx 在事务中运行 fn，成功则提交、出错则回滚。
// 遇到死锁(40P01)或序列化失败(40001)时会自动重试。
func (m *TxManager) WithTx(ctx context.Context, fn func(q Querier) error) error {
	var lastErr error
	for attempt := 0; attempt < maxTxRetries; attempt++ {
		err := m.runOnce(ctx, fn)
		if err == nil {
			return nil
		}
		if !isRetryable(err) {
			return err
		}
		lastErr = err
	}
	return fmt.Errorf("transaction failed after %d attempts: %w", maxTxRetries, lastErr)
}

// runOnce executes fn once inside a single transaction.
// runOnce 在单个事务里执行一次 fn。
func (m *TxManager) runOnce(ctx context.Context, fn func(q Querier) error) error {
	tx, err := m.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin: %w", err)
	}
	// Roll back on any early return or panic; a no-op after a successful commit.
	// 任何提前返回或 panic 都会回滚；成功提交后这次回滚是空操作。
	defer func() { _ = tx.Rollback(ctx) }()

	if err := fn(tx); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit: %w", err)
	}
	return nil
}

// isRetryable reports whether the error is a transient transaction conflict.
// isRetryable 判断该错误是否为可重试的瞬时事务冲突。
func isRetryable(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == "40001" || pgErr.Code == "40P01"
	}
	return false
}
