package dpay

import (
	"context"
	"errors"
	"strings"
	"testing"
)

const recurringTransactionID = "A75AEBB4-4B89-4834-AD43-EF442C133769"

// newVectorClient uses the synthetic service and hash of testdata/api_vectors.json.
func newVectorClient(t *testing.T, status int, response string) (*Client, *string, *string) {
	t.Helper()
	server, sent, path := recordingServer(t, status, response)
	client, err := New("sdk-test-service", "sdk-test-hash-0001", WithBaseURLs(BaseURLs{APIPayments: server.URL, Panel: server.URL}))
	if err != nil {
		t.Fatal(err)
	}
	return client, sent, path
}

func TestRecurringStatusReturnsTheAliasAndTerms(t *testing.T) {
	client, sent, path := newVectorClient(t, 200, `{"status":"success","data":{
		"alias":"SUB-0001","method":"blik","status":"ACTIVE","expiration_date":"2027-09-30",
		"registration":{"transaction_id":"`+recurringTransactionID+`","label":"Abonament","model":"A",
			"frequency":"1M","limit_amt":5999,"tot_limit_amt":71988,"is_limit_amt_fixed":true,
			"init_date":"2026-11-01","terms_url":"https://shop.example/terms","terms_version":"2026-09",
			"registered_at":"2026-09-26T12:00:00+02:00"}}}`)

	status, err := client.Recurring.Status(context.Background(), "SUB-0001")
	if err != nil {
		t.Fatal(err)
	}
	if *path != "/api/v1_0/payments/recurring/status" {
		t.Fatalf("path = %q", *path)
	}
	// sha256(service|hash|alias)
	want := `{"service":"sdk-test-service","alias":"SUB-0001","checksum":"01e38925de1ceefbfbaffdf84a917f1c95123403a0e51a7d74cb81f227c3e3ef"}`
	if *sent != want {
		t.Fatalf("body = %s\nwant   %s", *sent, want)
	}
	if !status.IsActive() || status.Status() != RecurringStateActive || status.Alias() != "SUB-0001" ||
		status.Method() != "blik" || status.ExpirationDate() != "2027-09-30" {
		t.Fatalf("status = %v", status.Raw())
	}
	registration := status.Registration()
	if registration == nil || registration.TransactionID() != recurringTransactionID || registration.Model() != RecurringModelAutomatic {
		t.Fatalf("registration = %v", registration)
	}
	if *registration.LimitAmt() != 5999 || *registration.TotLimitAmt() != 71988 || !*registration.IsLimitAmtFixed() {
		t.Fatal("limits lost")
	}
	if registration.TermsURL() != "https://shop.example/terms" || registration.TermsVersion() != "2026-09" ||
		registration.Label() != "Abonament" || registration.Frequency() != "1M" || registration.InitDate() != "2026-11-01" ||
		registration.RegisteredAt() != "2026-09-26T12:00:00+02:00" {
		t.Fatalf("terms lost: %v", registration.Raw())
	}
}

func TestRecurringStatusWithoutRegistration(t *testing.T) {
	client, _, _ := newVectorClient(t, 200, `{"status":"success","data":{"alias":"SUB-2","status":null,"registration":null}}`)
	status, err := client.Recurring.Status(context.Background(), "SUB-2")
	if err != nil {
		t.Fatal(err)
	}
	if status.Registration() != nil || status.IsActive() || status.Status() != "" {
		t.Fatalf("status = %v", status.Raw())
	}
}

func TestRecurringCancelSignsTheOperation(t *testing.T) {
	client, sent, path := newVectorClient(t, 200, `{"status":"success","data":{"alias":"SUB-0001","status":"UNREGISTERED"}}`)

	state, err := client.Recurring.Cancel(context.Background(), "SUB-0001", WithCancelReason("Rezygnacja"))
	if err != nil {
		t.Fatal(err)
	}
	if state != RecurringStateUnregistered || *path != "/api/v1_0/payments/recurring/cancel" {
		t.Fatalf("state = %q, path = %q", state, *path)
	}
	// sha256(service|hash|alias|cancel) - a status checksum cannot cancel
	want := `{"service":"sdk-test-service","alias":"SUB-0001","reason":"Rezygnacja",` +
		`"checksum":"848c236b3060a94c2ed14c150de154ee7aa2d64ec3682771259c30321cadf82b"}`
	if *sent != want {
		t.Fatalf("body = %s\nwant   %s", *sent, want)
	}
}

