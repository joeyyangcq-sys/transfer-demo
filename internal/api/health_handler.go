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
// HealthHandler 提供存活与就绪探针。
type HealthHandler struct {
	pool         *pgxpool.Pool
	shuttingDown atomic.Bool
}

// NewHealthHandler creates a HealthHandler.
// NewHealthHandler 创建一个 HealthHandler。
func NewHealthHandler(pool *pgxpool.Pool) *HealthHandler {
	return &HealthHandler{pool: pool}
}

// SetNotReady marks the service as draining so readiness returns 503 and the
// load balancer stops sending new traffic before shutdown.
// SetNotReady 标记服务进入排空状态，使就绪探针返回 503，
// 让负载均衡在关闭前停止转发新流量。
func (h *HealthHandler) SetNotReady() {
	h.shuttingDown.Store(true)
}

// Livez reports process liveness. No dependency checks, so a DB outage does
// not trigger pod restarts.
// Livez 报告进程存活。不检查依赖，因此数据库故障不会触发 Pod 重启。
func (h *HealthHandler) Livez(_ context.Context, c *app.RequestContext) {
	c.JSON(consts.StatusOK, map[string]string{"status": "ok"})
}

// Readyz reports readiness to serve traffic: the database must be reachable.
// Readyz 报告是否可承接流量：数据库必须可达。
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
