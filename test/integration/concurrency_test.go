//go:build integration

package integration

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/joeyyang/transfer-demo/internal/domain"
	"github.com/joeyyang/transfer-demo/internal/service"
)

// TestTransfer_ConcurrentNoOverdraft fires more transfers than the balance can
// cover and asserts the account is never overdrawn and money is conserved.
// TestTransfer_ConcurrentNoOverdraft 并发发起超过余额可覆盖的转账，
// 断言账户永不透支且资金守恒。
func TestTransfer_ConcurrentNoOverdraft(t *testing.T) {
	e := setup(t)
	ctx := context.Background()
	_ = e.accountSvc.Create(ctx, 1, dec("1000"))
	_ = e.accountSvc.Create(ctx, 2, dec("0"))

	const workers = 200 // 200 x 10 = 2000 requested, only 1000 available — 请求 2000，仅有 1000 可用
	var wg sync.WaitGroup
	var mu sync.Mutex
	var success, insufficient int

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := e.transferSvc.Transfer(ctx, service.TransferCmd{SourceID: 1, DestinationID: 2, Amount: dec("10")})
			mu.Lock()
			defer mu.Unlock()
			switch {
			case err == nil:
				success++
			case errors.Is(err, domain.ErrInsufficientFunds):
				insufficient++
			default:
				t.Errorf("unexpected error: %v", err)
			}
		}()
	}
	wg.Wait()

	a1, _ := e.accountSvc.Get(ctx, 1)
	a2, _ := e.accountSvc.Get(ctx, 2)

	if a1.Balance.Sign() < 0 {
		t.Fatalf("account 1 overdrawn: %s", a1.Balance)
	}
	// Conservation: total stays 1000.
	// 守恒：两账户总额始终为 1000。
	if !a1.Balance.Add(a2.Balance).Equal(dec("1000")) {
		t.Fatalf("money not conserved: %s + %s != 1000", a1.Balance, a2.Balance)
	}
	if success != 100 || insufficient != 100 {
		t.Fatalf("success=%d insufficient=%d, want 100/100", success, insufficient)
	}

	// Macro Conservation: Total balances MUST exactly match total deposits across the system.
	// 宏观守恒：系统中所有账户的总余额，必须严格等于所有开户注资的总和。
	var totalBalances, totalDeposits string
	err := e.pool.QueryRow(ctx, `
		SELECT 
			COALESCE(SUM(balance), 0)::text AS total_balances,
			COALESCE((SELECT SUM(amount) FROM deposits), 0)::text AS total_deposits
		FROM accounts
	`).Scan(&totalBalances, &totalDeposits)
	if err != nil {
		t.Fatalf("audit query failed: %v", err)
	}
	if totalBalances != totalDeposits {
		t.Fatalf("SYSTEM NOT CONSERVED: total balances (%s) != total deposits (%s)", totalBalances, totalDeposits)
	}
}
