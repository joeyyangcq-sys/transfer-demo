package service

import (
	"context"
	"errors"
	"testing"

	"github.com/joeyyang/transfer-demo/internal/domain"
)

func TestAccountService_CreateRejectsNegativeBalance(t *testing.T) {
	f := newFakeStore()
	err := NewAccountService(nil, f, f).Create(context.Background(), 1, dec("-1"))
	if !errors.Is(err, domain.ErrInvalidAmount) {
		t.Fatalf("err = %v, want ErrInvalidAmount", err)
	}
}

func TestAccountService_CreateRejectsNonPositiveID(t *testing.T) {
	// A missing account_id JSON field decodes to 0; both 0 and negative ids
	// must be rejected rather than silently creating an account.
	// 缺失的 account_id 字段会解码为 0；0 与负数都必须被拒绝，而非静默建号。
	for _, id := range []int64{0, -1} {
		f := newFakeStore()
		err := NewAccountService(nil, f, f).Create(context.Background(), id, dec("10"))
		if !errors.Is(err, domain.ErrInvalidAccountID) {
			t.Errorf("id %d: err = %v, want ErrInvalidAccountID", id, err)
		}
	}
}

func TestAccountService_CreateAndGet(t *testing.T) {
	f := newFakeStore()
	svc := NewAccountService(nil, f, f)
	if err := svc.Create(context.Background(), 1, dec("10.5")); err != nil {
		t.Fatalf("create: %v", err)
	}
	got, err := svc.Get(context.Background(), 1)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if !got.Balance.Equal(dec("10.5")) {
		t.Errorf("balance = %s, want 10.5", got.Balance)
	}
}

func TestAccountService_GetRejectsNonPositiveID(t *testing.T) {
	f := newFakeStore()
	svc := NewAccountService(nil, f, f)
	for _, id := range []int64{0, -1} {
		_, err := svc.Get(context.Background(), id)
		if !errors.Is(err, domain.ErrInvalidAccountID) {
			t.Errorf("id %d: err = %v, want ErrInvalidAccountID", id, err)
		}
	}
}
