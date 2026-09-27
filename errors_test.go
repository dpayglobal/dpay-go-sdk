package dpay

import (
	"context"
	"errors"
	"fmt"
	"testing"
)

func TestAPIErrorSentinels(t *testing.T) {
	err := &APIError{kind: ErrNotFound, message: "no such transaction", HTTPStatus: 404}
	if !errors.Is(err, ErrNotFound) {
		t.Fatal("must match its own sentinel")
	}
	if !errors.Is(err, ErrAPI) {
		t.Fatal("every API error must match ErrAPI")
	}
	if errors.Is(err, ErrRateLimit) {
		t.Fatal("must not match a foreign sentinel")
	}
	if err.Error() != "dpay: no such transaction (HTTP 404)" {
		t.Fatalf("Error() = %q", err.Error())
	}
}

func TestAPIErrorAsFromSpecializedTypes(t *testing.T) {
	rejected := &PaymentRejectedError{
		APIError:      &APIError{kind: ErrPaymentRejected, message: "rejected", HTTPStatus: 200, ErrorCode: "err05"},
		TransactionID: "tx-1",
	}
	var apiErr *APIError
	if !errors.As(error(rejected), &apiErr) {
		t.Fatal("errors.As to *APIError must work through embedding")
	}
	if apiErr.ErrorCode != "err05" {
		t.Fatalf("ErrorCode = %q", apiErr.ErrorCode)
	}
	if !errors.Is(error(rejected), ErrPaymentRejected) || !errors.Is(error(rejected), ErrAPI) {
		t.Fatal("sentinels broken for specialized type")
	}
	var target *PaymentRejectedError
	if !errors.As(error(rejected), &target) || target.TransactionID != "tx-1" {
		t.Fatal("errors.As to the concrete type must work")
	}
}

func TestRateLimitError(t *testing.T) {
	retryAfter := 30
	err := &RateLimitError{
		APIError:   &APIError{kind: ErrRateLimit, message: "slow down", HTTPStatus: 429},
		RetryAfter: &retryAfter,
	}
	if !errors.Is(error(err), ErrRateLimit) {
		t.Fatal("sentinel broken")
	}
	var target *RateLimitError
	if !errors.As(error(err), &target) || *target.RetryAfter != 30 {
		t.Fatal("RetryAfter lost")
	}
}

func TestTransportErrorUnwrapsCause(t *testing.T) {
	err := newTransportError("request failed", context.Canceled)
	if !errors.Is(err, ErrTransport) {
		t.Fatal("must match ErrTransport")
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatal("must unwrap to the cause")
	}
	if errors.Is(err, ErrAPI) {
		t.Fatal("transport failures are not API errors")
	}
}

func TestValidationError(t *testing.T) {
	err := newValidationError(`Option "service" is required and must be a non-empty string`)
	if !errors.Is(err, ErrInvalidArgument) {
		t.Fatal("must match ErrInvalidArgument")
	}
	if err.Error() != `dpay: Option "service" is required and must be a non-empty string` {
		t.Fatalf("Error() = %q", err.Error())
	}
}

func TestSignatureAndCardEncryptionErrors(t *testing.T) {
	signature := &SignatureError{message: "Invalid IPN signature"}
	if !errors.Is(error(signature), ErrSignature) {
		t.Fatal("signature sentinel broken")
	}
	encryption := &CardEncryptionError{message: "Invalid RSA public key"}
	if !errors.Is(error(encryption), ErrCardEncryption) {
		t.Fatal("card encryption sentinel broken")
	}
	if encryption.Error() != "dpay: Invalid RSA public key" {
		t.Fatalf("Error() = %q", encryption.Error())
	}
}

func TestAPIErrorWithoutStatus(t *testing.T) {
	err := &APIError{kind: ErrPaymentRejected, message: "Payment rejected", HTTPStatus: 200}
	if got := fmt.Sprint(err); got != "dpay: Payment rejected (HTTP 200)" {
		t.Fatalf("Error() = %q", got)
	}
}