func TestRecurringCancelDefaultsToUnregistered(t *testing.T) {
	client, sent, _ := newVectorClient(t, 200, `{"status":"success","data":[]}`)
	state, err := client.Recurring.Cancel(context.Background(), "SUB-0001")
	if err != nil {
		t.Fatal(err)
	}
	if state != RecurringStateUnregistered {
		t.Fatalf("state = %q", state)
	}
	if strings.Contains(*sent, "reason") {
		t.Fatalf("no reason must be sent: %s", *sent)
	}
}

func TestRecurringRetryReturnsThePendingRetry(t *testing.T) {
	client, sent, path := newVectorClient(t, 200,
		`{"status":"success","data":{"transactionId":"`+recurringTransactionID+`","retry":{"status":"pending","count":1}}}`)

	result, err := client.Recurring.Retry(context.Background(), recurringTransactionID)
	if err != nil {
		t.Fatal(err)
	}
	if *path != "/api/v1_0/payments/recurring/retry" {
		t.Fatalf("path = %q", *path)
	}
	want := `{"service":"sdk-test-service","transaction_id":"` + recurringTransactionID +
		`","checksum":"00bceec5fca2ee4c3a1737156df376441d225790eb61ca47fcbe9cba7243a77d"}`
	if *sent != want {
		t.Fatalf("body = %s\nwant   %s", *sent, want)
	}
	if !result.IsPending() || result.IsFailed() || *result.Count() != 1 || result.TransactionID() != recurringTransactionID {
		t.Fatalf("result = %v", result.Raw())
	}
}

func TestRecurringRetryDeclinedAtOnceIsAResultNotAnError(t *testing.T) {
	client, _, _ := newVectorClient(t, 200, `{"status":"success","data":{"transactionId":"`+recurringTransactionID+
		`","retry":{"status":"failed","count":2,"error":"INSUFFICIENT_FUNDS","error_description":"IssId: 1"}}}`)

	result, err := client.Recurring.Retry(context.Background(), recurringTransactionID)
	if err != nil {
		t.Fatal(err)
	}
	if !result.IsFailed() || result.Status() != RecurringRetryStatusFailed || result.ErrorCode() != "INSUFFICIENT_FUNDS" ||
		result.ErrorDescription() != "IssId: 1" || *result.Count() != 2 {
		t.Fatalf("result = %v", result.Raw())
	}
}

func TestRecurringRetryNotAllowedCarriesTheReason(t *testing.T) {
	client, _, _ := newVectorClient(t, 400, `{"status":"failed","message":"Recurring charge cannot be retried (DECLINE_NOT_RETRYABLE).",
		"errors":{"retry":"DECLINE_NOT_RETRYABLE","decline_reason":"SEC_DECLINED"}}`)

	_, err := client.Recurring.Retry(context.Background(), recurringTransactionID)
	var apiErr *APIError
	if !errors.Is(err, ErrInvalidRequest) || !errors.As(err, &apiErr) {
		t.Fatalf("err = %v", err)
	}
	if apiErr.HTTPStatus != 400 || len(apiErr.FieldErrors["retry"]) != 1 || apiErr.FieldErrors["retry"][0] != "DECLINE_NOT_RETRYABLE" {
		t.Fatalf("apiErr = %+v", apiErr)
	}
}

func TestRecurringRegistrationModelOSendsNoFrequencyOrLimits(t *testing.T) {
	registration := RecurringRegistration{
		Label: "Abonament", Model: RecurringModelOnDemand, TermsURL: "https://shop.example/terms",
		Alias: String("SUB-0001"), Methods: []RecurringMethod{RecurringMethodBlik}, TermsVersion: String("2026-09"),
	}
	if err := registration.Validate(); err != nil {
		t.Fatal(err)
	}
	want := `{"label":"Abonament","alias":"SUB-0001","model":"O","methods":["blik"],` +
		`"terms_url":"https://shop.example/terms","terms_version":"2026-09"}`
	if got := bodyJSON(t, registration.toBody()); got != want {
		t.Fatalf("toBody() = %s\nwant       %s", got, want)
	}
}

