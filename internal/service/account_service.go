package service

import (
	"context"

	"github.com/shopspring/decimal"

	"github.com/joeyyang/transfer-demo/internal/domain"
	"github.com/joeyyang/transfer-demo/internal/repository"
)

// AccountService handles account creation and lookup.
// AccountService 处理账户的创建与查询。
type AccountService struct {
	db       repository.Querier // pool; single-statement ops need no transaction — 连接池；单语句操作无需事务
	accounts AccountStore
}

// NewAccountService creates an AccountService.
// NewAccountService 创建一个 AccountService。
func NewAccountService(db repository.Querier, accounts AccountStore) *AccountService {
	return &AccountService{db: db, accounts: accounts}
}

// Create opens a new account with the given opening balance.
// Create 用给定的开户余额创建一个新账户。
func (s *AccountService) Create(ctx context.Context, id int64, initialBalance decimal.Decimal) error {
	if err := validateAccountID(id); err != nil {
		return err
	}
	if err := validateInitialBalance(initialBalance); err != nil {
		return err
	}
	return s.accounts.Create(ctx, s.db, id, initialBalance)
}

// Get returns an account by id.
// Get 按 id 返回账户。
func (s *AccountService) Get(ctx context.Context, id int64) (domain.Account, error) {
	return s.accounts.Get(ctx, s.db, id)
}
