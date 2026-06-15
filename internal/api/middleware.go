package api

import (
	"context"
	"log/slog"
	"time"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
	"github.com/google/uuid"

	"github.com/joeyyang/internal-transfers/internal/observability"
)

// requestIDHeader carries a per-request correlation id.
// requestIDHeader 承载每个请求的关联 id。
const requestIDHeader = "X-Request-Id"

// RequestID assigns a request id (from the header or a new UUID) and echoes it.
// RequestID 为请求分配 id（取自请求头或新生成的 UUID）并回写到响应头。
func RequestID() app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		id := string(c.GetHeader(requestIDHeader))
		if id == "" {
			id = uuid.NewString()
		}
		c.Set("request_id", id)
		c.Header(requestIDHeader, id)
		c.Next(ctx)
	}
}

// Observe records HTTP RED metrics and an access log line for each request.
// Observe 为每个请求记录 HTTP RED 指标和一条访问日志。
func Observe(m *observability.Metrics, log *slog.Logger) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		path := string(c.Path())

		start := time.Now()
		c.Next(ctx)
		elapsed := time.Since(start)

		route := c.FullPath()
		if route == "" {
			route = "unmatched"
		}
		method := string(c.Method())
		status := strconvStatus(c.Response.StatusCode())

		m.HTTPRequests.WithLabelValues(route, method, status).Inc()
		m.HTTPDuration.WithLabelValues(route, method).Observe(elapsed.Seconds())

		log.Info("request",
			"request_id", c.GetString("request_id"),
			"method", method,
			"path", path,
			"status", c.Response.StatusCode(),
			"duration_ms", elapsed.Milliseconds(),
		)
	}
}

// Recover turns panics into a 500 and records the panic metric.
// Recover 把 panic 转为 500 响应并累加 panic 指标。
func Recover(m *observability.Metrics, log *slog.Logger) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		defer func() {
			if r := recover(); r != nil {
				m.Errors.WithLabelValues("panic_recovered", "middleware").Inc()
				log.Error("panic recovered", "request_id", c.GetString("request_id"), "panic", r)
				c.AbortWithStatusJSON(consts.StatusInternalServerError, ErrorResponse{Error: "internal server error"})
			}
		}()
		c.Next(ctx)
	}
}

// strconvStatus buckets a status code into a low-cardinality label.
// strconvStatus 把状态码归并为低基数标签。
func strconvStatus(code int) string {
	switch {
	case code >= 500:
		return "5xx"
	case code >= 400:
		return "4xx"
	case code >= 300:
		return "3xx"
	case code >= 200:
		return "2xx"
	default:
		return "other"
	}
}
