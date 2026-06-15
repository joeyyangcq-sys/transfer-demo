package domain

import (
	"time"

	"github.com/shopspring/decimal"
)

// Direction marks whether a ledger entry takes money out or in.
// Direction 标记一条分录是出账还是入账。
type Direction string

const (
	DirectionDebit  Direction = "debit"  // money leaves the account — 资金流出账户
	DirectionCredit Direction = "credit" // money enters the account — 资金流入账户
)

// Transfer status values.
// 转账状态取值。
const (
	StatusCompleted = "completed"
	StatusFailed    = "failed"
)

// Transfer is one money movement between two accounts (immutable record).
// Transfer 表示两个账户之间的一次资金转移（不可变记录）。
type Transfer struct {
	ID             int64
	IdempotencyKey *string // nil when the caller sent no Idempotency-Key — 调用方未传 Idempotency-Key 时为 nil
	SourceID       int64
	DestinationID  int64
	Amount         decimal.Decimal
	Status         string
	CreatedAt      time.Time
}

// LedgerEntry is one side of a transfer's double-entry record.
// LedgerEntry 表示一笔转账复式记账中的一条分录。
type LedgerEntry struct {
	ID           int64
	TransferID   int64
	AccountID    int64
	Direction    Direction
	Amount       decimal.Decimal
	BalanceAfter decimal.Decimal
	CreatedAt    time.Time
}
