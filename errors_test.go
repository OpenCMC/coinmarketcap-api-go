package coinmarketcap

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
)

func TestHTTPErrorsAreTyped(t *testing.T) {
	tests := []struct {
		status   int
		body     string
		is       func(error) bool
		wantCode int
		wantMsg  string
	}{
		{400, `{"status":{"error_code":400,"error_message":"Invalid value for \"id\""}}`, as[*BadRequestError], 400, `Invalid value for "id"`},
		{401, `{"status":{"error_code":1001,"error_message":"This API Key is invalid."}}`, as[*AuthenticationError], 1001, "This API Key is invalid."},
		{402, `{"status":{"error_code":"1004","error_message":"Your API Key's subscription plan has expired."}}`, as[*PaymentRequiredError], 1004, "Your API Key's subscription plan has expired."},
		{403, `{"status":{"error_code":1005,"error_message":"Forbidden"}}`, as[*ForbiddenError], 1005, "Forbidden"},
		{404, `{"message":"Not Found"}`, as[*NotFoundError], 0, "Not Found"},
		{429, `{"status":{"error_code":1008,"error_message":"Rate limit"}}`, as[*RateLimitError], 1008, "Rate limit"},
		{500, `not json`, as[*InternalServerError], 0, ""},
		{503, `{"error":"unavailable"}`, as[*InternalServerError], 0, "unavailable"},
		{418, `{"status":{"error_code":"teapot"}}`, isPlainCMCError, 0, ""},
	}
	for _, tt := range tests {
		srv := newTestServer(t, reply{status: tt.status, body: tt.body, header: map[string]string{"Retry-After": "7"}})
		cmc := mustNew(t, WithAPIKey("key"), WithBaseURL(srv.URL), WithMaxRetries(0))

		_, err := cmc.Tools.KeyInfo(context.Background())
		if !tt.is(err) {
			t.Errorf("status %d: wrong error type %T: %v", tt.status, err, err)
			continue
		}
		var apiErr *CMCError
		if !errors.As(err, &apiErr) {
			t.Errorf("status %d: errors.As(*CMCError) failed for %T", tt.status, err)
			continue
		}
		if apiErr.StatusCode != tt.status || apiErr.ErrorCode != tt.wantCode || apiErr.Message != tt.wantMsg {
			t.Errorf("status %d: got status %d code %d message %q", tt.status, apiErr.StatusCode, apiErr.ErrorCode, apiErr.Message)
		}
		if apiErr.Header.Get("Retry-After") != "7" || string(apiErr.Body) != tt.body {
			t.Errorf("status %d: headers or body not kept", tt.status)
		}
	}
}

func as[T error](err error) bool {
	var target T
	return errors.As(err, &target)
}

func isPlainCMCError(err error) bool {
	_, ok := err.(*CMCError)
	return ok
}

func TestErrorMessage(t *testing.T) {
	err := newAPIError(401, nil, []byte(`{"status":{"error_code":1001,"error_message":"This API Key is invalid."}}`))
	want := "coinmarketcap: request failed with status 401: This API Key is invalid. (error_code 1001)"
	if err.Error() != want {
		t.Errorf("Error() = %q, want %q", err.Error(), want)
	}
	if got := newAPIError(502, nil, nil).Error(); got != "coinmarketcap: request failed with status 502" {
		t.Errorf("Error() = %q", got)
	}
}

func TestDecodeErrorKeepsWhatDecoded(t *testing.T) {
	body := `{"status":{"error_code":0,"credit_count":1},"data":"not a list"}`
	srv := newTestServer(t, reply{status: 200, body: body})
	cmc := mustNew(t, WithAPIKey("key"), WithBaseURL(srv.URL))

	res, err := cmc.Cryptocurrency.ListingsLatest(context.Background(), nil)
	var decodeErr *DecodeError
	if !errors.As(err, &decodeErr) {
		t.Fatalf("error = %v, want *DecodeError", err)
	}
	var typeErr *json.UnmarshalTypeError
	if !errors.As(err, &typeErr) || typeErr.Field != "data" {
		t.Errorf("want the json type error for field data, got %v", err)
	}
	if string(decodeErr.Body) != body || decodeErr.StatusCode != 200 {
		t.Errorf("DecodeError body/status not kept")
	}
	if res == nil || res.Status == nil || res.Status.CreditCount != 1 {
		t.Errorf("the fields that did decode should be returned, got %+v", res)
	}
}

func TestInvalidJSONIsADecodeError(t *testing.T) {
	srv := newTestServer(t, reply{status: 200, body: `<html>`})
	cmc := mustNew(t, WithAPIKey("key"), WithBaseURL(srv.URL))

	res, err := cmc.Tools.KeyInfo(context.Background())
	var decodeErr *DecodeError
	if !errors.As(err, &decodeErr) || res != nil {
		t.Fatalf("got %v, %v; want nil and *DecodeError", res, err)
	}
}

func TestRawEndpointReturnsBody(t *testing.T) {
	srv := newTestServer(t, reply{status: 200, body: `{"info":{"name":"collection"}}`})
	cmc := mustNew(t, WithAPIKey("key"), WithBaseURL(srv.URL))

	raw, err := cmc.Tools.Postman(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != `{"info":{"name":"collection"}}` {
		t.Errorf("raw = %s", raw)
	}
}
