//go:build integration

package integration

import (
	"bytes"
	"encoding/json"
	"strconv"
	"testing"

	"github.com/cloudwego/hertz/pkg/common/ut"
)

// jsonBody builds a ut.Body from a raw JSON string.
// jsonBody 用原始 JSON 字符串构造 ut.Body。
func jsonBody(s string) *ut.Body {
	return &ut.Body{Body: bytes.NewBufferString(s), Len: len(s)}
}

var jsonHeader = ut.Header{Key: "Content-Type", Value: "application/json"}

// TestHTTP_AccountLifecycle drives the real endpoints: create, then read back.
// TestHTTP_AccountLifecycle 驱动真实接口：创建账户后再读回。
func TestHTTP_AccountLifecycle(t *testing.T) {
	e := setup(t)

	w := ut.PerformRequest(e.engine, "POST", "/accounts",
		jsonBody(`{"account_id":123,"initial_balance":"100.23344"}`), jsonHeader)
	if got := w.Result().StatusCode(); got != 201 {
		t.Fatalf("create status = %d, want 201", got)
	}

	w = ut.PerformRequest(e.engine, "GET", "/accounts/123", nil)
	if got := w.Result().StatusCode(); got != 200 {
		t.Fatalf("get status = %d, want 200", got)
	}
	var body struct {
		AccountID int64  `json:"account_id"`
		Balance   string `json:"balance"`
	}
	if err := json.Unmarshal(w.Result().Body(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.AccountID != 123 || body.Balance != "100.23344" {
		t.Fatalf("body = %+v, want {123, 100.23344}", body)
	}
}

// TestHTTP_AccountErrors covers the error status codes on the account endpoints.
// TestHTTP_AccountErrors 覆盖账户接口的错误状态码。
func TestHTTP_AccountErrors(t *testing.T) {
	e := setup(t)

	// Malformed JSON -> 400. 非法 JSON -> 400。
	w := ut.PerformRequest(e.engine, "POST", "/accounts", jsonBody(`{not json`), jsonHeader)
	if got := w.Result().StatusCode(); got != 400 {
		t.Errorf("malformed body status = %d, want 400", got)
	}

	// Unknown account -> 404. 账户不存在 -> 404。
	w = ut.PerformRequest(e.engine, "GET", "/accounts/999", nil)
	if got := w.Result().StatusCode(); got != 404 {
		t.Errorf("missing account status = %d, want 404", got)
	}

	// Duplicate id -> 409. 重复 id -> 409。
	_ = ut.PerformRequest(e.engine, "POST", "/accounts", jsonBody(`{"account_id":1,"initial_balance":"5"}`), jsonHeader)
	w = ut.PerformRequest(e.engine, "POST", "/accounts", jsonBody(`{"account_id":1,"initial_balance":"5"}`), jsonHeader)
	if got := w.Result().StatusCode(); got != 409 {
		t.Errorf("duplicate account status = %d, want 409", got)
	}
}

// TestHTTP_Transfer drives a transfer over HTTP and checks the moved balances.
// TestHTTP_Transfer 通过 HTTP 发起转账并校验余额变动。
func TestHTTP_Transfer(t *testing.T) {
	e := setup(t)
	mustCreate(t, e, `{"account_id":1,"initial_balance":"100.23344"}`)
	mustCreate(t, e, `{"account_id":2,"initial_balance":"0"}`)

	w := ut.PerformRequest(e.engine, "POST", "/transactions",
		jsonBody(`{"source_account_id":1,"destination_account_id":2,"amount":"100.12345"}`), jsonHeader)
	if got := w.Result().StatusCode(); got != 201 {
		t.Fatalf("transfer status = %d, want 201", got)
	}

	if bal := getBalance(t, e, 1); bal != "0.10999" {
		t.Errorf("account 1 balance = %s, want 0.10999", bal)
	}
	if bal := getBalance(t, e, 2); bal != "100.12345" {
		t.Errorf("account 2 balance = %s, want 100.12345", bal)
	}
}

// TestHTTP_TransferInsufficient checks the 409 + error body on overdraft.
// TestHTTP_TransferInsufficient 校验余额不足时的 409 与错误响应体。
func TestHTTP_TransferInsufficient(t *testing.T) {
	e := setup(t)
	mustCreate(t, e, `{"account_id":1,"initial_balance":"5"}`)
	mustCreate(t, e, `{"account_id":2,"initial_balance":"0"}`)

	w := ut.PerformRequest(e.engine, "POST", "/transactions",
		jsonBody(`{"source_account_id":1,"destination_account_id":2,"amount":"10"}`), jsonHeader)
	if got := w.Result().StatusCode(); got != 409 {
		t.Fatalf("status = %d, want 409", got)
	}
	var body struct {
		Error string `json:"error"`
	}
	_ = json.Unmarshal(w.Result().Body(), &body)
	if body.Error != "insufficient funds" {
		t.Errorf("error = %q, want \"insufficient funds\"", body.Error)
	}
}

// TestHTTP_TransferIdempotent retries with the same Idempotency-Key header and
// asserts the money moves only once.
// TestHTTP_TransferIdempotent 用相同 Idempotency-Key 头重试，断言资金只移动一次。
func TestHTTP_TransferIdempotent(t *testing.T) {
	e := setup(t)
	mustCreate(t, e, `{"account_id":1,"initial_balance":"100"}`)
	mustCreate(t, e, `{"account_id":2,"initial_balance":"0"}`)

	key := ut.Header{Key: "Idempotency-Key", Value: "550e8400-e29b-41d4-a716-446655440000"}
	payload := `{"source_account_id":1,"destination_account_id":2,"amount":"30"}`

	for i := 0; i < 3; i++ {
		w := ut.PerformRequest(e.engine, "POST", "/transactions", jsonBody(payload), jsonHeader, key)
		if got := w.Result().StatusCode(); got != 201 {
			t.Fatalf("attempt %d status = %d, want 201", i, got)
		}
	}

	if bal := getBalance(t, e, 1); bal != "70" {
		t.Errorf("account 1 balance = %s, want 70 (debited once)", bal)
	}
}

// mustCreate creates an account over HTTP and fails the test on a non-201.
// mustCreate 通过 HTTP 创建账户，非 201 即失败。
func mustCreate(t *testing.T, e *env, payload string) {
	t.Helper()
	w := ut.PerformRequest(e.engine, "POST", "/accounts", jsonBody(payload), jsonHeader)
	if got := w.Result().StatusCode(); got != 201 {
		t.Fatalf("create account status = %d, want 201", got)
	}
}

// getBalance reads an account's balance string over HTTP.
// getBalance 通过 HTTP 读取账户余额字符串。
func getBalance(t *testing.T, e *env, id int64) string {
	t.Helper()
	w := ut.PerformRequest(e.engine, "GET", "/accounts/"+strconv.FormatInt(id, 10), nil)
	var body struct {
		Balance string `json:"balance"`
	}
	if err := json.Unmarshal(w.Result().Body(), &body); err != nil {
		t.Fatalf("decode balance: %v", err)
	}
	return body.Balance
}
