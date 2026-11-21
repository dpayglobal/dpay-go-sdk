package dpay

import (
	"context"
	"errors"
	"testing"
)

func TestMapAPIErrorRateLimitReset(t *testing.T) {
	err := mapAPIError(response(429, `{"message":"Too Many Attempts.","transactionId":"tx-1","cardError":{"code":"x"}}`, map[string]string{
		"retry-after": "12", "x-ratelimit-limit": "60", "x-ratelimit-remaining": "0", "x-ratelimit-reset": "1790503560",
	}))
	var rateLimit *RateLimitError
	if !errors.As(err, &rateLimit) {
		t.Fatalf("err = %v", err)
	}
	if *rateLimit.RetryAfter != 12 || *rateLimit.Limit != 60 || *rateLimit.Remaining != 0 || rateLimit.Reset == nil || *rateLimit.Reset != 1790503560 {
		t.Fatalf("rateLimit = %+v", rateLimit)
	}
	if rateLimit.TransactionID != "" || rateLimit.CardError != nil {
		t.Fatal("the rate limit error does not carry the transaction context")
	}
	errors.As(mapAPIError(response(429, `{}`, nil)), &rateLimit)
	if rateLimit.Reset != nil {
		t.Fatal("an absent X-RateLimit-Reset must stay nil")
	}
}

func TestMapAPIErrorCarriesTransactionIDAndCardError(t *testing.T) {
	body := `{"status":"failed","message":"Card payment declined","errors":{},` +
		`"transactionId":"A75AEBB4-4B89-4834-AD43-EF442C133769",` +
		`"cardError":{"code":"payment_declined","declineCode":"51","declineMessage":"Insufficient funds or over credit limit"}}`
	var apiErr *APIError
	err := mapAPIError(response(400, body, nil))
	if !errors.Is(err, ErrInvalidRequest) || !errors.As(err, &apiErr) {
		t.Fatalf("err = %v", err)
	}
	if apiErr.TransactionID != "A75AEBB4-4B89-4834-AD43-EF442C133769" {
		t.Fatalf("TransactionID = %q", apiErr.TransactionID)
	}
	want := CardError{Code: "payment_declined", DeclineCode: "51", DeclineMessage: "Insufficient funds or over credit limit"}
	if apiErr.CardError == nil || *apiErr.CardError != want {
		t.Fatalf("CardError = %+v", apiErr.CardError)
	}

	// every mapped status carries them
	for _, status := range []int{401, 403, 404, 422, 409, 500} {
		errors.As(mapAPIError(response(status, body, nil)), &apiErr)
		if apiErr.TransactionID == "" || apiErr.CardError == nil {
			t.Errorf("HTTP %d lost the transaction context", status)
		}
	}
}

func TestMapAPIErrorCardErrorKeepsNullFields(t *testing.T) {
	var apiErr *APIError
	errors.As(mapAPIError(response(400,
		`{"status":"failed","message":"Declined","cardError":{"code":"payment_declined","declineCode":null,"declineMessage":null}}`, nil)), &apiErr)
	if apiErr.CardError == nil || *apiErr.CardError != (CardError{Code: "payment_declined"}) || apiErr.TransactionID != "" {
		t.Fatalf("apiErr = %+v", apiErr)
	}
	errors.As(mapAPIError(response(400, `{"cardError":{"code":7,"declineCode":5,"declineMessage":true}}`, nil)), &apiErr)
	if *apiErr.CardError != (CardError{Code: "7", DeclineCode: "5"}) {
		t.Fatalf("integers are kept as strings, other values dropped: %+v", apiErr.CardError)
	}
}

func TestMapAPIErrorWithoutTransactionContextHasNone(t *testing.T) {
	var apiErr *APIError
	errors.As(mapAPIError(response(400, `{"status":"failed","message":"Invalid checksum","errors":{}}`, nil)), &apiErr)
	if apiErr.TransactionID != "" || apiErr.CardError != nil {
		t.Fatalf("apiErr = %+v", apiErr)
	}
	errors.As(mapAPIError(response(400, `{"transactionId":5,"cardError":"x"}`, nil)), &apiErr)
	if apiErr.TransactionID != "" || apiErr.CardError != nil {
		t.Fatalf("only a string id and an object are taken: %+v", apiErr)
	}
}

