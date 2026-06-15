package domain

import (
	"time"

	"github.com/shopspring/decimal"
)

// Direction marks whether a ledger entry takes money out or in.
type Direction string

const (
	DirectionDebit  Direction = "debit"  // money leaves the account
	DirectionCredit Direction = "credit" // money enters the account
)

// Transfer status values.
const (
	StatusCompleted = "completed"
	StatusFailed    = "failed"
)

// Transfer is one money movement between two accounts (immutable record).
type Transfer struct {
	ID             int64
	IdempotencyKey *string // nil when the caller sent no Idempotency-Key
	SourceID       int64
	DestinationID  int64
	Amount         decimal.Decimal
	Status         string
	CreatedAt      time.Time
}

// LedgerEntry is one side of a transfer's double-entry record.
type LedgerEntry struct {
	ID           int64
	TransferID   int64
	AccountID    int64
	Direction    Direction
	Amount       decimal.Decimal
	BalanceAfter decimal.Decimal
	CreatedAt    time.Time
}
