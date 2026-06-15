package api

import (
	"context"
	"errors"
	"log/slog"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol/consts"

	"github.com/joeyyang/internal-transfers/internal/domain"
	"github.com/joeyyang/internal-transfers/internal/observability"
)

// Responder writes JSON responses and maps domain errors to HTTP codes,
// recording each error in the metrics error classifier.
type Responder struct {
	metrics *observability.Metrics
	log     *slog.Logger
}

// NewResponder creates a Responder.
func NewResponder(m *observability.Metrics, log *slog.Logger) *Responder {
	return &Responder{metrics: m, log: log}
}

// errorMapping maps a domain error to an HTTP status and a metric type label.
func errorMapping(err error) (status int, errType string) {
	switch {
	case errors.Is(err, domain.ErrInvalidAmount):
		return consts.StatusBadRequest, "invalid_amount"
	case errors.Is(err, domain.ErrSameAccount):
		return consts.StatusBadRequest, "same_account"
	case errors.Is(err, domain.ErrInvalidIdempotency):
		return consts.StatusBadRequest, "invalid_idempotency_key"
	case errors.Is(err, domain.ErrAccountNotFound):
		return consts.StatusNotFound, "account_not_found"
	case errors.Is(err, domain.ErrAccountAlreadyExists):
		return consts.StatusConflict, "account_already_exists"
	case errors.Is(err, domain.ErrInsufficientFunds):
		return consts.StatusConflict, "insufficient_funds"
	case errors.Is(err, domain.ErrIdempotencyConflict):
		return consts.StatusUnprocessableEntity, "idempotency_conflict"
	default:
		return consts.StatusInternalServerError, "internal"
	}
}

// Error writes an error response and records the error metric.
// layer identifies where the error was caught (e.g. "handler", "service").
func (r *Responder) Error(_ context.Context, c *app.RequestContext, layer string, err error) {
	status, errType := errorMapping(err)
	r.metrics.Errors.WithLabelValues(errType, layer).Inc()

	// Log full detail for 5xx; client sees a generic message.
	msg := err.Error()
	if status >= consts.StatusInternalServerError {
		r.log.Error("request failed", "type", errType, "layer", layer, "error", err)
		msg = "internal server error"
	}
	c.JSON(status, ErrorResponse{Error: msg})
}

// BadRequest writes a 400 with a fixed message and records the metric.
func (r *Responder) BadRequest(c *app.RequestContext, errType, msg string) {
	r.metrics.Errors.WithLabelValues(errType, "handler").Inc()
	c.JSON(consts.StatusBadRequest, ErrorResponse{Error: msg})
}
