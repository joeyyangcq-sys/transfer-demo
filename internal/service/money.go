package service

import (
	"github.com/shopspring/decimal"

	"github.com/joeyyang/internal-transfers/internal/domain"
)

// maxDecimalPlaces matches the NUMERIC(38,18) scale.
const maxDecimalPlaces = 18

// maxValue is the exclusive upper bound for amounts (NUMERIC(38,18) allows up
// to 20 integer digits, i.e. values below 10^20).
var maxValue = decimal.New(1, 20) // 10^20

// validateAmount checks a transfer amount: strictly positive, within range,
// and no more than 18 decimal places.
func validateAmount(amount decimal.Decimal) error {
	if amount.Sign() <= 0 {
		return domain.ErrInvalidAmount
	}
	return checkRangeAndScale(amount)
}

// validateInitialBalance checks an opening balance: non-negative, within range,
// and no more than 18 decimal places.
func validateInitialBalance(balance decimal.Decimal) error {
	if balance.Sign() < 0 {
		return domain.ErrInvalidAmount
	}
	return checkRangeAndScale(balance)
}

func checkRangeAndScale(v decimal.Decimal) error {
	if v.Exponent() < -maxDecimalPlaces {
		return domain.ErrInvalidAmount
	}
	if v.Abs().GreaterThanOrEqual(maxValue) {
		return domain.ErrInvalidAmount
	}
	return nil
}
