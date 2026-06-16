package postgres

import (
	"context"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

// QueryObserver receives the latency of each executed query, labelled by a
// low-cardinality operation name.
// QueryObserver 接收每条查询的耗时，并带上低基数的操作名标签。
type QueryObserver func(operation string, seconds float64)

// queryTracer implements pgx.QueryTracer to time every query on the pool.
// queryTracer 实现 pgx.QueryTracer，为连接池上每条查询计时。
type queryTracer struct {
	observe QueryObserver
}

// NewQueryTracer returns a pgx tracer that reports query latency to observe.
// NewQueryTracer 返回一个把查询耗时上报给 observe 的 pgx tracer。
func NewQueryTracer(observe QueryObserver) pgx.QueryTracer {
	return &queryTracer{observe: observe}
}

// traceData carries per-query state from start to end (the SQL is only
// available on the start hook).
// traceData 把每条查询从 start 到 end 的状态带过去（SQL 只在 start 钩子可拿到）。
type traceData struct {
	start     time.Time
	operation string
}

type traceKey struct{}

func (t *queryTracer) TraceQueryStart(ctx context.Context, _ *pgx.Conn, data pgx.TraceQueryStartData) context.Context {
	return context.WithValue(ctx, traceKey{}, traceData{start: time.Now(), operation: operationName(data.SQL)})
}

func (t *queryTracer) TraceQueryEnd(ctx context.Context, _ *pgx.Conn, _ pgx.TraceQueryEndData) {
	td, ok := ctx.Value(traceKey{}).(traceData)
	if !ok {
		return
	}
	t.observe(td.operation, time.Since(td.start).Seconds())
}

// operationName derives a stable, low-cardinality label from a SQL statement,
// e.g. "select accounts (locked)", "update accounts", "insert transfers".
// operationName 从 SQL 派生出稳定、低基数的标签，
// 如 "select accounts (locked)"、"update accounts"、"insert transfers"。
func operationName(sql string) string {
	fields := strings.Fields(strings.ToLower(sql))
	if len(fields) == 0 {
		return "unknown"
	}
	verb := fields[0]

	var table string
	switch verb {
	case "select", "delete":
		table = tokenAfter(fields, "from")
	case "insert":
		table = tokenAfter(fields, "into")
	case "update":
		table = clean(fields[1])
	}

	op := verb
	if table != "" {
		op = verb + " " + table
	}
	// Flag row-locking reads — the ones most prone to contention.
	// 标记加锁读——最容易发生竞争的查询。
	if strings.Contains(strings.ToLower(sql), "for update") {
		op += " (locked)"
	}
	return op
}

// tokenAfter returns the cleaned token following keyword, or "".
// tokenAfter 返回 keyword 之后的（清洗过的）token，没有则返回 ""。
func tokenAfter(fields []string, keyword string) string {
	for i, f := range fields {
		if f == keyword && i+1 < len(fields) {
			return clean(fields[i+1])
		}
	}
	return ""
}

// clean strips punctuation that may cling to an identifier (e.g. "accounts(").
// clean 去掉黏在标识符上的标点（如 "accounts("）。
func clean(tok string) string {
	return strings.Trim(tok, "(),;")
}
