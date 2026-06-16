package service

import (
	"context"
	"errors"
	"testing"

	"github.com/joeyyang/internal-transfers/internal/domain"
)

func TestAccountService_CreateRejectsNegativeBalance(t *testing.T) {
	f := newFakeStore()
	err := NewAccountService(nil, f).Create(context.Background(), 1, dec("-1"))
	if !errors.Is(err, domain.ErrInvalidAmount) {
		t.Fatalf("err = %v, want ErrInvalidAmount", err)
	}
}

func TestAccountService_CreateAndGet(t *testing.T) {
	f := newFakeStore()
	svc := NewAccountService(nil, f)
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
