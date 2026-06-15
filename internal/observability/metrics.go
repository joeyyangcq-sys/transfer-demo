package observability

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// Metrics holds the application's Prometheus collectors.
type Metrics struct {
	// HTTP RED metrics.
	HTTPRequests *prometheus.CounterVec
	HTTPDuration *prometheus.HistogramVec

	// Business metrics.
	Transfers       *prometheus.CounterVec // by status
	TransferLatency prometheus.Histogram
	AccountsCreated *prometheus.CounterVec // by result
	IdempotencyHits *prometheus.CounterVec // by result (hit/miss)

	// Error classifier: every caught error increments this.
	Errors *prometheus.CounterVec // by type, layer

	// DB pool gauges.
	DBTotalConns    prometheus.Gauge
	DBIdleConns     prometheus.Gauge
	DBAcquiredConns prometheus.Gauge
}

// NewMetrics registers and returns the application metrics.
func NewMetrics(reg prometheus.Registerer) *Metrics {
	f := promauto.With(reg)
	return &Metrics{
		HTTPRequests: f.NewCounterVec(prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total HTTP requests.",
		}, []string{"route", "method", "status"}),
		HTTPDuration: f.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "HTTP request latency.",
			Buckets: prometheus.DefBuckets,
		}, []string{"route", "method"}),
		Transfers: f.NewCounterVec(prometheus.CounterOpts{
			Name: "transfers_total",
			Help: "Total transfers by status.",
		}, []string{"status"}),
		TransferLatency: f.NewHistogram(prometheus.HistogramOpts{
			Name:    "transfer_processing_duration_seconds",
			Help:    "Transfer processing latency, including lock wait.",
			Buckets: prometheus.DefBuckets,
		}),
		AccountsCreated: f.NewCounterVec(prometheus.CounterOpts{
			Name: "accounts_created_total",
			Help: "Total account creation attempts by result.",
		}, []string{"result"}),
		IdempotencyHits: f.NewCounterVec(prometheus.CounterOpts{
			Name: "idempotency_requests_total",
			Help: "Transfer requests by idempotency outcome.",
		}, []string{"result"}),
		Errors: f.NewCounterVec(prometheus.CounterOpts{
			Name: "transfers_errors_total",
			Help: "Caught errors by type and layer.",
		}, []string{"type", "layer"}),
		DBTotalConns: f.NewGauge(prometheus.GaugeOpts{
			Name: "db_pool_total_conns",
			Help: "Total connections in the pool.",
		}),
		DBIdleConns: f.NewGauge(prometheus.GaugeOpts{
			Name: "db_pool_idle_conns",
			Help: "Idle connections in the pool.",
		}),
		DBAcquiredConns: f.NewGauge(prometheus.GaugeOpts{
			Name: "db_pool_acquired_conns",
			Help: "Acquired (in-use) connections in the pool.",
		}),
	}
}
