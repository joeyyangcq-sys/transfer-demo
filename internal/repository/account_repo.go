package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/shopspring/decimal"

	"github.com/joeyyang/internal-transfers/internal/domain"
)

// AccountRepository reads and writes account rows.
type AccountRepository struct{}

// NewAccountRepository creates an AccountRepository.
func NewAccountRepository() *AccountRepository {
	return &AccountRepository{}
}

// Create inserts a new account. Returns ErrAccountAlreadyExists on duplicate id.
func (r *AccountRepository) Create(ctx context.Context, q Querier, id int64, balance decimal.Decimal) error {
	_, err := q.Exec(ctx,
		`INSERT INTO accounts (id, balance) VALUES ($1, $2)`,
		id, balance,
	)
	if err != nil {
		if isUniqueViolation(err, "accounts_pkey") {
			return domain.ErrAccountAlreadyExists
		}
		return fmt.Errorf("insert account: %w", err)
	}
	return nil
}

// Get returns one account. Returns ErrAccountNotFound when missing.
func (r *AccountRepository) Get(ctx context.Context, q Querier, id int64) (domain.Account, error) {
	var a domain.Account
	err := q.QueryRow(ctx,
		`SELECT id, balance, version, created_at, updated_at FROM accounts WHERE id = $1`,
		id,
	).Scan(&a.ID, &a.Balance, &a.Version, &a.CreatedAt, &a.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Account{}, domain.ErrAccountNotFound
		}
		return domain.Account{}, fmt.Errorf("get account: %w", err)
	}
	return a, nil
}

// LockForUpdate locks the given accounts FOR UPDATE in ascending id order to
// avoid deadlocks, and returns their current balances keyed by id.
func (r *AccountRepository) LockForUpdate(ctx context.Context, q Querier, ids []int64) (map[int64]domain.Account, error) {
	rows, err := q.Query(ctx,
		`SELECT id, balance, version, created_at, updated_at
		   FROM accounts
		  WHERE id = ANY($1)
		  ORDER BY id
		    FOR UPDATE`,
		ids,
	)
	if err != nil {
		return nil, fmt.Errorf("lock accounts: %w", err)
	}
	defer rows.Close()

	out := make(map[int64]domain.Account, len(ids))
	for rows.Next() {
		var a domain.Account
		if err := rows.Scan(&a.ID, &a.Balance, &a.Version, &a.CreatedAt, &a.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan account: %w", err)
		}
		out[a.ID] = a
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate accounts: %w", err)
	}
	return out, nil
}

// UpdateBalance sets a new balance and bumps version/updated_at.
func (r *AccountRepository) UpdateBalance(ctx context.Context, q Querier, id int64, balance decimal.Decimal) error {
	_, err := q.Exec(ctx,
		`UPDATE accounts
		    SET balance = $2, version = version + 1, updated_at = now()
		  WHERE id = $1`,
		id, balance,
	)
	if err != nil {
		if pgErrorCode(err) == codeCheckViolation {
			// Safety net: balance_non_negative tripped despite app checks.
			return domain.ErrInsufficientFunds
		}
		return fmt.Errorf("update balance: %w", err)
	}
	return nil
}
