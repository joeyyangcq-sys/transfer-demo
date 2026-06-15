package service

import (
	"github.com/shopspring/decimal"

	"github.com/joeyyang/internal-transfers/internal/domain"
)

// maxDecimalPlaces matches the NUMERIC(38,18) scale.
// maxDecimalPlaces 与 NUMERIC(38,18) 的小数位数一致。
const maxDecimalPlaces = 18

// maxValue is the exclusive upper bound for amounts (NUMERIC(38,18) allows up
// to 20 integer digits, i.e. values below 10^20).
// maxValue 是金额的开区间上界（NUMERIC(38,18) 整数位最多 20 位，即小于 10^20）。
var maxValue = decimal.New(1, 20) // 10^20

// validateAmount checks a transfer amount: strictly positive, within range,
// and no more than 18 decimal places.
// validateAmount 校验转账金额：必须为正、在范围内、小数位不超过 18。
func validateAmount(amount decimal.Decimal) error {
	if amount.Sign() <= 0 {
		return domain.ErrInvalidAmount
	}
	return checkRangeAndScale(amount)
}

// validateInitialBalance checks an opening balance: non-negative, within range,
// and no more than 18 decimal places.
// validateInitialBalance 校验开户余额：非负、在范围内、小数位不超过 18。
func validateInitialBalance(balance decimal.Decimal) error {
	if balance.Sign() < 0 {
		return domain.ErrInvalidAmount
	}
	return checkRangeAndScale(balance)
}

// checkRangeAndScale enforces the value range and decimal-scale limits.
// checkRangeAndScale 校验数值范围与小数位上限。
func checkRangeAndScale(v decimal.Decimal) error {
	if v.Exponent() < -maxDecimalPlaces {
		return domain.ErrInvalidAmount
	}
	if v.Abs().GreaterThanOrEqual(maxValue) {
		return domain.ErrInvalidAmount
	}
	return nil
}
