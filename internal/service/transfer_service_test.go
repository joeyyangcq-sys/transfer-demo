package service

import (
	"context"
	"errors"
	"testing"

	"github.com/shopspring/decimal"

	"github.com/joeyyang/transfer-demo/internal/domain"
	"github.com/joeyyang/transfer-demo/internal/repository"
)

func newTransferSvc(f *fakeStore) *TransferService {
	return NewTransferService(nil, f, f, f)
}

func dec(s string) decimal.Decimal { return decimal.RequireFromString(s) }

func TestTransfer_Success(t *testing.T) {
	f := newFakeStore()
	f.addAccount(1, "100.00")
	f.addAccount(2, "50.00")
	svc := newTransferSvc(f)

	_, err := svc.Transfer(context.Background(), TransferCmd{SourceID: 1, DestinationID: 2, Amount: dec("30")})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got := f.accounts[1].Balance; !got.Equal(dec("70")) {
		t.Errorf("source balance = %s, want 70", got)
	}
	if got := f.accounts[2].Balance; !got.Equal(dec("80")) {
		t.Errorf("dest balance = %s, want 80", got)
	}
	if len(f.ledger) != 2 {
		t.Fatalf("ledger entries = %d, want 2", len(f.ledger))
	}
	// debit then credit, balance_after snapshots recorded.
	// 先借记后贷记，且记录了 balance_after 余额快照。
	if f.ledger[0].Direction != domain.DirectionDebit || !f.ledger[0].BalanceAfter.Equal(dec("70")) {
		t.Errorf("debit entry wrong: %+v", f.ledger[0])
	}
	if f.ledger[1].Direction != domain.DirectionCredit || !f.ledger[1].BalanceAfter.Equal(dec("80")) {
		t.Errorf("credit entry wrong: %+v", f.ledger[1])
	}
}

func TestTransfer_InsufficientFunds(t *testing.T) {
	f := newFakeStore()
	f.addAccount(1, "10")
	f.addAccount(2, "0")
	svc := newTransferSvc(f)

	_, err := svc.Transfer(context.Background(), TransferCmd{SourceID: 1, DestinationID: 2, Amount: dec("11")})
	if !errors.Is(err, domain.ErrInsufficientFunds) {
		t.Fatalf("err = %v, want ErrInsufficientFunds", err)
	}
	// Balances untouched.
	// 余额未被改动。
	if !f.accounts[1].Balance.Equal(dec("10")) {
		t.Errorf("source balance changed: %s", f.accounts[1].Balance)
	}
}

func TestTransfer_AccountNotFound(t *testing.T) {
	f := newFakeStore()
	f.addAccount(1, "100")
	svc := newTransferSvc(f)

	_, err := svc.Transfer(context.Background(), TransferCmd{SourceID: 1, DestinationID: 2, Amount: dec("10")})
	if !errors.Is(err, domain.ErrAccountNotFound) {
		t.Fatalf("err = %v, want ErrAccountNotFound", err)
	}
}

func TestTransfer_SameAccount(t *testing.T) {
	f := newFakeStore()
	f.addAccount(1, "100")
	svc := newTransferSvc(f)

	_, err := svc.Transfer(context.Background(), TransferCmd{SourceID: 1, DestinationID: 1, Amount: dec("10")})
	if !errors.Is(err, domain.ErrSameAccount) {
		t.Fatalf("err = %v, want ErrSameAccount", err)
	}
}

func TestTransfer_RejectsNonPositiveAccountID(t *testing.T) {
	// A missing source/destination JSON field decodes to 0 and must be rejected
	// before any DB work, naming it as an invalid account id.
	// 缺失的 source/destination 字段会解码为 0，须在访问数据库前以非法账户 id 拒绝。
	cases := []TransferCmd{
		{SourceID: 0, DestinationID: 2, Amount: dec("10")},
		{SourceID: 1, DestinationID: 0, Amount: dec("10")},
		{SourceID: -1, DestinationID: 2, Amount: dec("10")},
	}
	for _, cmd := range cases {
		f := newFakeStore()
		_, err := newTransferSvc(f).Transfer(context.Background(), cmd)
		if !errors.Is(err, domain.ErrInvalidAccountID) {
			t.Errorf("cmd %+v: err = %v, want ErrInvalidAccountID", cmd, err)
		}
	}
}

