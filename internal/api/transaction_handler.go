package api

import (
	"context"
	"time"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol/consts"

	"github.com/joeyyang/transfer-demo/internal/observability"
	"github.com/joeyyang/transfer-demo/internal/service"
)

// idempotencyHeader is the HTTP header carrying the optional idempotency key.
// idempotencyHeader 是承载可选幂等键的 HTTP 头。
const idempotencyHeader = "Idempotency-Key"

// TransactionHandler serves the transfer endpoint.
// TransactionHandler 提供转账接口。
type TransactionHandler struct {
	svc     *service.TransferService
	resp    *Responder
	metrics *observability.Metrics
}

// NewTransactionHandler creates a TransactionHandler.
// NewTransactionHandler 创建一个 TransactionHandler。
func NewTransactionHandler(svc *service.TransferService, resp *Responder, m *observability.Metrics) *TransactionHandler {
	return &TransactionHandler{svc: svc, resp: resp, metrics: m}
}

// Transfer handles POST /transactions.
// Transfer 处理 POST /transactions。
func (h *TransactionHandler) Transfer(ctx context.Context, c *app.RequestContext) {
	var req TransferRequest
	if err := c.BindJSON(&req); err != nil {
		h.resp.BadRequest(c, "invalid_request", "invalid request body")
		return
	}

	// Optional idempotency key from the header; empty means non-idempotent.
	// 从请求头取可选幂等键；为空表示非幂等。
	key := string(c.GetHeader(idempotencyHeader))
	if key != "" {
		h.metrics.IdempotencyHits.WithLabelValues("with_key").Inc()
	} else {
		h.metrics.IdempotencyHits.WithLabelValues("no_key").Inc()
	}

	start := time.Now()
	transfer, err := h.svc.Transfer(ctx, service.TransferCmd{
		SourceID:       req.SourceAccountID,
		DestinationID:  req.DestinationAccountID,
		Amount:         req.Amount,
		IdempotencyKey: key,
	})
	h.metrics.TransferLatency.Observe(time.Since(start).Seconds())
	if err != nil {
		h.metrics.Transfers.WithLabelValues("failed").Inc()
		h.resp.Error(ctx, c, "service", err)
		return
	}
	h.metrics.Transfers.WithLabelValues("completed").Inc()
	c.JSON(consts.StatusOK, TransactionResponse{
		TransactionID:        transfer.ID,
		SourceAccountID:      transfer.SourceID,
		DestinationAccountID: transfer.DestinationID,
		Amount:               transfer.Amount,
		Status:               transfer.Status,
	})
}
