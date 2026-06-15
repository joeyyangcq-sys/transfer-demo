package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/joeyyang/internal-transfers/internal/domain"
)

// ErrDuplicateIdempotencyKey signals a race where another transaction already
// inserted a transfer with the same idempotency key. The service re-fetches it.
// ErrDuplicateIdempotencyKey 表示发生了竞态：另一个事务已用相同幂等键插入了转账。
// service 会据此重新查询该笔转账。
var ErrDuplicateIdempotencyKey = errors.New("duplicate idempotency key")

// TransferRepository writes transfers and ledger entries, and looks up
// transfers by idempotency key.
// TransferRepository 负责写入转账与分录，并按幂等键查询转账。
type TransferRepository struct{}

// NewTransferRepository creates a TransferRepository.
// NewTransferRepository 创建一个 TransferRepository。
func NewTransferRepository() *TransferRepository {
	return &TransferRepository{}
}

// FindByIdempotencyKey returns the transfer for key, and whether it was found.
// FindByIdempotencyKey 按幂等键返回转账，以及是否找到。
func (r *TransferRepository) FindByIdempotencyKey(ctx context.Context, q Querier, key string) (domain.Transfer, bool, error) {
	var t domain.Transfer
	var k *string
	err := q.QueryRow(ctx,
		`SELECT id, idempotency_key::text, source_id, destination_id, amount, status, created_at
		   FROM transfers
		  WHERE idempotency_key = $1::uuid`,
		key,
	).Scan(&t.ID, &k, &t.SourceID, &t.DestinationID, &t.Amount, &t.Status, &t.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Transfer{}, false, nil
		}
		return domain.Transfer{}, false, fmt.Errorf("find transfer: %w", err)
	}
	t.IdempotencyKey = k
	return t, true, nil
}

// Insert writes a transfer and returns its generated id.
// Returns ErrDuplicateIdempotencyKey if the key already exists.
// Insert 写入一笔转账并返回自增 id；若幂等键已存在则返回 ErrDuplicateIdempotencyKey。
func (r *TransferRepository) Insert(ctx context.Context, q Querier, t domain.Transfer) (int64, error) {
	var id int64
	err := q.QueryRow(ctx,
		`INSERT INTO transfers (idempotency_key, source_id, destination_id, amount, status)
		 VALUES ($1::uuid, $2, $3, $4, $5)
		 RETURNING id`,
		t.IdempotencyKey, t.SourceID, t.DestinationID, t.Amount, t.Status,
	).Scan(&id)
	if err != nil {
		if isUniqueViolation(err, "uq_transfers_idempotency_key") {
			return 0, ErrDuplicateIdempotencyKey
		}
		return 0, fmt.Errorf("insert transfer: %w", err)
	}
	return id, nil
}

// InsertLedgerEntry writes one side of a transfer's double-entry record.
// InsertLedgerEntry 写入一笔转账复式记账中的一条分录。
func (r *TransferRepository) InsertLedgerEntry(ctx context.Context, q Querier, e domain.LedgerEntry) error {
	_, err := q.Exec(ctx,
		`INSERT INTO ledger_entries (transfer_id, account_id, direction, amount, balance_after)
		 VALUES ($1, $2, $3, $4, $5)`,
		e.TransferID, e.AccountID, string(e.Direction), e.Amount, e.BalanceAfter,
	)
	if err != nil {
		return fmt.Errorf("insert ledger entry: %w", err)
	}
	return nil
}
