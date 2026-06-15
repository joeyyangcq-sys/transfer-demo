package service

import (
	"context"

	"github.com/shopspring/decimal"

	"github.com/joeyyang/internal-transfers/internal/domain"
	"github.com/joeyyang/internal-transfers/internal/repository"
)

// AccountService handles account creation and lookup.
type AccountService struct {
	db       repository.Querier // pool; single-statement ops need no transaction
	accounts AccountStore
}

// NewAccountService creates an AccountService.
func NewAccountService(db repository.Querier, accounts AccountStore) *AccountService {
	return &AccountService{db: db, accounts: accounts}
}

// Create opens a new account with the given opening balance.
func (s *AccountService) Create(ctx context.Context, id int64, initialBalance decimal.Decimal) error {
	if err := validateInitialBalance(initialBalance); err != nil {
		return err
	}
	return s.accounts.Create(ctx, s.db, id, initialBalance)
}

// Get returns an account by id.
func (s *AccountService) Get(ctx context.Context, id int64) (domain.Account, error) {
	return s.accounts.Get(ctx, s.db, id)
}
