package observability

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// Metrics holds the application's Prometheus collectors.
type Metrics struct {
	// HTTP RED metrics.
	// HTTP RED 指标（请求量/错误/延迟）。
	HTTPRequests *prometheus.CounterVec
	HTTPDuration *prometheus.HistogramVec

	// Business metrics.
	// 业务指标。
	Transfers       *prometheus.CounterVec // by status — 按状态
	TransferLatency prometheus.Histogram
	AccountsCreated *prometheus.CounterVec // by result — 按结果
	IdempotencyHits *prometheus.CounterVec // by result (hit/miss) — 按结果（命中/未命中）

	// Error classifier: every caught error increments this.
	// 错误分类器：每个被捕获的错误都会在此累加。
	Errors *prometheus.CounterVec // by type, layer — 按类型、层

	// DB pool gauges.
	// 数据库连接池 gauge。
	DBTotalConns    prometheus.Gauge
	DBIdleConns     prometheus.Gauge
	DBAcquiredConns prometheus.Gauge

	// Per-query latency, for spotting slow queries.
	// 单条查询耗时，用于定位慢查询。
	DBQueryDuration *prometheus.HistogramVec // by operation — 按操作
}

// NewMetrics registers and returns the application metrics.
// NewMetrics 注册并返回应用指标集合。
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
		DBQueryDuration: f.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "db_query_duration_seconds",
			Help:    "Database query latency by operation.",
			Buckets: []float64{.0005, .001, .0025, .005, .01, .025, .05, .1, .25, .5, 1, 2.5},
		}, []string{"operation"}),
	}
}
