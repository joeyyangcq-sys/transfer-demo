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

	// Race simulation: the in-tx lookup misses, Insert hits a duplicate key,
	// and the post-tx lookup resolves to seeded (covers resolveDuplicate).
	// 竞态模拟：事务内查不到、Insert 撞唯一键、事务后查到 seeded（覆盖 resolveDuplicate）。
	raceDup      bool
	raceFindMiss bool  // even the post-tx lookup misses — 连事务后查询也查不到
	raceFindErr  error // the post-tx lookup itself errors — 事务后查询本身报错
	findCount    int
	seeded       domain.Transfer

	// Step-level error injection for execute's failure paths.
	// 针对 execute 各失败路径的分步错误注入。
	lockErr     error
	updateErr   error
	updateOn    int // which UpdateBalance call fails (1=source, 2=dest, 0=any)
	updateCalls int
	insertErr   error
	ledgerErr   error
	ledgerOn    int // which InsertLedgerEntry call fails (1=debit, 2=credit, 0=any)
	ledgerCalls int
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
	if f.lockErr != nil {
		return nil, f.lockErr
	}
	out := map[int64]domain.Account{}
	for _, id := range ids {
		if a, ok := f.accounts[id]; ok {
			out[id] = a
		}
	}
	return out, nil
}

func (f *fakeStore) UpdateBalance(_ context.Context, _ repository.Querier, id int64, balance decimal.Decimal) error {
	f.updateCalls++
	if f.updateErr != nil && (f.updateOn == 0 || f.updateOn == f.updateCalls) {
		return f.updateErr
	}
	a := f.accounts[id]
	a.Balance = balance
	f.accounts[id] = a
	return nil
}

// --- TransferStore ---

func (f *fakeStore) FindByIdempotencyKey(_ context.Context, _ repository.Querier, key string) (domain.Transfer, bool, error) {
	if f.raceDup {
		// First call (inside tx) misses; later call (resolveDuplicate) hits,
		// unless raceFindMiss forces it to keep missing.
		// 第一次调用（事务内）查不到；之后的调用（resolveDuplicate）查到，
		// 除非 raceFindMiss 强制持续查不到。
		f.findCount++
		if f.findCount == 1 || f.raceFindMiss {
			return domain.Transfer{}, false, nil
		}
		if f.raceFindErr != nil {
			return domain.Transfer{}, false, f.raceFindErr
		}
		return f.seeded, true, nil
	}
	for _, t := range f.transfers {
		if t.IdempotencyKey != nil && *t.IdempotencyKey == key {
			return t, true, nil
		}
	}
	return domain.Transfer{}, false, nil
}

func (f *fakeStore) Insert(_ context.Context, _ repository.Querier, t domain.Transfer) (int64, error) {
	if f.raceDup {
		return 0, repository.ErrDuplicateIdempotencyKey
	}
	if f.insertErr != nil {
		return 0, f.insertErr
	}
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
	f.ledgerCalls++
	if f.ledgerErr != nil && (f.ledgerOn == 0 || f.ledgerOn == f.ledgerCalls) {
		return f.ledgerErr
	}
	f.ledger = append(f.ledger, e)
	return nil
}
