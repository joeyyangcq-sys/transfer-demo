package service

import (
	"context"

	"github.com/shopspring/decimal"

	"github.com/joeyyang/internal-transfers/internal/domain"
	"github.com/joeyyang/internal-transfers/internal/repository"
)

// fakeStore is an in-memory AccountStore + TransferStore + TxManager for tests.
// fakeStore 是测试用的内存实现，同时充当 AccountStore + TransferStore + TxManager。
type fakeStore struct {
	accounts  map[int64]domain.Account
	transfers []domain.Transfer
	ledger    []domain.LedgerEntry
	nextID    int64
}

func newFakeStore() *fakeStore {
	return &fakeStore{accounts: map[int64]domain.Account{}, nextID: 1}
}

func (f *fakeStore) addAccount(id int64, balance string) {
	f.accounts[id] = domain.Account{ID: id, Balance: decimal.RequireFromString(balance)}
}

// --- TxManager ---

func (f *fakeStore) WithTx(ctx context.Context, fn func(q repository.Querier) error) error {
	return fn(nil)
}

// --- AccountStore ---

func (f *fakeStore) Create(_ context.Context, _ repository.Querier, id int64, balance decimal.Decimal) error {
	if _, ok := f.accounts[id]; ok {
		return domain.ErrAccountAlreadyExists
	}
	f.accounts[id] = domain.Account{ID: id, Balance: balance}
	return nil
}

func (f *fakeStore) Get(_ context.Context, _ repository.Querier, id int64) (domain.Account, error) {
	a, ok := f.accounts[id]
	if !ok {
		return domain.Account{}, domain.ErrAccountNotFound
	}
	return a, nil
}

func (f *fakeStore) LockForUpdate(_ context.Context, _ repository.Querier, ids []int64) (map[int64]domain.Account, error) {
	out := map[int64]domain.Account{}
	for _, id := range ids {
		if a, ok := f.accounts[id]; ok {
			out[id] = a
		}
	}
	return out, nil
}

func (f *fakeStore) UpdateBalance(_ context.Context, _ repository.Querier, id int64, balance decimal.Decimal) error {
	a := f.accounts[id]
	a.Balance = balance
	f.accounts[id] = a
	return nil
}

// --- TransferStore ---

func (f *fakeStore) FindByIdempotencyKey(_ context.Context, _ repository.Querier, key string) (domain.Transfer, bool, error) {
	for _, t := range f.transfers {
		if t.IdempotencyKey != nil && *t.IdempotencyKey == key {
			return t, true, nil
		}
	}
	return domain.Transfer{}, false, nil
}

func (f *fakeStore) Insert(_ context.Context, _ repository.Querier, t domain.Transfer) (int64, error) {
	if t.IdempotencyKey != nil {
		for _, e := range f.transfers {
			if e.IdempotencyKey != nil && *e.IdempotencyKey == *t.IdempotencyKey {
				return 0, repository.ErrDuplicateIdempotencyKey
			}
		}
	}
	t.ID = f.nextID
	f.nextID++
	f.transfers = append(f.transfers, t)
	return t.ID, nil
}

func (f *fakeStore) InsertLedgerEntry(_ context.Context, _ repository.Querier, e domain.LedgerEntry) error {
	f.ledger = append(f.ledger, e)
	return nil
}
