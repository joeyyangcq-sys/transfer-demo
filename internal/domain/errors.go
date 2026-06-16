package domain

import (
	"errors"
	"fmt"
)

// Domain errors. The API layer maps these to HTTP status codes.
// 领域错误。API 层会把它们映射为对应的 HTTP 状态码。
var (
	ErrAccountNotFound      = errors.New("account not found")
	ErrAccountAlreadyExists = errors.New("account already exists")
	ErrInsufficientFunds    = errors.New("insufficient funds")
	ErrInvalidAmount        = errors.New("invalid amount")
	ErrInvalidAccountID     = errors.New("invalid account id")
	ErrSameAccount          = errors.New("source and destination must differ")
	ErrInvalidIdempotency   = errors.New("invalid idempotency key")
	// ErrIdempotencyConflict: same key reused with different request params.
	// ErrIdempotencyConflict：同一幂等键被复用但请求参数不同。
	ErrIdempotencyConflict = errors.New("idempotency key conflict")
)

// AccountNotFoundError reports which account was missing. It unwraps to
// ErrAccountNotFound, so existing errors.Is checks and the HTTP status mapping
// keep working while the message names the specific account id.
// AccountNotFoundError 指明缺失的是哪个账户。它会 Unwrap 到 ErrAccountNotFound，
// 因此既有的 errors.Is 判断与 HTTP 状态码映射照常工作，而错误消息会带上具体账户 id。
type AccountNotFoundError struct {
	ID int64
}

func (e *AccountNotFoundError) Error() string {
	return fmt.Sprintf("account %d not found", e.ID)
}

func (e *AccountNotFoundError) Unwrap() error { return ErrAccountNotFound }

// AccountNotFound builds an AccountNotFoundError for the given account id.
// AccountNotFound 为给定账户 id 构造一个 AccountNotFoundError。
func AccountNotFound(id int64) error {
	return &AccountNotFoundError{ID: id}
}
