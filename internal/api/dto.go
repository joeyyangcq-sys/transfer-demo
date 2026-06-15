package api

import "github.com/shopspring/decimal"

// CreateAccountRequest is the POST /accounts body.
type CreateAccountRequest struct {
	AccountID      int64           `json:"account_id"`
	InitialBalance decimal.Decimal `json:"initial_balance"`
}

// AccountResponse is the GET /accounts/{id} body.
type AccountResponse struct {
	AccountID int64           `json:"account_id"`
	Balance   decimal.Decimal `json:"balance"`
}

// TransferRequest is the POST /transactions body.
type TransferRequest struct {
	SourceAccountID      int64           `json:"source_account_id"`
	DestinationAccountID int64           `json:"destination_account_id"`
	Amount               decimal.Decimal `json:"amount"`
}

// ErrorResponse is the standard error body.
type ErrorResponse struct {
	Error string `json:"error"`
}

// ReadyResponse is the GET /readyz body.
type ReadyResponse struct {
	Status string            `json:"status"`
	Checks map[string]string `json:"checks,omitempty"`
}