func TestRecurringRegistrationModelAKeepsTheOrder(t *testing.T) {
	registration := RecurringRegistration{
		Label: "Abonament", Model: RecurringModelAutomatic, TermsURL: "https://shop.example/terms",
		Frequency: String("1M"), LimitAmt: Int(5999), TotLimitAmt: Int(71988),
		ExpirationDate: String("2027-09-30"), InitDate: String("2026-11-01"),
	}
	if err := registration.Validate(); err != nil {
		t.Fatal(err)
	}
	want := `{"label":"Abonament","model":"A","frequency":"1M","limit_amt":5999,"tot_limit_amt":71988,` +
		`"expiration_date":"2027-09-30","init_date":"2026-11-01","terms_url":"https://shop.example/terms"}`
	if got := bodyJSON(t, registration.toBody()); got != want {
		t.Fatalf("toBody() = %s\nwant       %s", got, want)
	}
}

func TestRecurringRegistrationValidate(t *testing.T) {
	base := func() RecurringRegistration {
		return RecurringRegistration{Label: "Abonament", Model: RecurringModelManual, TermsURL: "https://shop.example/terms"}
	}
	modelA := func() RecurringRegistration {
		registration := base()
		registration.Model = RecurringModelAutomatic
		registration.Frequency, registration.LimitAmt, registration.TotLimitAmt = String("1M"), Int(5999), Int(71988)
		registration.ExpirationDate, registration.InitDate = String("2027-09-30"), String("2026-11-01")
		return registration
	}
	cases := []struct {
		want   string
		modify func(*RecurringRegistration)
	}{
		{"Recurring payment label must be 1-50 characters", func(r *RecurringRegistration) { r.Label = "" }},
		{"Recurring payment label must be 1-50 characters", func(r *RecurringRegistration) { r.Label = strings.Repeat("ł", 51) }},
		{`Invalid recurring model "B"`, func(r *RecurringRegistration) { r.Model = "B" }},
		{`Invalid terms URL "not a url"`, func(r *RecurringRegistration) { r.TermsURL = "not a url" }},
		{`Invalid terms URL ""`, func(r *RecurringRegistration) { r.TermsURL = "" }},
		{"Recurring alias must be 1-128 characters", func(r *RecurringRegistration) { r.Alias = String(strings.Repeat("a", 129)) }},
		{"Terms version must be 1-64 characters", func(r *RecurringRegistration) { r.TermsVersion = String("") }},
		{"Methods must be a non-empty list of distinct methods", func(r *RecurringRegistration) { r.Methods = []RecurringMethod{} }},
		{"Methods must be a non-empty list of distinct methods", func(r *RecurringRegistration) {
			r.Methods = []RecurringMethod{RecurringMethodBlik, RecurringMethodBlik}
		}},
		{`Unsupported recurring method "card"`, func(r *RecurringRegistration) { r.Methods = []RecurringMethod{"card"} }},
		// Q (quarterly) is not a BLIK frequency
		{`Invalid recurring frequency "1Q"`, func(r *RecurringRegistration) { r.Frequency = String("1Q") }},
		{`Invalid recurring frequency "0M"`, func(r *RecurringRegistration) { r.Frequency = String("0M") }},
		{"limit_amt must be at least 1 (minor units)", func(r *RecurringRegistration) { r.LimitAmt = Int(0) }},
		{"tot_limit_amt must be at least 1 (minor units)", func(r *RecurringRegistration) { r.TotLimitAmt = Int(-5) }},
		{`Date "2027/09/30" must be in YYYY-MM-DD format`, func(r *RecurringRegistration) { r.ExpirationDate = String("2027/09/30") }},
		{`Date "01-11-2026" must be in YYYY-MM-DD format`, func(r *RecurringRegistration) { r.InitDate = String("01-11-2026") }},
	}
	for _, testCase := range cases {
		registration := base()
		testCase.modify(&registration)
		if err := registration.Validate(); !errors.Is(err, ErrInvalidArgument) || err.Error() != "dpay: "+testCase.want {
			t.Errorf("err = %v, want %q", err, testCase.want)
		}
	}

	modelO := base()
	modelO.Model, modelO.Frequency = RecurringModelOnDemand, String("1M")
	if err := modelO.Validate(); err == nil || err.Error() != "dpay: frequency is not allowed in recurring model O" {
		t.Fatalf("model O: err = %v", err)
	}
	modelO.Frequency, modelO.LimitAmtFixed = nil, Bool(true)
	if err := modelO.Validate(); err == nil || err.Error() != "dpay: is_limit_amt_fixed is not allowed in recurring model O" {
		t.Fatalf("model O: err = %v", err)
	}

	incomplete := modelA()
	incomplete.InitDate = nil
	if err := incomplete.Validate(); err == nil || err.Error() != "dpay: init_date is required in recurring model A" {
		t.Fatalf("model A: err = %v", err)
	}
	notFixed := modelA()
	notFixed.LimitAmtFixed = Bool(false)
	if err := notFixed.Validate(); err == nil ||
		err.Error() != "dpay: Recurring model A requires a fixed amount (is_limit_amt_fixed = true)" {
		t.Fatalf("model A: err = %v", err)
	}
	fixed := modelA()
	fixed.LimitAmtFixed = Bool(true)
	if err := fixed.Validate(); err != nil {
		t.Fatalf("model A with a fixed amount: %v", err)
	}

	manual := base()
	manual.Frequency, manual.LimitAmt, manual.LimitAmtFixed = String("14D"), Int(1000), Bool(false)
	if err := manual.Validate(); err != nil {
		t.Fatalf("model M takes optional terms: %v", err)
	}
}