func response(status int, body string, headers map[string]string) apiResponse {
	normalized := map[string]string{}
	for name, value := range headers {
		normalized[lowercase(name)] = value
	}
	return apiResponse{status: status, headers: normalized, body: []byte(body)}
}

func TestMapAPIErrorByStatus(t *testing.T) {
	cases := []struct {
		status   int
		sentinel error
	}{
		{401, ErrAuthentication},
		{403, ErrAccessDenied},
		{404, ErrNotFound},
		{400, ErrInvalidRequest},
		{422, ErrInvalidRequest},
		{500, ErrServer},
		{503, ErrServer},
		{418, ErrAPI},
	}
	for _, c := range cases {
		err := mapAPIError(response(c.status, `{"message":"boom"}`, nil))
		if !errors.Is(err, c.sentinel) {
			t.Errorf("status %d did not map to %v, got %v", c.status, c.sentinel, err)
		}
		var apiErr *APIError
		if !errors.As(err, &apiErr) || apiErr.HTTPStatus != c.status {
			t.Errorf("status %d lost", c.status)
		}
	}
}

func TestMapAPIErrorMessageFallbacks(t *testing.T) {
	var apiErr *APIError

	errors.As(mapAPIError(response(400, `{"message":"from message"}`, nil)), &apiErr)
	if apiErr.Error() != "dpay: from message (HTTP 400)" {
		t.Fatalf("message key: %q", apiErr.Error())
	}
	errors.As(mapAPIError(response(400, `{"msg":"from msg"}`, nil)), &apiErr)
	if apiErr.Error() != "dpay: from msg (HTTP 400)" {
		t.Fatalf("msg key: %q", apiErr.Error())
	}
	errors.As(mapAPIError(response(400, `not json`, nil)), &apiErr)
	if apiErr.Error() != "dpay: Unexpected API error (HTTP 400)" {
		t.Fatalf("fallback: %q", apiErr.Error())
	}
	if apiErr.RawBody != "not json" {
		t.Fatalf("RawBody = %q", apiErr.RawBody)
	}
}

func TestMapAPIErrorFieldErrors(t *testing.T) {
	var apiErr *APIError

	errors.As(mapAPIError(response(400, `{"errors":{"value":"is required"}}`, nil)), &apiErr)
	if len(apiErr.FieldErrors["value"]) != 1 || apiErr.FieldErrors["value"][0] != "is required" {
		t.Fatalf("string form: %v", apiErr.FieldErrors)
	}
	errors.As(mapAPIError(response(422, `{"errors":{"value":["too low","too odd"]}}`, nil)), &apiErr)
	if len(apiErr.FieldErrors["value"]) != 2 || apiErr.FieldErrors["value"][1] != "too odd" {
		t.Fatalf("array form: %v", apiErr.FieldErrors)
	}
	errors.As(mapAPIError(response(422, `{"errors":{"n":[1,true]}}`, nil)), &apiErr)
	if apiErr.FieldErrors["n"][0] != "1" || apiErr.FieldErrors["n"][1] != "1" {
		t.Fatalf("scalar coercion: %v", apiErr.FieldErrors)
	}
}

func TestMapAPIErrorErrorCode(t *testing.T) {
	var apiErr *APIError
	errors.As(mapAPIError(response(403, `{"message":"no","errorcode":"err01"}`, nil)), &apiErr)
	if apiErr.ErrorCode != "err01" {
		t.Fatalf("ErrorCode = %q", apiErr.ErrorCode)
	}
}

