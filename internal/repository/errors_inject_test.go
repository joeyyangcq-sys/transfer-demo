package repository

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/shopspring/decimal"

	"github.com/joeyyang/transfer-demo/internal/domain"
)

// fakeRow returns a fixed error from Scan.
// fakeRow 让 Scan 返回固定错误。
type fakeRow struct{ err error }

func (r fakeRow) Scan(_ ...any) error { return r.err }

// fakeRows is a minimal pgx.Rows that yields rowCount rows and can inject a
// Scan or iteration error.
// fakeRows 是最小化的 pgx.Rows，产出 rowCount 行，并可注入 Scan 或迭代错误。
type fakeRows struct {
	rowCount int
	i        int
	scanErr  error
	iterErr  error
}

func (r *fakeRows) Next() bool                                   { r.i++; return r.i <= r.rowCount }
func (r *fakeRows) Scan(_ ...any) error                          { return r.scanErr }
func (r *fakeRows) Err() error                                   { return r.iterErr }
func (r *fakeRows) Close()                                       {}
func (r *fakeRows) CommandTag() pgconn.CommandTag                { return pgconn.CommandTag{} }
func (r *fakeRows) FieldDescriptions() []pgconn.FieldDescription { return nil }
func (r *fakeRows) Values() ([]any, error)                       { return nil, nil }
func (r *fakeRows) RawValues() [][]byte                          { return nil }
func (r *fakeRows) Conn() *pgx.Conn                              { return nil }

// fakeQuerier injects errors so repository error mapping can be tested without
// a database (e.g. a 23514 check_violation that real constraints prevent).
// fakeQuerier 注入错误，让 repository 的错误映射无需数据库即可测试
// （例如真实约束根本造不出的 23514 check_violation）。
type fakeQuerier struct {
	execErr  error
	scanErr  error
	rows     pgx.Rows
	queryErr error
}

func (q fakeQuerier) Exec(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, q.execErr
}
func (q fakeQuerier) Query(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
	return q.rows, q.queryErr
}
func (q fakeQuerier) QueryRow(_ context.Context, _ string, _ ...any) pgx.Row {
	return fakeRow{err: q.scanErr}
}

func pgErr(code, constraint string) error {
	return &pgconn.PgError{Code: code, ConstraintName: constraint}
}

func TestAccountRepo_ErrorMapping(t *testing.T) {
	ctx := context.Background()
	r := NewAccountRepository()

	if err := r.Create(ctx, fakeQuerier{execErr: pgErr("23505", "accounts_pkey")}, 1, decimal.Zero); !errors.Is(err, domain.ErrAccountAlreadyExists) {
		t.Errorf("Create dup: got %v, want ErrAccountAlreadyExists", err)
	}
	if err := r.Create(ctx, fakeQuerier{execErr: errors.New("boom")}, 1, decimal.Zero); err == nil || errors.Is(err, domain.ErrAccountAlreadyExists) {
		t.Errorf("Create generic: got %v, want wrapped error", err)
	}
	if _, err := r.Get(ctx, fakeQuerier{scanErr: pgx.ErrNoRows}, 1); !errors.Is(err, domain.ErrAccountNotFound) {
		t.Errorf("Get no rows: got %v, want ErrAccountNotFound", err)
	}
	if _, err := r.Get(ctx, fakeQuerier{scanErr: errors.New("boom")}, 1); err == nil || errors.Is(err, domain.ErrAccountNotFound) {
		t.Errorf("Get generic: got %v, want wrapped error", err)
	}
	// 23514: the balance_non_negative safety net (real constraints prevent it).
	// 23514：balance_non_negative 安全网（真实约束根本不会让它发生）。
	if err := r.UpdateBalance(ctx, fakeQuerier{execErr: pgErr("23514", "balance_non_negative")}, 1, decimal.Zero); !errors.Is(err, domain.ErrInsufficientFunds) {
		t.Errorf("UpdateBalance check_violation: got %v, want ErrInsufficientFunds", err)
	}
	if err := r.UpdateBalance(ctx, fakeQuerier{execErr: errors.New("boom")}, 1, decimal.Zero); err == nil || errors.Is(err, domain.ErrInsufficientFunds) {
		t.Errorf("UpdateBalance generic: got %v, want wrapped error", err)
	}

	// LockForUpdate error paths: query error, scan error, iteration error.
	// LockForUpdate 的错误路径：查询错误、扫描错误、迭代错误。
	if _, err := r.LockForUpdate(ctx, fakeQuerier{queryErr: errors.New("boom")}, []int64{1}); err == nil {
		t.Errorf("LockForUpdate query error: want wrapped error")
	}
	if _, err := r.LockForUpdate(ctx, fakeQuerier{rows: &fakeRows{rowCount: 1, scanErr: errors.New("boom")}}, []int64{1}); err == nil {
		t.Errorf("LockForUpdate scan error: want wrapped error")
	}
	if _, err := r.LockForUpdate(ctx, fakeQuerier{rows: &fakeRows{iterErr: errors.New("boom")}}, []int64{1}); err == nil {
		t.Errorf("LockForUpdate iterate error: want wrapped error")
	}
}

func TestIsRetryable(t *testing.T) {
	cases := []struct {
		err  error
		want bool
	}{
		{&pgconn.PgError{Code: "40001"}, true},  // serialization failure
		{&pgconn.PgError{Code: "40P01"}, true},  // deadlock
		{&pgconn.PgError{Code: "23505"}, false}, // unique violation
		{errors.New("plain"), false},
	}
	for _, tc := range cases {
		if got := isRetryable(tc.err); got != tc.want {
			t.Errorf("isRetryable(%v) = %v, want %v", tc.err, got, tc.want)
		}
	}
}

func TestErrorHelpers(t *testing.T) {
	if got := constraintName(&pgconn.PgError{ConstraintName: "uq_x"}); got != "uq_x" {
		t.Errorf("constraintName pg = %q, want uq_x", got)
	}
	if got := constraintName(errors.New("plain")); got != "" {
		t.Errorf("constraintName non-pg = %q, want empty", got)
	}
	if got := pgErrorCode(errors.New("plain")); got != "" {
		t.Errorf("pgErrorCode non-pg = %q, want empty", got)
	}
}

func TestTransferRepo_ErrorMapping(t *testing.T) {
	ctx := context.Background()
	r := NewTransferRepository()

	if _, err := r.Insert(ctx, fakeQuerier{scanErr: pgErr("23505", "uq_transfers_idempotency_key")}, domain.Transfer{}); !errors.Is(err, ErrDuplicateIdempotencyKey) {
		t.Errorf("Insert dup key: got %v, want ErrDuplicateIdempotencyKey", err)
	}
	if _, err := r.Insert(ctx, fakeQuerier{scanErr: errors.New("boom")}, domain.Transfer{}); err == nil || errors.Is(err, ErrDuplicateIdempotencyKey) {
		t.Errorf("Insert generic: got %v, want wrapped error", err)
	}
	if _, found, err := r.FindByIdempotencyKey(ctx, fakeQuerier{scanErr: pgx.ErrNoRows}, "k"); found || err != nil {
		t.Errorf("Find no rows: found=%v err=%v, want false/nil", found, err)
	}
	if _, _, err := r.FindByIdempotencyKey(ctx, fakeQuerier{scanErr: errors.New("boom")}, "k"); err == nil {
		t.Errorf("Find generic: want wrapped error")
	}
	if err := r.InsertLedgerEntry(ctx, fakeQuerier{execErr: errors.New("boom")}, domain.LedgerEntry{}); err == nil {
		t.Errorf("InsertLedgerEntry generic: want wrapped error")
	}
}
