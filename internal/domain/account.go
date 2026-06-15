package domain

import (
	"time"

	"github.com/shopspring/decimal"
)

// Account is a single account with its current balance.
// Account 表示一个账户及其当前余额。
type Account struct {
	ID        int64
	Balance   decimal.Decimal
	Version   int64
	CreatedAt time.Time
	UpdatedAt time.Time
}