func TestMapAPIErrorRateLimit(t *testing.T) {
	err := mapAPIError(response(429, `{"message":"slow"}`, map[string]string{
		"Retry-After":           "30",
		"X-RateLimit-Limit":     "100",
		"X-RateLimit-Remaining": "0",
	}))
	var rateLimit *RateLimitError
	if !errors.As(err, &rateLimit) {
		t.Fatal("429 must map to *RateLimitError")
	}
	if *rateLimit.RetryAfter != 30 || *rateLimit.Limit != 100 || *rateLimit.Remaining != 0 {
		t.Fatalf("headers lost: %+v", rateLimit)
	}
	missing := mapAPIError(response(429, `{}`, nil))
	errors.As(missing, &rateLimit)
	if rateLimit.RetryAfter != nil {
		t.Fatal("absent header must stay nil")
	}
}

func TestMapAPIErrorCodeAndReasonOfCardsAndWebhookErrors(t *testing.T) {
	var apiErr *APIError
	body := `{"success":false,"status":"error","code":"WEBHOOK_URL_INVALID","reason":"https_required","message":"Invalid webhook URL: https_required"}`
	err := mapAPIError(response(400, body, nil))
	if !errors.Is(err, ErrInvalidRequest) || !errors.As(err, &apiErr) {
		t.Fatalf("err = %v", err)
	}
	if apiErr.ErrorCode != "WEBHOOK_URL_INVALID" || apiErr.Reason != "https_required" {
		t.Fatalf("apiErr = %+v", apiErr)
	}
}

func TestMapAPIErrorMissingChecksumIsAnAuthenticationError(t *testing.T) {
	var apiErr *APIError
	body := `{"success":false,"status":"error","code":"CHECKSUM_REQUIRED","message":"Missing service or checksum"}`
	err := mapAPIError(response(401, body, nil))
	if !errors.Is(err, ErrAuthentication) || !errors.As(err, &apiErr) || apiErr.ErrorCode != "CHECKSUM_REQUIRED" || apiErr.Reason != "" {
		t.Fatalf("err = %v", err)
	}
}

func TestMapAPIErrorPrefersCodeOverErrorcode(t *testing.T) {
	var apiErr *APIError
	errors.As(mapAPIError(response(400, `{"code":"INVALID_CHECKSUM","errorcode":"err01","reason":5}`, nil)), &apiErr)
	if apiErr.ErrorCode != "INVALID_CHECKSUM" || apiErr.Reason != "" {
		t.Fatalf("apiErr = %+v", apiErr)
	}
	errors.As(mapAPIError(response(400, `{"code":7,"errorcode":"err01"}`, nil)), &apiErr)
	if apiErr.ErrorCode != "err01" {
		t.Fatalf("a non-string code falls back to errorcode: %+v", apiErr)
	}
}

func TestMapAPIErrorRateLimitCarriesNoReason(t *testing.T) {
	var rateLimit *RateLimitError
	errors.As(mapAPIError(response(429, `{"message":"slow","code":"X","reason":"y"}`, nil)), &rateLimit)
	if rateLimit == nil || rateLimit.ErrorCode != "" || rateLimit.Reason != "" {
		t.Fatalf("rateLimit = %+v", rateLimit)
	}
}

func TestPaymentRejectedCarriesTheErrorDescription(t *testing.T) {
	server, _, _ := recordingServer(t, 200, `{"error":true,"msg":"Transaction canceled","status":false,"transactionId":"tx-9",`+
		`"additionalInfo":{"error":"INSUFFICIENT_FUNDS","error_description":"IssId: 1"}}`)
	client, _ := New("svc", "hash", WithBaseURLs(BaseURLs{APIPayments: server.URL}))

	_, err := client.Payments.Register(context.Background(), minimalRequest())
	var rejected *PaymentRejectedError
	if !errors.As(err, &rejected) {
		t.Fatalf("err = %v", err)
	}
	if rejected.ErrorCode != "INSUFFICIENT_FUNDS" || rejected.ErrorDescription != "IssId: 1" || rejected.TransactionID != "tx-9" {
		t.Fatalf("rejected = %+v", rejected)
	}
}
