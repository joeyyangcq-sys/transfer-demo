package service

import (
	"context"

	"github.com/shopspring/decimal"

	"github.com/joeyyang/transfer-demo/internal/domain"
	"github.com/joeyyang/transfer-demo/internal/repository"
)

// AccountStore is the account persistence behaviour the services need.
// AccountStore 是 service 所需的账户持久化行为接口。
type AccountStore interface {
	Create(ctx context.Context, q repository.Querier, id int64, balance decimal.Decimal) error
	Get(ctx context.Context, q repository.Querier, id int64) (domain.Account, error)
	LockForUpdate(ctx context.Context, q repository.Querier, ids []int64) (map[int64]domain.Account, error)
	UpdateBalance(ctx context.Context, q repository.Querier, id int64, balance decimal.Decimal) error
}

// TransferStore is the transfer/ledger persistence behaviour the services need.
// TransferStore 是 service 所需的转账/分录持久化行为接口。
type TransferStore interface {
	FindByIdempotencyKey(ctx context.Context, q repository.Querier, key string) (domain.Transfer, bool, error)
	Insert(ctx context.Context, q repository.Querier, t domain.Transfer) (int64, error)
	InsertLedgerEntry(ctx context.Context, q repository.Querier, e domain.LedgerEntry) error
}

// TxManager runs a function inside a database transaction.
// TxManager 在数据库事务中运行一个函数。
type TxManager interface {
	WithTx(ctx context.Context, fn func(q repository.Querier) error) error
}
