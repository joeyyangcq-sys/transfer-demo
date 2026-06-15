package domain

import "errors"

// Domain errors. The API layer maps these to HTTP status codes.
// 领域错误。API 层会把它们映射为对应的 HTTP 状态码。
var (
	ErrAccountNotFound      = errors.New("account not found")
	ErrAccountAlreadyExists = errors.New("account already exists")
	ErrInsufficientFunds    = errors.New("insufficient funds")
	ErrInvalidAmount        = errors.New("invalid amount")
	ErrSameAccount          = errors.New("source and destination must differ")
	ErrInvalidIdempotency   = errors.New("invalid idempotency key")
	// ErrIdempotencyConflict: same key reused with different request params.
	// ErrIdempotencyConflict：同一幂等键被复用但请求参数不同。
	ErrIdempotencyConflict = errors.New("idempotency key conflict")
)
