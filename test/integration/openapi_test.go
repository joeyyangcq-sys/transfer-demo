//go:build integration

package integration

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/cloudwego/hertz/pkg/common/ut"
	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/getkin/kin-openapi/routers/gorillamux"
)

// TestOpenAPIConformance drives success and every error case through the real
// router and validates each response against api/openapi.yaml. If a handler
// returns a status or shape the spec does not declare, this fails — keeping the
// API documentation and the implementation in sync.
// TestOpenAPIConformance 通过真实路由跑成功与各类错误用例，并把每个响应对着
// api/openapi.yaml 校验。若 handler 返回了规范未声明的状态码/结构，测试即失败，
// 从而保证接口文档与实现同步。
func TestOpenAPIConformance(t *testing.T) {
	e := setup(t)
	ctx := context.Background()

	loader := openapi3.NewLoader()
	doc, err := loader.LoadFromFile("../../api/openapi.yaml")
	if err != nil {
		t.Fatalf("load spec: %v", err)
	}
	if err := doc.Validate(ctx); err != nil {
		t.Fatalf("invalid spec: %v", err)
	}
	router, err := gorillamux.NewRouter(doc)
	if err != nil {
		t.Fatalf("build router: %v", err)
	}

	conform := func(name, method, path, body string, hdrs ...ut.Header) {
		t.Helper()
		var reqBody *ut.Body
		if body != "" {
			reqBody = jsonBody(body)
		}
		w := ut.PerformRequest(e.engine, method, path, reqBody, hdrs...)
		resp := w.Result()

		req, err := http.NewRequest(method, "http://localhost:8080"+path, strings.NewReader(body))
		if err != nil {
			t.Fatalf("%s: build req: %v", name, err)
		}
		if body != "" {
			req.Header.Set("Content-Type", "application/json")
		}
		route, pathParams, err := router.FindRoute(req)
		if err != nil {
			t.Fatalf("%s: no documented route for %s %s: %v", name, method, path, err)
		}

		respHeader := http.Header{}
		if ct := string(resp.Header.ContentType()); ct != "" {
			respHeader.Set("Content-Type", ct)
		}
		input := &openapi3filter.ResponseValidationInput{
			RequestValidationInput: &openapi3filter.RequestValidationInput{
				Request:    req,
				PathParams: pathParams,
				Route:      route,
			},
			Status:  resp.StatusCode(),
			Header:  respHeader,
			Body:    io.NopCloser(bytes.NewReader(resp.Body())),
			Options: &openapi3filter.Options{IncludeResponseStatus: true},
		}
		if err := openapi3filter.ValidateResponse(ctx, input); err != nil {
			t.Errorf("%s: %s %s -> %d is not conformant with openapi.yaml: %v",
				name, method, path, resp.StatusCode(), err)
		}
	}

	jh := jsonHeader
	badKey := ut.Header{Key: "Idempotency-Key", Value: "not-a-uuid"}
	goodKey := ut.Header{Key: "Idempotency-Key", Value: "550e8400-e29b-41d4-a716-446655440000"}

	// Seed + success.
	conform("create src", "POST", "/accounts", `{"account_id":1,"initial_balance":"100"}`, jh)
	conform("create dst", "POST", "/accounts", `{"account_id":2,"initial_balance":"0"}`, jh)
	conform("get account 200", "GET", "/accounts/1", "")
	conform("transfer 200", "POST", "/transactions", `{"source_account_id":1,"destination_account_id":2,"amount":"10"}`, jh)

	// Errors — each must match a documented response.
	conform("malformed 400", "POST", "/accounts", `{bad`, jh)
	conform("non-positive account id 400", "POST", "/accounts", `{"account_id":0,"initial_balance":"1"}`, jh)
	conform("duplicate 409", "POST", "/accounts", `{"account_id":1,"initial_balance":"1"}`, jh)
	conform("not found 404", "GET", "/accounts/999", "")
	conform("same account 400", "POST", "/transactions", `{"source_account_id":1,"destination_account_id":1,"amount":"1"}`, jh)
	conform("bad idem key 400", "POST", "/transactions", `{"source_account_id":1,"destination_account_id":2,"amount":"1"}`, jh, badKey)
	conform("transfer src 404", "POST", "/transactions", `{"source_account_id":999,"destination_account_id":2,"amount":"1"}`, jh)
	conform("transfer dst 404", "POST", "/transactions", `{"source_account_id":1,"destination_account_id":888,"amount":"1"}`, jh)
	conform("insufficient 409", "POST", "/transactions", `{"source_account_id":2,"destination_account_id":1,"amount":"9999"}`, jh)
	conform("idem first 200", "POST", "/transactions", `{"source_account_id":1,"destination_account_id":2,"amount":"5"}`, jh, goodKey)
	conform("idem conflict 422", "POST", "/transactions", `{"source_account_id":1,"destination_account_id":2,"amount":"6"}`, jh, goodKey)
}
