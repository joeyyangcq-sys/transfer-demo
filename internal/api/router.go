package api

import (
	"log/slog"

	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/cloudwego/hertz/pkg/common/adaptor"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/joeyyang/transfer-demo/internal/observability"
)

// Handlers bundles the HTTP handlers needed to register routes.
// Handlers 汇总注册路由所需的各个 HTTP handler。
type Handlers struct {
	Account     *AccountHandler
	Transaction *TransactionHandler
	Health      *HealthHandler
}

// RegisterPublicRoutes mounts middleware and the business endpoints on the
// public server. Operational endpoints are NOT exposed here.
// RegisterPublicRoutes 在公开 server 上挂载中间件和业务接口。
// 运维类接口不在此暴露。
func RegisterPublicRoutes(h *server.Hertz, m *observability.Metrics, log *slog.Logger, handlers Handlers) {
	// Global middleware: recover first, then request id, then observability.
	// 全局中间件：先 recover，再注入 request id，最后做可观测性记录。
	h.Use(Recover(m, log), RequestID(), Observe(m, log))

	h.POST("/accounts", handlers.Account.Create)
	h.GET("/accounts/:account_id", handlers.Account.Get)
	h.POST("/transactions", handlers.Transaction.Transfer)
}

// RegisterAdminRoutes mounts metrics and health probes on the internal admin
// server. This port should not be exposed to the public; metrics reveal
// internal state and are scraped from within the cluster/VPC only.
// RegisterAdminRoutes 在内部 admin server 上挂载指标与健康探针。
// 该端口不应对外暴露；指标会泄露内部状态，只在集群/VPC 内部被抓取。
func RegisterAdminRoutes(h *server.Hertz, reg *prometheus.Registry, m *observability.Metrics, log *slog.Logger, handlers Handlers) {
	h.Use(Recover(m, log))

	h.GET("/livez", handlers.Health.Livez)
	h.GET("/readyz", handlers.Health.Readyz)
	h.GET("/metrics", adaptor.HertzHandler(promhttp.HandlerFor(reg, promhttp.HandlerOpts{})))
}
