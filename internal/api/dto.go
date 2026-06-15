package api

import "github.com/shopspring/decimal"

// CreateAccountRequest is the POST /accounts body.
// CreateAccountRequest 是 POST /accounts 的请求体。
type CreateAccountRequest struct {
	AccountID      int64           `json:"account_id"`
	InitialBalance decimal.Decimal `json:"initial_balance"`
}

// AccountResponse is the GET /accounts/{id} body.
// AccountResponse 是 GET /accounts/{id} 的响应体。
type AccountResponse struct {
	AccountID int64           `json:"account_id"`
	Balance   decimal.Decimal `json:"balance"`
}

// TransferRequest is the POST /transactions body.
// TransferRequest 是 POST /transactions 的请求体。
type TransferRequest struct {
	SourceAccountID      int64           `json:"source_account_id"`
	DestinationAccountID int64           `json:"destination_account_id"`
	Amount               decimal.Decimal `json:"amount"`
}

// ErrorResponse is the standard error body.
// ErrorResponse 是统一的错误响应体。
type ErrorResponse struct {
	Error string `json:"error"`
}

// ReadyResponse is the GET /readyz body.
// ReadyResponse 是 GET /readyz 的响应体。
type ReadyResponse struct {
	Status string            `json:"status"`
	Checks map[string]string `json:"checks,omitempty"`
}
