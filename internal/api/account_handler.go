package api

import (
	"context"
	"strconv"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol/consts"

	"github.com/joeyyang/internal-transfers/internal/observability"
	"github.com/joeyyang/internal-transfers/internal/service"
)

// AccountHandler serves account endpoints.
// AccountHandler 提供账户相关接口。
type AccountHandler struct {
	svc     *service.AccountService
	resp    *Responder
	metrics *observability.Metrics
}

// NewAccountHandler creates an AccountHandler.
// NewAccountHandler 创建一个 AccountHandler。
func NewAccountHandler(svc *service.AccountService, resp *Responder, m *observability.Metrics) *AccountHandler {
	return &AccountHandler{svc: svc, resp: resp, metrics: m}
}

// Create handles POST /accounts.
// Create 处理 POST /accounts。
func (h *AccountHandler) Create(ctx context.Context, c *app.RequestContext) {
	var req CreateAccountRequest
	if err := c.BindJSON(&req); err != nil {
		h.resp.BadRequest(c, "invalid_request", "invalid request body")
		return
	}

	if err := h.svc.Create(ctx, req.AccountID, req.InitialBalance); err != nil {
		h.metrics.AccountsCreated.WithLabelValues("error").Inc()
		h.resp.Error(ctx, c, "service", err)
		return
	}
	h.metrics.AccountsCreated.WithLabelValues("ok").Inc()
	c.Status(consts.StatusCreated)
}

// Get handles GET /accounts/:account_id.
// Get 处理 GET /accounts/:account_id。
func (h *AccountHandler) Get(ctx context.Context, c *app.RequestContext) {
	id, err := strconv.ParseInt(c.Param("account_id"), 10, 64)
	if err != nil {
		h.resp.BadRequest(c, "invalid_request", "invalid account id")
		return
	}

	account, err := h.svc.Get(ctx, id)
	if err != nil {
		h.resp.Error(ctx, c, "service", err)
		return
	}
	c.JSON(consts.StatusOK, AccountResponse{
		AccountID: account.ID,
		Balance:   account.Balance,
	})
}
