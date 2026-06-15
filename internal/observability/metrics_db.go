package observability

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// SamplePool periodically copies pool stats into gauges until ctx is done.
func (m *Metrics) SamplePool(ctx context.Context, pool *pgxpool.Pool, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			stat := pool.Stat()
			m.DBTotalConns.Set(float64(stat.TotalConns()))
			m.DBIdleConns.Set(float64(stat.IdleConns()))
			m.DBAcquiredConns.Set(float64(stat.AcquiredConns()))
		}
	}
}