func TestRegisterErrorCarriesTransactionIDAndCardError(t *testing.T) {
	client, _ := scriptedClient(t, scriptedResponse{status: 400, body: `{"status":"failed","message":"Card payment declined",` +
		`"transactionId":"A75AEBB4-4B89-4834-AD43-EF442C133769","cardError":{"code":"payment_declined","declineCode":"51","declineMessage":"x"}}`})
	request := minimalRequest()
	request.CardRecurringAlias = String("card-alias")
	request.TransactionType = TransactionTypeCardRecurring
	_, err := client.Payments.Register(context.Background(), request)
	var apiErr *APIError
	if !errors.As(err, &apiErr) || apiErr.TransactionID != "A75AEBB4-4B89-4834-AD43-EF442C133769" || apiErr.CardError.DeclineCode != "51" {
		t.Fatalf("err = %v", err)
	}
}

func TestPaymentRejectedTransactionIDIsOnTheBaseError(t *testing.T) {
	err := paymentRejectedFromAPI(map[string]any{"error": true, "msg": "Transaction canceled", "transactionId": "tx-9"})
	var apiErr *APIError
	if !errors.As(error(err), &apiErr) || apiErr.TransactionID != "tx-9" || err.TransactionID != "tx-9" {
		t.Fatalf("err = %+v", err)
	}
}

func TestCardDeclineAtHTTP200CarriesCardError(t *testing.T) {
	client, _ := scriptedClient(t,
		okResponse(`{"success":false,"status":"error","message":"Payment declined",`+
			`"error":{"code":"payment_declined","declineCode":"05","declineMessage":"Do not honor"}}`),
		okResponse(`{"success":false,"status":"error","message":"DCC_OFFER_EXPIRED"}`),
	)
	_, err := client.Cards.Capture(context.Background(), "tx-1", PLN(2999))
	var cardErr *CardPaymentError
	if !errors.As(err, &cardErr) || cardErr.ErrorCode != "Payment declined" || cardErr.HTTPStatus != 200 {
		t.Fatalf("err = %v", err)
	}
	if cardErr.CardError == nil || *cardErr.CardError != (CardError{Code: "payment_declined", DeclineCode: "05", DeclineMessage: "Do not honor"}) {
		t.Fatalf("CardError = %+v", cardErr.CardError)
	}
	_, err = client.Cards.Capture(context.Background(), "tx-1", PLN(2999))
	if !errors.As(err, &cardErr) || cardErr.CardError != nil {
		t.Fatalf("a decline without error has no CardError: %+v", cardErr)
	}
}

func TestExtraHeadersAreSentAndValidated(t *testing.T) {
	client, doer := scriptedClient(t, okResponse(`{}`))
	if _, err := client.send(context.Background(), "POST", hostPanel, "/x", []byte(`{}`),
		requestHeader{name: "DigitalPayments-PSP-Message-ID", value: "RET-1"}); err != nil {
		t.Fatal(err)
	}
	if values := map[string][]string(doer.requests[0].Header)["DigitalPayments-PSP-Message-ID"]; len(values) != 1 || values[0] != "RET-1" {
		t.Fatalf("headers = %v - the name must go out exactly as given", doer.requests[0].Header)
	}
	if doer.requests[0].Header.Get("Accept") != "application/json" || doer.requests[0].Header.Get("Content-Type") != "application/json" {
		t.Fatal("the SDK headers must stay")
	}

	for _, header := range []requestHeader{
		{name: "X-Evil\r\nHost", value: "x"},
		{name: "X Space", value: "x"},
		{name: "X-Id", value: "a\r\nX-Evil: 1"},
		{name: "X-Id", value: "a\x7f"},
	} {
		_, err := client.send(context.Background(), "POST", hostPanel, "/x", []byte(`{}`), header)
		if !errors.Is(err, ErrInvalidArgument) || err.Error() != `dpay: Invalid request header "`+header.name+`"` {
			t.Errorf("%q: err = %v", header.name, err)
		}
	}
	if len(doer.requests) != 1 {
		t.Fatal("an invalid header must not be sent")
	}
}
