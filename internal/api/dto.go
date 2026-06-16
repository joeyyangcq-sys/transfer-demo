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

// TransactionResponse is the POST /transactions success body — the recorded
// transfer. On an idempotent replay it is the original transfer, so the same
// transaction_id comes back without moving money again.
// TransactionResponse 是 POST /transactions 的成功响应体——已记录的转账。
// 幂等重放时返回的是原始转账，因此会回传相同的 transaction_id 且不再扣款。
type TransactionResponse struct {
	TransactionID        int64           `json:"transaction_id"`
	SourceAccountID      int64           `json:"source_account_id"`
	DestinationAccountID int64           `json:"destination_account_id"`
	Amount               decimal.Decimal `json:"amount"`
	Status               string          `json:"status"`
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
