package domain

import (
	"time"

	"github.com/shopspring/decimal"
)

// Account is a single account with its current balance.
type Account struct {
	ID        int64
	Balance   decimal.Decimal
	Version   int64
	CreatedAt time.Time
	UpdatedAt time.Time
}
