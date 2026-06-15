package service

import (
	"context"

	"github.com/shopspring/decimal"

	"github.com/joeyyang/internal-transfers/internal/domain"
	"github.com/joeyyang/internal-transfers/internal/repository"
)

// AccountStore is the account persistence behaviour the services need.
type AccountStore interface {
	Create(ctx context.Context, q repository.Querier, id int64, balance decimal.Decimal) error
	Get(ctx context.Context, q repository.Querier, id int64) (domain.Account, error)
	LockForUpdate(ctx context.Context, q repository.Querier, ids []int64) (map[int64]domain.Account, error)
	UpdateBalance(ctx context.Context, q repository.Querier, id int64, balance decimal.Decimal) error
}

// TransferStore is the transfer/ledger persistence behaviour the services need.
type TransferStore interface {
	FindByIdempotencyKey(ctx context.Context, q repository.Querier, key string) (domain.Transfer, bool, error)
	Insert(ctx context.Context, q repository.Querier, t domain.Transfer) (int64, error)
	InsertLedgerEntry(ctx context.Context, q repository.Querier, e domain.LedgerEntry) error
}

// TxManager runs a function inside a database transaction.
type TxManager interface {
	WithTx(ctx context.Context, fn func(q repository.Querier) error) error
}