func recurringURLs(withIPN bool) ReturnURLs {
	urls := ReturnURLs{Success: "https://shop.example/ok", Fail: "https://shop.example/fail"}
	if withIPN {
		urls.IPN = "https://shop.example/ipn"
	}
	return urls
}

func TestRecurringRegistrationRequestWithoutIPN(t *testing.T) {
	client, sent, _ := newVectorClient(t, 200, `{"error":false,"msg":"Internal processing","status":true,"transactionId":"TX-REG",
		"additionalInfo":{"recurring_registration":{"alias":"SUB-0001","methods":["blik"]}}}`)

	payment, err := client.Payments.Register(context.Background(), &RegisterPaymentRequest{
		Amount: PLN(0), TransactionType: TransactionTypeTransfers, URLs: recurringURLs(false),
		BlikCode: String("777123"), UserAgent: String("Mozilla/5.0"), UserIP: String("83.238.17.42"),
		RecurringRegistration: &RecurringRegistration{
			Label: "Abonament", Model: RecurringModelOnDemand, TermsURL: "https://shop.example/terms", Alias: String("SUB-0001"),
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	want := `{"service":"sdk-test-service","value":"0.00","transactionType":"transfers",` +
		`"url_success":"https://shop.example/ok","url_fail":"https://shop.example/fail",` +
		`"user_agent":"Mozilla/5.0","user_ip":"83.238.17.42","blik_code":"777123",` +
		`"recurring_registration":{"label":"Abonament","alias":"SUB-0001","model":"O","terms_url":"https://shop.example/terms"},` +
		// without IPN: an empty last segment, the registration keeps the alias out of the checksum
		`"checksum":"b5dbca75c515bc76094dba4d2853493bf050bfcd47c25bc899361884080f3544"}`
	if *sent != want {
		t.Fatalf("body = %s\nwant   %s", *sent, want)
	}
	if payment.RecurringAlias() != "SUB-0001" || len(payment.RecurringMethods()) != 1 || payment.RecurringMethods()[0] != "blik" {
		t.Fatalf("payment = %v", payment.Raw())
	}
}

func TestRecurringChargeBindsTheAliasInTheChecksum(t *testing.T) {
	client, sent, _ := newVectorClient(t, 200, `{"error":false,"msg":"Internal processing","status":true,"transactionId":"TX-CHG"}`)

	if _, err := client.Payments.Register(context.Background(), &RegisterPaymentRequest{
		Amount: PLN(4999), TransactionType: TransactionTypeTransfers, URLs: recurringURLs(true),
		RecurringAlias: String("SUB-0001"), Description: String("Abonament 10/2026"),
	}); err != nil {
		t.Fatal(err)
	}
	body := decodeBody(t, *sent)
	if body["recurring_alias"] != "SUB-0001" {
		t.Fatalf("body = %v", body)
	}
	if _, present := body["user_ip"]; present {
		t.Fatal("the client context is optional for a charge")
	}
	// sha256(service|hash|value|url_success|url_fail|url_ipn|recurring_alias)
	if body["checksum"] != "96b80b9bceab99b92228bc8bd793b432b1594484ac45dc2715d2ebd542f3b2aa" {
		t.Fatalf("checksum = %v", body["checksum"])
	}
}

func TestRecurringChargeWithoutIPNKeepsTheEmptySegment(t *testing.T) {
	client, sent, _ := newVectorClient(t, 200, `{"error":false,"msg":"Internal processing","status":true,"transactionId":"TX-CHG"}`)

	if _, err := client.Payments.Register(context.Background(), &RegisterPaymentRequest{
		Amount: PLN(4999), TransactionType: TransactionTypeTransfers, URLs: recurringURLs(false),
		RecurringAlias: String("SUB-0001"), UserAgent: String("Mozilla/5.0"), UserIP: String("83.238.17.42"),
	}); err != nil {
		t.Fatal(err)
	}
	body := decodeBody(t, *sent)
	if body["user_ip"] != "83.238.17.42" || body["checksum"] != "b521018f255bab928187d73802d2dd79c48dda4c6574d1f57e7395cda70a8e5c" {
		t.Fatalf("body = %v", body)
	}
	if _, present := body["url_ipn"]; present {
		t.Fatal("url_ipn must not be sent without an IPN URL")
	}
}

func TestWebhookAndReferenceStayOutOfTheRegistrationChecksum(t *testing.T) {
	client, sent, _ := newVectorClient(t, 200, `{"error":false,"msg":"https://secure.dpay.pl/transfer@pay@TX","status":true,"transactionId":"TX"}`)

	if _, err := client.Payments.Register(context.Background(), &RegisterPaymentRequest{
		Amount: PLN(1000), TransactionType: TransactionTypeTransfers, URLs: recurringURLs(true),
		Webhook: &WebhookTarget{URL: "https://shop.example/webhooks", Events: []WebhookEventType{
			WebhookEventTypePaymentSucceeded, WebhookEventTypePaymentFailed,
		}},
		Reference: String("  order-1234\t"),
	}); err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(*sent, `"webhook":{"url":"https://shop.example/webhooks","events":["payment.succeeded","payment.failed"]},`+
		`"reference":"order-1234","checksum":"0a163ad60b5d2fd09eacce0032cc4d584e707c457051d2c4365b320dc340bc28"}`) {
		t.Fatalf("body = %s", *sent)
	}
}

func TestRecurringRequestsAreRejectedBeforeSending(t *testing.T) {
	registration := &RecurringRegistration{Label: "Abonament", Model: RecurringModelOnDemand, TermsURL: "https://shop.example/terms"}
	charge := func(modify func(*RegisterPaymentRequest)) *RegisterPaymentRequest {
		request := &RegisterPaymentRequest{
			Amount: PLN(4999), TransactionType: TransactionTypeTransfers, URLs: recurringURLs(true), RecurringAlias: String("SUB-0001"),
		}
		modify(request)
		return request
	}
	cases := []struct {
		want    string
		request *RegisterPaymentRequest
	}{
		{"recurring_registration requires the customer's BLIK code (BlikCode)", charge(func(r *RegisterPaymentRequest) {
			r.RecurringAlias, r.RecurringRegistration, r.Amount = nil, registration, PLN(0)
		})},
		{"blik_code cannot be combined with a recurring payment", charge(func(r *RegisterPaymentRequest) {
			r.BlikCode, r.UserAgent, r.UserIP = String("777123"), String("Mozilla/5.0"), String("83.238.17.42")
		})},
		{"A recurring charge requires an amount above 0", charge(func(r *RegisterPaymentRequest) { r.Amount = PLN(0) })},
		{`Recurring payments require transactionType "transfers"`, charge(func(r *RegisterPaymentRequest) {
			r.TransactionType = TransactionTypeCardRecurring
		})},
		{"recurring_registration cannot be combined with recurring_alias", charge(func(r *RegisterPaymentRequest) {
			r.RecurringRegistration, r.BlikCode = registration, String("777123")
		})},
		{"channel cannot be combined with a recurring payment", charge(func(r *RegisterPaymentRequest) {
			r.RecurringAlias, r.RecurringRegistration, r.BlikCode, r.Channel = nil, registration, String("777123"), String("86")
		})},
		{"card_recurring_alias cannot be combined with a recurring payment", charge(func(r *RegisterPaymentRequest) {
			r.CardRecurringAlias = String("card-1")
		})},
		{"register_blik_alias cannot be combined with a recurring payment", charge(func(r *RegisterPaymentRequest) {
			r.RegisterBlikAlias = &BlikAliasRegistration{Label: "Sklep", Type: BlikAliasTypeUID}
		})},
		{"blik_alias cannot be combined with blik_code, alias registration or recurring payments", charge(func(r *RegisterPaymentRequest) {
			r.BlikAlias = String("alias-1")
		})},
		{"Recurring alias must be 1-128 characters", charge(func(r *RegisterPaymentRequest) { r.RecurringAlias = String("") })},
		{`Invalid user IP "localhost"`, charge(func(r *RegisterPaymentRequest) { r.UserIP = String("localhost") })},
		{"Reference must be 1-64 characters without control characters", charge(func(r *RegisterPaymentRequest) {
			r.Reference = String(" \t ")
		})},
		{"Reference must be 1-64 characters without control characters", charge(func(r *RegisterPaymentRequest) {
			r.Reference = String("order\n1")
		})},
		{"Reference must be 1-64 characters without control characters", charge(func(r *RegisterPaymentRequest) {
			r.Reference = String(strings.Repeat("ż", 65))
		})},
		{`Event "payout.paid" is not allowed in the webhook object of a payment registration`, charge(func(r *RegisterPaymentRequest) {
			r.Webhook = &WebhookTarget{URL: "https://shop.example/webhooks", Events: []WebhookEventType{WebhookEventTypePayoutPaid}}
		})},
		{`Webhook URL "http://shop.example/webhooks" must be a valid https:// URL`, charge(func(r *RegisterPaymentRequest) {
			r.Webhook = &WebhookTarget{URL: "http://shop.example/webhooks"}
		})},
		{"frequency is not allowed in recurring model O", charge(func(r *RegisterPaymentRequest) {
			r.RecurringAlias, r.BlikCode, r.Amount = nil, String("777123"), PLN(0)
			r.RecurringRegistration = &RecurringRegistration{
				Label: "Abonament", Model: RecurringModelOnDemand, TermsURL: "https://shop.example/terms", Frequency: String("1M"),
			}
		})},
	}
	client, _ := New("svc", "hash", WithBaseURLs(BaseURLs{APIPayments: "http://127.0.0.1:1"}))
	for _, testCase := range cases {
		_, err := client.Payments.Register(context.Background(), testCase.request)
		if !errors.Is(err, ErrInvalidArgument) || err.Error() != "dpay: "+testCase.want {
			t.Errorf("err = %v, want %q", err, testCase.want)
		}
	}
}

func TestRemovedTransactionTypesAreRejected(t *testing.T) {
	// blik_recurring: registration through RecurringRegistration; bizum_direct: the API does not support it
	for _, removed := range []TransactionType{"blik_recurring", "bizum_direct"} {
		request := &RegisterPaymentRequest{Amount: PLN(1000), TransactionType: removed, URLs: recurringURLs(true)}
		if err := request.Validate(); err == nil || err.Error() != `dpay: Invalid transaction type "`+string(removed)+`"` {
			t.Errorf("%s: err = %v", removed, err)
		}
	}
}
