package service

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"github.com/joeyyang/transfer-demo/internal/domain"
	"github.com/joeyyang/transfer-demo/internal/repository"
)

// TransferCmd is the input to a transfer.
// TransferCmd 是一次转账的输入参数。
type TransferCmd struct {
	SourceID       int64
	DestinationID  int64
	Amount         decimal.Decimal
	IdempotencyKey string // optional; empty means non-idempotent — 可选；为空表示非幂等
}

// TransferService moves money between two accounts atomically.
// TransferService 在两个账户之间原子地转移资金。
type TransferService struct {
	db        repository.Querier // pool, for idempotent re-fetch outside the tx — 连接池，用于事务外的幂等重查
	tx        TxManager
	accounts  AccountStore
	transfers TransferStore
}

// NewTransferService creates a TransferService.
// NewTransferService 创建一个 TransferService。
func NewTransferService(db repository.Querier, tx TxManager, accounts AccountStore, transfers TransferStore) *TransferService {
	return &TransferService{db: db, tx: tx, accounts: accounts, transfers: transfers}
}

// Transfer validates and executes a transfer in a single transaction.
// If an Idempotency-Key was supplied and already used, the original transfer
// is returned without moving money again.
// Transfer 在单个事务中校验并执行转账。
// 若提供的 Idempotency-Key 已被使用，则直接返回原转账，不再次扣款。
func (s *TransferService) Transfer(ctx context.Context, cmd TransferCmd) (domain.Transfer, error) {
	if err := s.validate(cmd); err != nil {
		return domain.Transfer{}, err
	}

	var result domain.Transfer
	err := s.tx.WithTx(ctx, func(q repository.Querier) error {
		// Idempotency fast path: return the existing transfer untouched.
		// 幂等快路径：命中则原样返回已有转账，不做任何改动。
		if cmd.IdempotencyKey != "" {
			existing, found, err := s.transfers.FindByIdempotencyKey(ctx, q, cmd.IdempotencyKey)
			if err != nil {
				return err
			}
			if found {
				if !matchesCmd(existing, cmd) {
					return domain.ErrIdempotencyConflict
				}
				result = existing
				return nil
			}
		}

		t, err := s.execute(ctx, q, cmd)
		if err != nil {
			return err
		}
		result = t
		return nil
	})

	// Lost the race: another tx inserted the same key. Return the original.
	// 竞态失败：另一个事务已插入相同幂等键。返回那一笔原始转账。
	if errors.Is(err, repository.ErrDuplicateIdempotencyKey) {
		return s.resolveDuplicate(ctx, cmd)
	}
	if err != nil {
		return domain.Transfer{}, err
	}
	return result, nil
}

// execute performs the locked balance update and ledger writes.
// execute 完成加锁后的余额更新与分录写入。
func (s *TransferService) execute(ctx context.Context, q repository.Querier, cmd TransferCmd) (domain.Transfer, error) {
	// Lock both rows in ascending id order to avoid deadlocks.
	// 按 id 升序锁定两行，避免死锁。
	locked, err := s.accounts.LockForUpdate(ctx, q, []int64{cmd.SourceID, cmd.DestinationID})
	if err != nil {
		return domain.Transfer{}, err
	}
	source, ok := locked[cmd.SourceID]
	if !ok {
		return domain.Transfer{}, domain.AccountNotFound(cmd.SourceID)
	}
	dest, ok := locked[cmd.DestinationID]
	if !ok {
		return domain.Transfer{}, domain.AccountNotFound(cmd.DestinationID)
	}

	// Reject if the source cannot cover the amount.
	// 源账户余额不足以覆盖金额时拒绝。
	if source.Balance.LessThan(cmd.Amount) {
		return domain.Transfer{}, domain.ErrInsufficientFunds
	}

	newSource := source.Balance.Sub(cmd.Amount)
	newDest := dest.Balance.Add(cmd.Amount)
	if err := s.accounts.UpdateBalance(ctx, q, source.ID, newSource); err != nil {
		return domain.Transfer{}, err
	}
	if err := s.accounts.UpdateBalance(ctx, q, dest.ID, newDest); err != nil {
		return domain.Transfer{}, err
	}

	// Record the transfer. A duplicate key here means a concurrent retry won.
	// 记录转账。此处若撞唯一键，说明并发重试中的另一笔已胜出。
	transfer := domain.Transfer{
		IdempotencyKey: optionalKey(cmd.IdempotencyKey),
		SourceID:       cmd.SourceID,
		DestinationID:  cmd.DestinationID,
		Amount:         cmd.Amount,
		Status:         domain.StatusCompleted,
	}
	id, err := s.transfers.Insert(ctx, q, transfer)
	if err != nil {
		return domain.Transfer{}, err
	}
	transfer.ID = id

	// Double-entry ledger: debit source, credit destination, with snapshots.
	// 复式记账：源账户借记、目标账户贷记，并记录余额快照。
	debit := domain.LedgerEntry{
		TransferID:   id,
		AccountID:    source.ID,
		Direction:    domain.DirectionDebit,
		Amount:       cmd.Amount,
		BalanceAfter: newSource,
	}
	credit := domain.LedgerEntry{
		TransferID:   id,
		AccountID:    dest.ID,
		Direction:    domain.DirectionCredit,
		Amount:       cmd.Amount,
		BalanceAfter: newDest,
	}
	if err := s.transfers.InsertLedgerEntry(ctx, q, debit); err != nil {
		return domain.Transfer{}, err
	}
	if err := s.transfers.InsertLedgerEntry(ctx, q, credit); err != nil {
		return domain.Transfer{}, err
	}
	return transfer, nil
}

// resolveDuplicate fetches the transfer that won the idempotency race.
// resolveDuplicate 取回在幂等竞态中胜出的那笔转账。
func (s *TransferService) resolveDuplicate(ctx context.Context, cmd TransferCmd) (domain.Transfer, error) {
	existing, found, err := s.transfers.FindByIdempotencyKey(ctx, s.db, cmd.IdempotencyKey)
	if err != nil {
		return domain.Transfer{}, err
	}
	if !found {
		return domain.Transfer{}, repository.ErrDuplicateIdempotencyKey
	}
	if !matchesCmd(existing, cmd) {
		return domain.Transfer{}, domain.ErrIdempotencyConflict
	}
	return existing, nil
}

// validate checks the command before any DB work.
// validate 在访问数据库前校验命令参数。
func (s *TransferService) validate(cmd TransferCmd) error {
	if cmd.SourceID == cmd.DestinationID {
		return domain.ErrSameAccount
	}
	if err := validateAmount(cmd.Amount); err != nil {
		return err
	}
	if cmd.IdempotencyKey != "" {
		if _, err := uuid.Parse(cmd.IdempotencyKey); err != nil {
			return domain.ErrInvalidIdempotency
		}
	}
	return nil
}

// matchesCmd reports whether an existing transfer has the same parameters,
// so a reused idempotency key with different params can be rejected.
// matchesCmd 判断已有转账与命令参数是否一致，
// 用于拒绝"同一幂等键、不同参数"的复用。
func matchesCmd(t domain.Transfer, cmd TransferCmd) bool {
	return t.SourceID == cmd.SourceID &&
		t.DestinationID == cmd.DestinationID &&
		t.Amount.Equal(cmd.Amount)
}

// optionalKey converts an empty key to nil (stored as NULL).
// optionalKey 把空字符串的键转为 nil（存为 NULL）。
func optionalKey(key string) *string {
	if key == "" {
		return nil
	}
	return &key
}
