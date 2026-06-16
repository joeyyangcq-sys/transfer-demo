//go:build integration

package integration

import (
	"context"
	"sync"
	"testing"

	"github.com/joeyyang/transfer-demo/internal/service"
)

// TestTransfer_IdempotentRetry sends the same Idempotency-Key several times,
// including concurrently, and asserts money moves exactly once.
// TestTransfer_IdempotentRetry 多次（含并发）发送相同 Idempotency-Key，
// 断言资金恰好只移动一次。
func TestTransfer_IdempotentRetry(t *testing.T) {
	e := setup(t)
	ctx := context.Background()
	_ = e.accountSvc.Create(ctx, 1, dec("100"))
	_ = e.accountSvc.Create(ctx, 2, dec("0"))

	key := "550e8400-e29b-41d4-a716-446655440000"
	cmd := service.TransferCmd{SourceID: 1, DestinationID: 2, Amount: dec("30"), IdempotencyKey: key}

	const tries = 10
	var wg sync.WaitGroup
	ids := make([]int64, tries)
	for i := 0; i < tries; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			tr, err := e.transferSvc.Transfer(ctx, cmd)
			if err != nil {
				t.Errorf("transfer: %v", err)
				return
			}
			ids[idx] = tr.ID
		}(i)
	}
	wg.Wait()

	// All retries resolve to the same transfer id.
	// 所有重试都解析到同一个转账 id。
	for _, id := range ids {
		if id != ids[0] {
			t.Fatalf("retries returned different transfer ids: %v", ids)
		}
	}

	a1, _ := e.accountSvc.Get(ctx, 1)
	a2, _ := e.accountSvc.Get(ctx, 2)
	if !a1.Balance.Equal(dec("70")) || !a2.Balance.Equal(dec("30")) {
		t.Fatalf("money moved more than once: a1=%s a2=%s", a1.Balance, a2.Balance)
	}
}