func TestTransfer_InvalidAmount(t *testing.T) {
	f := newFakeStore()
	f.addAccount(1, "100")
	f.addAccount(2, "0")
	svc := newTransferSvc(f)

	for _, amt := range []string{"0", "-5"} {
		_, err := svc.Transfer(context.Background(), TransferCmd{SourceID: 1, DestinationID: 2, Amount: dec(amt)})
		if !errors.Is(err, domain.ErrInvalidAmount) {
			t.Errorf("amount %s: err = %v, want ErrInvalidAmount", amt, err)
		}
	}
}

func TestTransfer_InvalidIdempotencyKey(t *testing.T) {
	f := newFakeStore()
	f.addAccount(1, "100")
	f.addAccount(2, "0")
	svc := newTransferSvc(f)

	_, err := svc.Transfer(context.Background(), TransferCmd{SourceID: 1, DestinationID: 2, Amount: dec("10"), IdempotencyKey: "not-a-uuid"})
	if !errors.Is(err, domain.ErrInvalidIdempotency) {
		t.Fatalf("err = %v, want ErrInvalidIdempotency", err)
	}
}

func TestTransfer_IdempotentReplay(t *testing.T) {
	f := newFakeStore()
	f.addAccount(1, "100")
	f.addAccount(2, "0")
	svc := newTransferSvc(f)
	key := "550e8400-e29b-41d4-a716-446655440000"
	cmd := TransferCmd{SourceID: 1, DestinationID: 2, Amount: dec("30"), IdempotencyKey: key}

	first, err := svc.Transfer(context.Background(), cmd)
	if err != nil {
		t.Fatalf("first transfer: %v", err)
	}
	second, err := svc.Transfer(context.Background(), cmd)
	if err != nil {
		t.Fatalf("replay: %v", err)
	}

	if first.ID != second.ID {
		t.Errorf("replay returned different transfer: %d vs %d", first.ID, second.ID)
	}
	// Money moved only once.
	// 资金只移动了一次。
	if !f.accounts[1].Balance.Equal(dec("70")) {
		t.Errorf("source balance = %s, want 70 (debited once)", f.accounts[1].Balance)
	}
	if len(f.transfers) != 1 {
		t.Errorf("transfers recorded = %d, want 1", len(f.transfers))
	}
}

func TestTransfer_IdempotencyRaceResolvesToOriginal(t *testing.T) {
	f := newFakeStore()
	f.addAccount(1, "100")
	f.addAccount(2, "0")
	key := "550e8400-e29b-41d4-a716-446655440000"
	// Simulate the lost race: another tx already committed this transfer.
	// 模拟竞态失败：另一个事务已提交了这笔转账。
	f.raceDup = true
	f.seeded = domain.Transfer{ID: 99, IdempotencyKey: &key, SourceID: 1, DestinationID: 2, Amount: dec("30"), Status: domain.StatusCompleted}
	svc := newTransferSvc(f)

	got, err := svc.Transfer(context.Background(), TransferCmd{SourceID: 1, DestinationID: 2, Amount: dec("30"), IdempotencyKey: key})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.ID != 99 {
		t.Errorf("resolved transfer id = %d, want 99 (the original)", got.ID)
	}
}

func TestTransfer_IdempotencyRaceConflict(t *testing.T) {
	f := newFakeStore()
	f.addAccount(1, "100")
	f.addAccount(2, "0")
	key := "550e8400-e29b-41d4-a716-446655440000"
	f.raceDup = true
	// Refetched transfer has a different amount -> conflict.
	// 回查到的转账金额不同 -> 冲突。
	f.seeded = domain.Transfer{ID: 99, IdempotencyKey: &key, SourceID: 1, DestinationID: 2, Amount: dec("40")}
	svc := newTransferSvc(f)

	_, err := svc.Transfer(context.Background(), TransferCmd{SourceID: 1, DestinationID: 2, Amount: dec("30"), IdempotencyKey: key})
	if !errors.Is(err, domain.ErrIdempotencyConflict) {
		t.Fatalf("err = %v, want ErrIdempotencyConflict", err)
	}
}

