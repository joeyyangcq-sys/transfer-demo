package api

import (
	"log/slog"

	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/cloudwego/hertz/pkg/common/adaptor"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/joeyyang/internal-transfers/internal/observability"
)

// Handlers bundles the HTTP handlers needed to register routes.
type Handlers struct {
	Account     *AccountHandler
	Transaction *TransactionHandler
	Health      *HealthHandler
}

// RegisterRoutes mounts middleware and all routes on the Hertz server.
func RegisterRoutes(h *server.Hertz, reg *prometheus.Registry, m *observability.Metrics, log *slog.Logger, handlers Handlers) {
	// Global middleware: recover first, then request id, then observability.
	h.Use(Recover(m, log), RequestID(), Observe(m, log))

	h.POST("/accounts", handlers.Account.Create)
	h.GET("/accounts/:account_id", handlers.Account.Get)
	h.POST("/transactions", handlers.Transaction.Transfer)

	h.GET("/livez", handlers.Health.Livez)
	h.GET("/readyz", handlers.Health.Readyz)
	h.GET("/metrics", adaptor.HertzHandler(promhttp.HandlerFor(reg, promhttp.HandlerOpts{})))
}
