package domain

import "errors"

// Domain errors. The API layer maps these to HTTP status codes.
var (
	ErrAccountNotFound      = errors.New("account not found")
	ErrAccountAlreadyExists = errors.New("account already exists")
	ErrInsufficientFunds    = errors.New("insufficient funds")
	ErrInvalidAmount        = errors.New("invalid amount")
	ErrSameAccount          = errors.New("source and destination must differ")
	ErrInvalidIdempotency   = errors.New("invalid idempotency key")
	// ErrIdempotencyConflict: same key reused with different request params.
	ErrIdempotencyConflict = errors.New("idempotency key conflict")
)
