package api

import (
	"context"
	"sync/atomic"
	"time"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
	"github.com/jackc/pgx/v5/pgxpool"
)

// HealthHandler serves liveness and readiness probes.
type HealthHandler struct {
	pool         *pgxpool.Pool
	shuttingDown atomic.Bool
}

// NewHealthHandler creates a HealthHandler.
func NewHealthHandler(pool *pgxpool.Pool) *HealthHandler {
	return &HealthHandler{pool: pool}
}

// SetNotReady marks the service as draining so readiness returns 503 and the
// load balancer stops sending new traffic before shutdown.
func (h *HealthHandler) SetNotReady() {
	h.shuttingDown.Store(true)
}

// Livez reports process liveness. No dependency checks, so a DB outage does
// not trigger pod restarts.
func (h *HealthHandler) Livez(_ context.Context, c *app.RequestContext) {
	c.JSON(consts.StatusOK, map[string]string{"status": "ok"})
}

// Readyz reports readiness to serve traffic: the database must be reachable.
func (h *HealthHandler) Readyz(ctx context.Context, c *app.RequestContext) {
	if h.shuttingDown.Load() {
		c.JSON(consts.StatusServiceUnavailable, ReadyResponse{Status: "draining"})
		return
	}

	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	if err := h.pool.Ping(ctx); err != nil {
		c.JSON(consts.StatusServiceUnavailable, ReadyResponse{
			Status: "unavailable",
			Checks: map[string]string{"postgres": "down"},
		})
		return
	}
	c.JSON(consts.StatusOK, ReadyResponse{
		Status: "ready",
		Checks: map[string]string{"postgres": "ok"},
	})
}
