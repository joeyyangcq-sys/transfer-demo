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
type Querier interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// maxTxRetries bounds automatic retries on transient serialization failures.
const maxTxRetries = 3

// TxManager runs functions inside a database transaction.
type TxManager struct {
	pool *pgxpool.Pool
}

// NewTxManager creates a TxManager backed by the given pool.
func NewTxManager(pool *pgxpool.Pool) *TxManager {
	return &TxManager{pool: pool}
}

// WithTx runs fn inside a transaction, committing on success and rolling back
// on error. It retries on deadlock (40P01) and serialization (40001) failures.
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

func (m *TxManager) runOnce(ctx context.Context, fn func(q Querier) error) error {
	tx, err := m.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin: %w", err)
	}
	if err := fn(tx); err != nil {
		_ = tx.Rollback(ctx)
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		_ = tx.Rollback(ctx)
		return err
	}
	return nil
}

// isRetryable reports whether the error is a transient transaction conflict.
func isRetryable(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == "40001" || pgErr.Code == "40P01"
	}
	return false
}
