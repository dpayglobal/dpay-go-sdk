package dpay

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"testing"
)

const trimTransactionID = "30D9493D-1D73-3FBD-A5D4-633723CC7A68"

func trimClient(t *testing.T, service string, responses ...scriptedResponse) (*Client, *scriptedDoer) {
	t.Helper()
	doer := &scriptedDoer{responses: responses}
	client, err := New(service, "secret123", WithHTTPClient(doer))
	if err != nil {
		t.Fatal(err)
	}
	return client, doer
}

func assertInvalidArgument(t *testing.T, err error, message string, doer *scriptedDoer) {
	t.Helper()
	if !errors.Is(err, ErrInvalidArgument) || err.Error() != "dpay: "+message {
		t.Errorf("err = %v, want %q", err, message)
	}
	if len(doer.requests) != 0 {
		t.Errorf("%q: a request was sent", message)
	}
}

// The panel API trims every string before it verifies the checksum, so the
// SDK sends and signs the trimmed values (vectors of the PHP SDK tests).
func TestRefundCreateSendsAndSignsTrimmedValues(t *testing.T) {
	client, doer := trimClient(t, " MyShop ", okResponse(`{"status":"success","refund":true}`))
	if _, err := client.Refunds.Create(context.Background(), " "+trimTransactionID+"\n",
		WithRefundAmount(PLN(1500)), WithRefundReason("\u3000reklamacja\u200b")); err != nil {
		t.Fatal(err)
	}
	body := doer.lastBody(t)
	if body["service"] != "MyShop" || body["transaction_id"] != trimTransactionID || body["value"] != "15.00" || body["reason"] != "reklamacja" {
		t.Fatalf("body = %v", body)
	}
	if body["checksum"] != "5c00684addf39fda89d3d6cff88abf26bb8173119056fd3012050422943c798a" {
		t.Fatalf("checksum = %v", body["checksum"])
	}
}

func TestRefundAvailabilitySendsABlankReasonAsEmpty(t *testing.T) {
	client, doer := scriptedClient(t, okResponse(`{"refund":true}`))
	if _, err := client.Refunds.CheckAvailability(context.Background(), trimTransactionID,
		WithRefundAmount(PLN(1500)), WithRefundReason("\ufeff \n")); err != nil {
		t.Fatal(err)
	}
	if keys := keysInOrder(t, doer.bodies[0]); !reflect.DeepEqual(keys, []string{"service", "transaction_id", "value", "reason", "checksum"}) {
		t.Fatalf("keys = %v", keys)
	}
	body := doer.lastBody(t)
	if body["reason"] != "" || body["checksum"] != sha256Hex(t, "MyShop|"+trimTransactionID+"|15.00||secret123") ||
		body["checksum"] != "12dcb016aa2302bbe8ea71be9007eeacdedaeab1d4dc317966bab1b1e8d53a1c" {
		t.Fatalf("body = %v", body)
	}
}

func TestBlankTransactionIDIsRejectedBeforeSending(t *testing.T) {
	client, doer := scriptedClient(t)
	_, err := client.Refunds.Create(context.Background(), " \t")
	assertInvalidArgument(t, err, "transaction_id must not be empty", doer)
	_, err = client.Refunds.CheckAvailability(context.Background(), "")
	assertInvalidArgument(t, err, "transaction_id must not be empty", doer)
	_, err = client.Payments.Details(context.Background(), "\u200b")
	assertInvalidArgument(t, err, "transaction_id must not be empty", doer)
}

func TestPanelValidationOrder(t *testing.T) {
	// refunds.Create: webhook events, message ID, service, transaction_id
	client, doer := trimClient(t, "\u3000")
	_, err := client.Refunds.Create(context.Background(), "", WithRefundMessageID("RET\n"))
	assertInvalidArgument(t, err, "Refund message ID must be non-empty printable ASCII", doer)
	_, err = client.Refunds.Create(context.Background(), "")
	assertInvalidArgument(t, err, "service must not be empty", doer)
	_, err = client.Refunds.CheckAvailability(context.Background(), "")
	assertInvalidArgument(t, err, "service must not be empty", doer)
	_, err = client.Payments.Details(context.Background(), "")
	assertInvalidArgument(t, err, "service must not be empty", doer)
	// payouts.List: service before the filters, date_from before date_to before state
	invalidState := PayoutState(2)
	_, err = client.Payouts.List(context.Background(), &PayoutListParams{DateFrom: String(" "), State: &invalidState})
	assertInvalidArgument(t, err, "service must not be empty", doer)
	_, err = client.Payouts.DetailsFull(context.Background(), 1, &PayoutPageParams{PerPage: Int(0)})
	assertInvalidArgument(t, err, "service must not be empty", doer)

	client, doer = scriptedClient(t)
	_, err = client.Payouts.List(context.Background(), &PayoutListParams{DateTo: String("\n"), DateFrom: String("\u00a0"), State: &invalidState})
	assertInvalidArgument(t, err, "date_from must not be empty", doer)
	_, err = client.Payouts.List(context.Background(), &PayoutListParams{DateTo: String("\n"), State: &invalidState})
	assertInvalidArgument(t, err, "date_to must not be empty", doer)
}

func TestPaymentDetailsSendsTheTrimmedTransactionID(t *testing.T) {
	client, doer := scriptedClient(t, okResponse(`{"transaction":{}}`))
	if _, err := client.Payments.Details(context.Background(), "\t"+trimTransactionID+" "); err != nil {
		t.Fatal(err)
	}
	body := doer.lastBody(t)
	if body["transaction_id"] != trimTransactionID || body["checksum"] != "3b345f55900ae6a43220dd14a3827ed9a26a02a3eb46b856d302e2baa44b48a4" {
		t.Fatalf("body = %v", body)
	}
}

