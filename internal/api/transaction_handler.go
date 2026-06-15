package api

import (
	"context"
	"time"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol/consts"

	"github.com/joeyyang/internal-transfers/internal/observability"
	"github.com/joeyyang/internal-transfers/internal/service"
)

// idempotencyHeader is the HTTP header carrying the optional idempotency key.
const idempotencyHeader = "Idempotency-Key"

// TransactionHandler serves the transfer endpoint.
type TransactionHandler struct {
	svc     *service.TransferService
	resp    *Responder
	metrics *observability.Metrics
}

// NewTransactionHandler creates a TransactionHandler.
func NewTransactionHandler(svc *service.TransferService, resp *Responder, m *observability.Metrics) *TransactionHandler {
	return &TransactionHandler{svc: svc, resp: resp, metrics: m}
}

// Transfer handles POST /transactions.
func (h *TransactionHandler) Transfer(ctx context.Context, c *app.RequestContext) {
	var req TransferRequest
	if err := c.BindJSON(&req); err != nil {
		h.resp.BadRequest(c, "invalid_request", "invalid request body")
		return
	}

	key := string(c.GetHeader(idempotencyHeader))
	if key != "" {
		h.metrics.IdempotencyHits.WithLabelValues("with_key").Inc()
	} else {
		h.metrics.IdempotencyHits.WithLabelValues("no_key").Inc()
	}

	start := time.Now()
	_, err := h.svc.Transfer(ctx, service.TransferCmd{
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
	c.Status(consts.StatusCreated)
}
