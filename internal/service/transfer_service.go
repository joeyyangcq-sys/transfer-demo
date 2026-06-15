package service

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"github.com/joeyyang/internal-transfers/internal/domain"
	"github.com/joeyyang/internal-transfers/internal/repository"
)

// TransferCmd is the input to a transfer.
type TransferCmd struct {
	SourceID       int64
	DestinationID  int64
	Amount         decimal.Decimal
	IdempotencyKey string // optional; empty means non-idempotent
}

// TransferService moves money between two accounts atomically.
type TransferService struct {
	db        repository.Querier // pool, for idempotent re-fetch outside the tx
	tx        TxManager
	accounts  AccountStore
	transfers TransferStore
}

// NewTransferService creates a TransferService.
func NewTransferService(db repository.Querier, tx TxManager, accounts AccountStore, transfers TransferStore) *TransferService {
	return &TransferService{db: db, tx: tx, accounts: accounts, transfers: transfers}
}

// Transfer validates and executes a transfer in a single transaction.
// If an Idempotency-Key was supplied and already used, the original transfer
// is returned without moving money again.
func (s *TransferService) Transfer(ctx context.Context, cmd TransferCmd) (domain.Transfer, error) {
	if err := s.validate(cmd); err != nil {
		return domain.Transfer{}, err
	}

	var result domain.Transfer
	err := s.tx.WithTx(ctx, func(q repository.Querier) error {
		// Idempotency fast path: return the existing transfer untouched.
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
	if errors.Is(err, repository.ErrDuplicateIdempotencyKey) {
		return s.resolveDuplicate(ctx, cmd)
	}
	if err != nil {
		return domain.Transfer{}, err
	}
	return result, nil
}

// execute performs the locked balance update and ledger writes.
func (s *TransferService) execute(ctx context.Context, q repository.Querier, cmd TransferCmd) (domain.Transfer, error) {
	// Lock both rows in ascending id order to avoid deadlocks.
	locked, err := s.accounts.LockForUpdate(ctx, q, []int64{cmd.SourceID, cmd.DestinationID})
	if err != nil {
		return domain.Transfer{}, err
	}
	source, ok := locked[cmd.SourceID]
	if !ok {
		return domain.Transfer{}, domain.ErrAccountNotFound
	}
	dest, ok := locked[cmd.DestinationID]
	if !ok {
		return domain.Transfer{}, domain.ErrAccountNotFound
	}

	// Reject if the source cannot cover the amount.
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
func matchesCmd(t domain.Transfer, cmd TransferCmd) bool {
	return t.SourceID == cmd.SourceID &&
		t.DestinationID == cmd.DestinationID &&
		t.Amount.Equal(cmd.Amount)
}

func optionalKey(key string) *string {
	if key == "" {
		return nil
	}
	return &key
}
