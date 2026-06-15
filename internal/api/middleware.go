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
const requestIDHeader = "X-Request-Id"

// skipObservePaths are operational endpoints excluded from access logs and
// HTTP metrics to avoid noise.
var skipObservePaths = map[string]bool{
	"/livez":   true,
	"/readyz":  true,
	"/metrics": true,
}

// RequestID assigns a request id (from the header or a new UUID) and echoes it.
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

// Observe records HTTP RED metrics and an access log line, skipping probes.
func Observe(m *observability.Metrics, log *slog.Logger) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		path := string(c.Path())
		if skipObservePaths[path] {
			c.Next(ctx)
			return
		}

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