func TestBanksForServiceTrimsTheConfiguredService(t *testing.T) {
	client, doer := trimClient(t, " MyShop\ufeff", okResponse(`[]`))
	if _, err := client.Banks.ForService(context.Background(), WithTimestamp(1753100000)); err != nil {
		t.Fatal(err)
	}
	body := doer.lastBody(t)
	if body["service"] != "MyShop" || body["checksum"] != "189d0301f163fb105c9f3d607e8fe0adbbfc5ecd8e16271a682d672f43ab623c" {
		t.Fatalf("body = %v", body)
	}
	if client.Service() != " MyShop\ufeff" {
		t.Fatal("the configured service itself must not change")
	}

	client, doer = trimClient(t, "\u3000")
	_, err := client.Banks.ForService(context.Background(), WithTimestamp(1753100000))
	assertInvalidArgument(t, err, "service must not be empty", doer)
}

func TestPayoutsTrimTheServiceAndTheDates(t *testing.T) {
	client, doer := trimClient(t, " MyShop\n", okResponse(emptyPayoutPage),
		okResponse(`{"id":1}`), okResponse(`{"settlement":{},"transactions":[],"meta":{}}`))
	ctx := context.Background()
	if _, err := client.Payouts.List(ctx, &PayoutListParams{
		DateFrom: String("\ufeff2026-08-01 "), DateTo: String("\u30002026-08-31 23:59:59\u200b"),
	}); err != nil {
		t.Fatal(err)
	}
	body := decodeBody(t, doer.bodies[0])
	if body["service"] != "MyShop" || body["date_from"] != "2026-08-01" || body["date_to"] != "2026-08-31 23:59:59" ||
		body["checksum"] != "59828763de9628f5794f01750e9876aba13876caac856f62daf7840552f28d6f" {
		t.Fatalf("body = %v", body)
	}
	if _, err := client.Payouts.Details(ctx, 12345); err != nil {
		t.Fatal(err)
	}
	if body := decodeBody(t, doer.bodies[1]); body["service"] != "MyShop" || body["checksum"] != sha256Hex(t, "MyShop|12345|secret123") {
		t.Fatalf("details body = %v", body)
	}
	if _, err := client.Payouts.DetailsFull(ctx, 12345, nil); err != nil {
		t.Fatal(err)
	}
	if body := decodeBody(t, doer.bodies[2]); body["service"] != "MyShop" || body["checksum"] != sha256Hex(t, "MyShop|12345|secret123") {
		t.Fatalf("details/full body = %v", body)
	}
}

// api-payments is not trimmed: the configured service goes out as given.
func TestPaymentsAPIRequestsAreNotTrimmed(t *testing.T) {
	client, doer := trimClient(t, " MyShop ", okResponse(`{"error":false,"msg":"Internal processing","status":true,"transactionId":"TX"}`))
	mustRegister(t, client, context.Background(), &RegisterPaymentRequest{
		Amount: PLN(100), TransactionType: TransactionTypeTransfers,
		URLs: ReturnURLs{Success: "https://shop.example/ok", Fail: "https://shop.example/fail"},
	})
	if service := doer.lastBody(t)["service"]; service != " MyShop " {
		t.Fatalf("service = %q", service)
	}
}

func TestRefundAvailabilityIsDecidedByTheRefundKey(t *testing.T) {
	client, _ := scriptedClient(t,
		scriptedResponse{status: 422, body: `{"status":"error","refund":false,"message":"Refund not possible"}`},
		scriptedResponse{status: 429, body: `{"status":"error","message":"Too Many Requests","error":"Too Many Requests"}`,
			headers: map[string]string{"Retry-After": "12"}},
	)
	availability, err := client.Refunds.CheckAvailability(context.Background(), "TX-1")
	if err != nil || availability.IsAvailable() || availability.HTTPStatus() != 422 || availability.Message() != "Refund not possible" {
		t.Fatalf("availability = %+v, err = %v", availability, err)
	}
	_, err = client.Refunds.CheckAvailability(context.Background(), "TX-1")
	var rateLimit *RateLimitError
	if !errors.Is(err, ErrRateLimit) || !errors.As(err, &rateLimit) || rateLimit.RetryAfter == nil || *rateLimit.RetryAfter != 12 {
		t.Fatalf("err = %v", err)
	}
}

// The payout flags are true only for the JSON integer 1. encoding/json decodes
// 1.0 to the same float64, so in Go it counts as 1 (see isJSONIntegerOne).
func TestPayoutFlagsAreTrueOnlyForTheJSONInteger1(t *testing.T) {
	cases := map[string]bool{`1`: true, `1.0`: true, `0`: false, `"1"`: false, `"1abc"`: false, `true`: false, `2`: false, `null`: false}
	for value, want := range cases {
		var data map[string]any
		raw := `{"paysafecard":` + value + `,"direct_settlement":` + value + `,"declined":` + value + `}`
		if err := json.Unmarshal([]byte(raw), &data); err != nil {
			t.Fatal(err)
		}
		details := payoutDetailsFromAPI(data)
		item := payoutListItemFromAPI(data)
		if details.IsPaysafecard() != want || details.IsDirectSettlement() != want || details.IsDeclined() != want || item.IsDeclined() != want {
			t.Errorf("%s: flags = %v %v %v %v, want %v", value,
				details.IsPaysafecard(), details.IsDirectSettlement(), details.IsDeclined(), item.IsDeclined(), want)
		}
	}
	missing := payoutDetailsFromAPI(map[string]any{})
	if missing.IsPaysafecard() || missing.IsDirectSettlement() || missing.IsDeclined() || payoutListItemFromAPI(map[string]any{}).IsDeclined() {
		t.Fatal("missing flags are false")
	}
}
