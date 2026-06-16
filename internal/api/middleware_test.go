package api

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/cloudwego/hertz/pkg/common/ut"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"

	"github.com/joeyyang/internal-transfers/internal/observability"
)

// TestRecover_TurnsPanicInto500 checks the panic path: a handler panic becomes
// a 500, increments the panic_recovered error metric, and is logged once.
// TestRecover_TurnsPanicInto500 校验 panic 路径：handler panic 转为 500、
// 累加 panic_recovered 指标，并被记录一次。
func TestRecover_TurnsPanicInto500(t *testing.T) {
	logs := &bytes.Buffer{}
	logger := slog.New(slog.NewJSONHandler(logs, &slog.HandlerOptions{Level: slog.LevelDebug}))
	metrics := observability.NewMetrics(prometheus.NewRegistry())

	h := server.New()
	h.Use(Recover(metrics, logger), RequestID())
	h.GET("/boom", func(_ context.Context, _ *app.RequestContext) {
		panic("boom")
	})

	w := ut.PerformRequest(h.Engine, "GET", "/boom", nil)

	if got := w.Result().StatusCode(); got != 500 {
		t.Fatalf("status = %d, want 500", got)
	}
	if got := testutil.ToFloat64(metrics.Errors.WithLabelValues("panic_recovered", "middleware")); got != 1 {
		t.Errorf("panic_recovered metric = %v, want 1", got)
	}
	if !strings.Contains(logs.String(), `"msg":"panic recovered"`) {
		t.Errorf("expected a 'panic recovered' log, got: %s", logs.String())
	}
}
