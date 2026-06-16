//go:build integration

package integration

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/cloudwego/hertz/pkg/common/ut"
)

// logLine is one parsed JSON log record.
// logLine 是一条解析后的 JSON 日志记录。
type logLine map[string]any

// readLogs parses the captured log buffer into records.
// readLogs 把捕获的日志缓冲区解析为记录。
func readLogs(t *testing.T, e *env) []logLine {
	t.Helper()
	var out []logLine
	for _, raw := range strings.Split(strings.TrimSpace(e.logs.String()), "\n") {
		if raw == "" {
			continue
		}
		var m logLine
		if err := json.Unmarshal([]byte(raw), &m); err != nil {
			t.Fatalf("bad log line %q: %v", raw, err)
		}
		out = append(out, m)
	}
	return out
}

// TestErrorLogs_ClientErrorsNotErrorLogged drives a 4xx from each layer and
// confirms the single-handling contract: client errors are recorded in the
// access log (and metrics) but never produce an ERROR-level log.
// 驱动各层的 4xx，验证单一处理契约：客户端错误进访问日志（与指标），但绝不产生 ERROR 级日志。
func TestErrorLogs_ClientErrorsNotErrorLogged(t *testing.T) {
	e := setup(t)
	mustCreate(t, e, `{"account_id":1,"initial_balance":"5"}`)
	mustCreate(t, e, `{"account_id":2,"initial_balance":"0"}`)

	key := ut.Header{Key: "Idempotency-Key", Value: "550e8400-e29b-41d4-a716-446655440000"}

	cases := []struct {
		name   string
		method string
		path   string
		body   string
		hdr    []ut.Header
		want   int
	}{
		{"handler: malformed json", "POST", "/accounts", `{bad`, []ut.Header{jsonHeader}, 400},
		{"handler: bad account id", "GET", "/accounts/not-a-number", "", nil, 400},
		{"service: invalid amount", "POST", "/transactions", `{"source_account_id":1,"destination_account_id":2,"amount":"0"}`, []ut.Header{jsonHeader}, 400},
		{"service: same account", "POST", "/transactions", `{"source_account_id":1,"destination_account_id":1,"amount":"1"}`, []ut.Header{jsonHeader}, 400},
		{"handler: bad idempotency key", "POST", "/transactions", `{"source_account_id":1,"destination_account_id":2,"amount":"1"}`, []ut.Header{jsonHeader, {Key: "Idempotency-Key", Value: "not-a-uuid"}}, 400},
		{"service: account not found", "GET", "/accounts/999", "", nil, 404},
		{"service: account exists", "POST", "/accounts", `{"account_id":1,"initial_balance":"5"}`, []ut.Header{jsonHeader}, 409},
		{"service: insufficient funds", "POST", "/transactions", `{"source_account_id":1,"destination_account_id":2,"amount":"9999"}`, []ut.Header{jsonHeader}, 409},
	}
	for _, tc := range cases {
		var body *ut.Body
		if tc.body != "" {
			body = jsonBody(tc.body)
		}
		w := ut.PerformRequest(e.engine, tc.method, tc.path, body, tc.hdr...)
		if got := w.Result().StatusCode(); got != tc.want {
			t.Errorf("%s: status = %d, want %d", tc.name, got, tc.want)
		}
	}

	// 422: same idempotency key, different amount.
	// 422：相同幂等键、不同金额。
	_ = ut.PerformRequest(e.engine, "POST", "/transactions", jsonBody(`{"source_account_id":1,"destination_account_id":2,"amount":"1"}`), jsonHeader, key)
	w := ut.PerformRequest(e.engine, "POST", "/transactions", jsonBody(`{"source_account_id":1,"destination_account_id":2,"amount":"2"}`), jsonHeader, key)
	if got := w.Result().StatusCode(); got != 422 {
		t.Errorf("idempotency conflict: status = %d, want 422", got)
	}

	// Assert no ERROR-level "request failed" was emitted for any 4xx, and show
	// the captured access logs so the contract is visible.
	// 断言任何 4xx 都没产生 ERROR 级 "request failed"，并打印捕获的访问日志，使契约可见。
	for _, l := range readLogs(t, e) {
		if l["level"] == "ERROR" && l["msg"] == "request failed" {
			t.Errorf("4xx must not be error-logged, got: %v", l)
		}
		t.Logf("%s %v %v -> %v", l["level"], l["method"], l["path"], l["status"])
	}
}

// TestErrorLogs_ServerErrorIsErrorLogged forces a 5xx (closed pool) and shows
// the single, correlated ERROR log produced at the boundary.
// 制造一个 5xx（关闭连接池），展示边界处产生的那条唯一且带关联字段的 ERROR 日志。
func TestErrorLogs_ServerErrorIsErrorLogged(t *testing.T) {
	e := setup(t)

	// Take the database away, then make a request that must hit it.
	// 把数据库撤掉，再发一个必然访问数据库的请求。
	e.pool.Close()

	w := ut.PerformRequest(e.engine, "GET", "/accounts/1", nil)
	if got := w.Result().StatusCode(); got != 500 {
		t.Fatalf("status = %d, want 500", got)
	}

	var failed logLine
	for _, l := range readLogs(t, e) {
		if l["level"] == "ERROR" && l["msg"] == "request failed" {
			failed = l
		}
	}
	if failed == nil {
		t.Fatal("expected one ERROR 'request failed' log for the 5xx")
	}

	// The boundary log must carry correlation + classification fields.
	// 边界日志必须带有关联与分类字段。
	for _, field := range []string{"request_id", "method", "path", "type", "layer", "error"} {
		if _, ok := failed[field]; !ok {
			t.Errorf("error log missing %q: %v", field, failed)
		}
	}
	if failed["type"] != "internal" {
		t.Errorf("type = %v, want internal", failed["type"])
	}

	pretty, _ := json.MarshalIndent(failed, "", "  ")
	t.Logf("5xx boundary error log:\n%s", pretty)
}
