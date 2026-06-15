//go:build integration

package integration

import (
	"context"
	"errors"
	"testing"

	"github.com/shopspring/decimal"

	"github.com/joeyyang/internal-transfers/internal/domain"
	"github.com/joeyyang/internal-transfers/internal/service"
)

func dec(s string) decimal.Decimal { return decimal.RequireFromString(s) }

func TestTransfer_EndToEnd(t *testing.T) {
	e := setup(t)
	ctx := context.Background()

	if err := e.accountSvc.Create(ctx, 1, dec("100.23344")); err != nil {
		t.Fatalf("create account 1: %v", err)
	}
	if err := e.accountSvc.Create(ctx, 2, dec("0")); err != nil {
		t.Fatalf("create account 2: %v", err)
	}

	if _, err := e.transferSvc.Transfer(ctx, service.TransferCmd{SourceID: 1, DestinationID: 2, Amount: dec("100.12345")}); err != nil {
		t.Fatalf("transfer: %v", err)
	}

	a1, _ := e.accountSvc.Get(ctx, 1)
	a2, _ := e.accountSvc.Get(ctx, 2)
	if !a1.Balance.Equal(dec("0.10999")) {
		t.Errorf("account 1 balance = %s, want 0.10999", a1.Balance)
	}
	if !a2.Balance.Equal(dec("100.12345")) {
		t.Errorf("account 2 balance = %s, want 100.12345", a2.Balance)
	}
}

func TestTransfer_InsufficientFunds(t *testing.T) {
	e := setup(t)
	ctx := context.Background()
	_ = e.accountSvc.Create(ctx, 1, dec("5"))
	_ = e.accountSvc.Create(ctx, 2, dec("0"))

	_, err := e.transferSvc.Transfer(ctx, service.TransferCmd{SourceID: 1, DestinationID: 2, Amount: dec("10")})
	if !errors.Is(err, domain.ErrInsufficientFunds) {
		t.Fatalf("err = %v, want ErrInsufficientFunds", err)
	}
}

func TestTransfer_AccountNotFound(t *testing.T) {
	e := setup(t)
	ctx := context.Background()
	_ = e.accountSvc.Create(ctx, 1, dec("100"))

	_, err := e.transferSvc.Transfer(ctx, service.TransferCmd{SourceID: 1, DestinationID: 999, Amount: dec("10")})
	if !errors.Is(err, domain.ErrAccountNotFound) {
		t.Fatalf("err = %v, want ErrAccountNotFound", err)
	}
}

func TestAccount_DuplicateCreate(t *testing.T) {
	e := setup(t)
	ctx := context.Background()
	if err := e.accountSvc.Create(ctx, 1, dec("100")); err != nil {
		t.Fatalf("first create: %v", err)
	}
	if err := e.accountSvc.Create(ctx, 1, dec("50")); !errors.Is(err, domain.ErrAccountAlreadyExists) {
		t.Fatalf("err = %v, want ErrAccountAlreadyExists", err)
	}
}