func TestTransfer_IdempotencyRaceRefetchMissing(t *testing.T) {
	f := newFakeStore()
	f.addAccount(1, "100")
	f.addAccount(2, "0")
	key := "550e8400-e29b-41d4-a716-446655440000"
	f.raceDup = true
	f.raceFindMiss = true // duplicate on insert, but refetch finds nothing
	svc := newTransferSvc(f)

	_, err := svc.Transfer(context.Background(), TransferCmd{SourceID: 1, DestinationID: 2, Amount: dec("30"), IdempotencyKey: key})
	if !errors.Is(err, repository.ErrDuplicateIdempotencyKey) {
		t.Fatalf("err = %v, want ErrDuplicateIdempotencyKey", err)
	}
}

func TestTransfer_IdempotencyRaceRefetchError(t *testing.T) {
	f := newFakeStore()
	f.addAccount(1, "100")
	f.addAccount(2, "0")
	key := "550e8400-e29b-41d4-a716-446655440000"
	f.raceDup = true
	f.raceFindErr = errors.New("refetch boom") // refetch after the dup error fails
	svc := newTransferSvc(f)

	_, err := svc.Transfer(context.Background(), TransferCmd{SourceID: 1, DestinationID: 2, Amount: dec("30"), IdempotencyKey: key})
	if err == nil || errors.Is(err, repository.ErrDuplicateIdempotencyKey) {
		t.Fatalf("err = %v, want the refetch error", err)
	}
}

func TestTransfer_ExecuteFailurePaths(t *testing.T) {
	boom := errors.New("boom")
	cases := []struct {
		name  string
		setup func(f *fakeStore)
	}{
		{"lock error", func(f *fakeStore) { f.lockErr = boom }},
		{"source update error", func(f *fakeStore) { f.updateErr = boom; f.updateOn = 1 }},
		{"dest update error", func(f *fakeStore) { f.updateErr = boom; f.updateOn = 2 }},
		{"insert error", func(f *fakeStore) { f.insertErr = boom }},
		{"debit ledger error", func(f *fakeStore) { f.ledgerErr = boom; f.ledgerOn = 1 }},
		{"credit ledger error", func(f *fakeStore) { f.ledgerErr = boom; f.ledgerOn = 2 }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newFakeStore()
			f.addAccount(1, "100")
			f.addAccount(2, "0")
			tc.setup(f)
			_, err := newTransferSvc(f).Transfer(context.Background(),
				TransferCmd{SourceID: 1, DestinationID: 2, Amount: dec("30")})
			if !errors.Is(err, boom) {
				t.Fatalf("err = %v, want boom", err)
			}
		})
	}
}

func TestTransfer_SourceNotFound(t *testing.T) {
	f := newFakeStore()
	f.addAccount(2, "0") // only the destination exists
	_, err := newTransferSvc(f).Transfer(context.Background(),
		TransferCmd{SourceID: 1, DestinationID: 2, Amount: dec("10")})
	if !errors.Is(err, domain.ErrAccountNotFound) {
		t.Fatalf("err = %v, want ErrAccountNotFound", err)
	}
}

func TestTransfer_IdempotencyConflict(t *testing.T) {
	f := newFakeStore()
	f.addAccount(1, "100")
	f.addAccount(2, "0")
	svc := newTransferSvc(f)
	key := "550e8400-e29b-41d4-a716-446655440000"

	if _, err := svc.Transfer(context.Background(), TransferCmd{SourceID: 1, DestinationID: 2, Amount: dec("30"), IdempotencyKey: key}); err != nil {
		t.Fatalf("first transfer: %v", err)
	}
	// Same key, different amount.
	// 相同幂等键，不同金额。
	_, err := svc.Transfer(context.Background(), TransferCmd{SourceID: 1, DestinationID: 2, Amount: dec("40"), IdempotencyKey: key})
	if !errors.Is(err, domain.ErrIdempotencyConflict) {
		t.Fatalf("err = %v, want ErrIdempotencyConflict", err)
	}
}
