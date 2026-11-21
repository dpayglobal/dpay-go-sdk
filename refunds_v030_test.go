package dpay

import (
	"context"
	"errors"
	"reflect"
	"testing"
)

func TestRefundAvailabilityBusinessRefusalsOfAny4xxAreAnswers(t *testing.T) {
	cases := []struct {
		status  int
		body    string
		message string
	}{
		{404, `{"status":"error","refund":false,"message":"Transaction does not exist!","0":"Transaction does not exist!"}`, "Transaction does not exist!"},
		{423, `{"status":"error","refund":false,"code":"BUYER_SUPPORT_REFUND_NOT_APPLICABLE","message":"Zwrot przez BOK dpay"}`, "Zwrot przez BOK dpay"},
		{401, `{"refund":false,"message":"Transakcji dokonanych kanałami paysafecard nie można zwrócić"}`, "Transakcji dokonanych kanałami paysafecard nie można zwrócić"},
		{409, `{"refund":false,"message":"Refund in progress"}`, "Refund in progress"},
	}
	for _, testCase := range cases {
		client, _ := scriptedClient(t, scriptedResponse{status: testCase.status, body: testCase.body})
		availability, err := client.Refunds.CheckAvailability(context.Background(), "TX-1")
		if err != nil {
			t.Errorf("HTTP %d: err = %v", testCase.status, err)
			continue
		}
		if availability.IsAvailable() || availability.HTTPStatus() != testCase.status || availability.Message() != testCase.message {
			t.Errorf("HTTP %d: availability = %+v", testCase.status, availability)
		}
	}
}

func TestRefundAvailabilityWithoutRefundKeyOrOnServerErrorFails(t *testing.T) {
	cases := []struct {
		status int
		body   string
		kind   error
	}{
		{404, `{"message":"Not Found"}`, ErrNotFound},
		{500, `{"status":"error","refund":false,"message":"Server error"}`, ErrServer},
		{401, `{"refund":false,"message":"Unauthorized request"}`, ErrAuthentication},
		{422, `{"message":"The given data was invalid.","errors":{"transaction_id":["required"]}}`, ErrInvalidRequest},
	}
	for _, testCase := range cases {
		client, _ := scriptedClient(t, scriptedResponse{status: testCase.status, body: testCase.body})
		_, err := client.Refunds.CheckAvailability(context.Background(), "TX-1")
		var apiErr *APIError
		if !errors.Is(err, testCase.kind) || !errors.As(err, &apiErr) || apiErr.HTTPStatus != testCase.status {
			t.Errorf("HTTP %d: err = %v", testCase.status, err)
		}
	}
}

func TestRefundMessageIDGoesToTheHeaderNotTheChecksum(t *testing.T) {
	client, doer := scriptedClient(t, okResponse(`{"status":"success","refund":true,"message":"dpay.pl zwrot RET-2026-0001"}`))
	refund, err := client.Refunds.Create(context.Background(), "30D9493D-1D73-3FBD-A5D4-633723CC7A68", WithRefundMessageID("RET-2026-0001"))
	if err != nil {
		t.Fatal(err)
	}
	if refund.Message() != "dpay.pl zwrot RET-2026-0001" {
		t.Fatalf("message = %q", refund.Message())
	}
	if values := map[string][]string(doer.requests[0].Header)[RefundMessageIDHeader]; !reflect.DeepEqual(values, []string{"RET-2026-0001"}) {
		t.Fatalf("headers = %v", doer.requests[0].Header)
	}
	if RefundMessageIDHeader != "DigitalPayments-PSP-Message-ID" {
		t.Fatal("header name changed")
	}
	if keys := keysInOrder(t, doer.bodies[0]); !reflect.DeepEqual(keys, []string{"service", "transaction_id", "checksum"}) {
		t.Fatalf("keys = %v", keys)
	}
	if doer.lastBody(t)["checksum"] != "3b345f55900ae6a43220dd14a3827ed9a26a02a3eb46b856d302e2baa44b48a4" {
		t.Fatalf("checksum = %v", doer.lastBody(t)["checksum"])
	}
}

func TestRefundWithoutMessageIDSendsNoHeader(t *testing.T) {
	client, doer := scriptedClient(t, okResponse(`{"status":"success","refund":true,"message":"dpay.pl TX"}`), okResponse(`{"refund":true}`))
	if _, err := client.Refunds.Create(context.Background(), "TX-1"); err != nil {
		t.Fatal(err)
	}
	// CheckAvailability ignores the option, like the webhook
	if _, err := client.Refunds.CheckAvailability(context.Background(), "TX-1", WithRefundMessageID("RET-1")); err != nil {
		t.Fatal(err)
	}
	for index, request := range doer.requests {
		if headerValue(request.Header, RefundMessageIDHeader) != "" {
			t.Errorf("request %d sent the message ID header", index)
		}
	}
}

func TestInvalidRefundMessageIDIsRejected(t *testing.T) {
	for _, messageID := range []string{"", "   ", "RET\r\nX-Evil: 1", "ZWROT-ŁÓDŹ", "TAB\tX", "RET\n"} {
		client, doer := scriptedClient(t)
		_, err := client.Refunds.Create(context.Background(), "TX-1", WithRefundMessageID(messageID))
		if !errors.Is(err, ErrInvalidArgument) || err.Error() != "dpay: Refund message ID must be non-empty printable ASCII" {
			t.Errorf("%q: err = %v", messageID, err)
		}
		if len(doer.requests) != 0 {
			t.Errorf("%q: a request was sent", messageID)
		}
	}
}

// The fixture refund_full checksum does not change with a message ID.
func TestRefundMessageIDKeepsTheSharedVector(t *testing.T) {
	vectors := loadAPIVectors(t)
	want := ""
	for _, vector := range vectors.OrderedBody {
		if vector.Name == "refund_full" {
			want = vector.Checksum
		}
	}
	if want == "" {
		t.Fatal("vector refund_full is missing")
	}
	doer := &scriptedDoer{responses: []scriptedResponse{okResponse(`{"status":"success","refund":true}`)}}
	client, _ := New(vectors.Service, vectors.SecretHash, WithHTTPClient(doer))
	if _, err := client.Refunds.Create(context.Background(), vectors.TransactionID, WithRefundMessageID("RET-2026-0001")); err != nil {
		t.Fatal(err)
	}
	if doer.lastBody(t)["checksum"] != want {
		t.Fatalf("checksum = %v, want %s", doer.lastBody(t)["checksum"], want)
	}
}
