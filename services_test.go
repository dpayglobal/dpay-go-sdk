package dpay

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func minimalRequest() *RegisterPaymentRequest {
	return &RegisterPaymentRequest{
		Amount:          PLN(2999),
		TransactionType: TransactionTypeTransfers,
		URLs: ReturnURLs{
			Success: "https://shop.test/ok",
			Fail:    "https://shop.test/fail",
			IPN:     "https://shop.test/ipn",
		},
	}
}

func recordingServer(t *testing.T, status int, response string) (*httptest.Server, *string, *string) {
	t.Helper()
	body := new(string)
	path := new(string)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		buffer := make([]byte, r.ContentLength)
		if r.ContentLength > 0 {
			r.Body.Read(buffer)
		}
		*body, *path = string(buffer), r.URL.EscapedPath()
		w.WriteHeader(status)
		w.Write([]byte(response))
	}))
	t.Cleanup(server.Close)
	return server, body, path
}

func sha256Hex(t *testing.T, payload string) string {
	t.Helper()
	digest := sha256.Sum256([]byte(payload))
	return hex.EncodeToString(digest[:])
}

func expectedRegisterChecksum(t *testing.T, service, secret string, fields ...string) string {
	t.Helper()
	parts := append([]string{service, secret}, fields...)
	return sha256Hex(t, strings.Join(parts, "|"))
}

func expectedOrderedChecksum(t *testing.T, secret string, values ...string) string {
	t.Helper()
	return sha256Hex(t, strings.Join(values, "|")+"|"+secret)
}

func decodeBody(t *testing.T, raw string) map[string]any {
	t.Helper()
	var decoded map[string]any
	if err := json.Unmarshal([]byte(raw), &decoded); err != nil {
		t.Fatalf("body is not JSON: %v (%s)", err, raw)
	}
	return decoded
}

func TestRegisterRequestMinimalBody(t *testing.T) {
	body, err := minimalRequest().toBody("test_service")
	if err != nil {
		t.Fatal(err)
	}
	want := `{"service":"test_service","value":"29.99","transactionType":"transfers",` +
		`"url_success":"https://shop.test/ok","url_fail":"https://shop.test/fail","url_ipn":"https://shop.test/ipn"}`
	if got := bodyJSON(t, body); got != want {
		t.Fatalf("toBody() = %s\nwant      = %s", got, want)
	}
}

func TestRegisterRequestFieldOrder(t *testing.T) {
	request := minimalRequest()
	request.Description = String("Zamówienie #1234")
	request.Custom = String("order-1234")
	request.Payer = &Payer{Email: String("jan@example.com"), FirstName: String("Jan"), LastName: String("Kowalski")}
	request.AcceptTos = Bool(true)
	request.Channel = String("86")
	request.CreditCard = Bool(true)
	request.Paysafecard = Bool(false)
	request.Blik = Bool(true)
	request.Installment = Bool(false)
	request.PayPal = Bool(true)
	request.NoBanks = Bool(false)
	request.PhoneNumber = String("+48123456789")
	request.CurrencyCode = CurrencyEUR
	request.PartnerPlatform = String("SHOPIFY01")
	request.AliasIPNURL = String("https://shop.test/alias-ipn")
	request.NoDelay = Bool(true)
	request.AuthorizeOnly = Bool(false)
	request.CardRecurringOperation = CardRecurringOperationCharge

	body, err := request.toBody("test_service")
	if err != nil {
		t.Fatal(err)
	}
	wantKeys := []string{
		"service", "value", "transactionType", "url_success", "url_fail", "url_ipn",
		"description", "custom", "email", "client_name", "client_surname", "accept_tos", "channel",
		"creditcard", "paysafecard", "blik", "installment", "paypal", "nobanks",
		"phone_number", "currency_code", "partner_platform",
		"alias_ipn_url", "no_delay", "authorize_only", "card_recurring_operation",
	}
	gotKeys := body.Keys()
	if len(gotKeys) != len(wantKeys) {
		t.Fatalf("keys = %v", gotKeys)
	}
	for index, want := range wantKeys {
		if gotKeys[index] != want {
			t.Fatalf("key %d = %q, want %q (full: %v)", index, gotKeys[index], want, gotKeys)
		}
	}
}

func TestRegisterRequestChannelFlagsAreIntegers(t *testing.T) {
	request := minimalRequest()
	request.CreditCard = Bool(true)
	request.Paysafecard = Bool(false)
	request.AcceptTos = Bool(false)
	request.NoDelay = Bool(false)

	body, _ := request.toBody("svc")
	got := bodyJSON(t, body)
	for _, fragment := range []string{`"creditcard":1`, `"paysafecard":0`, `"accept_tos":false`, `"no_delay":false`} {
		if !strings.Contains(got, fragment) {
			t.Fatalf("missing %s in %s", fragment, got)
		}
	}
}

func TestRegisterRequestNestedObjects(t *testing.T) {
	value := PLN(1000)
	request := minimalRequest()
	request.RegisterBlikRecurringAlias = &BlikRecurringRegistration{
		Label: "Subskrypcja", Model: BlikRecurringModelAutomatic, Frequency: "1M", Value: &value,
	}
	request.Payout = &PayoutInstruction{
		FeeMode:   PayoutFeeModeGross,
		Positions: []PayoutPosition{{IBAN: "PL61", Title: "Wypłata 1", Amount: PLN(1050)}},
	}
	request.BillingAddress = NewFields().Set("street", "Testowa 1").Set("city", "Warszawa")
	request.DeviceInfo = &DeviceInfo{DeviceID: "d", ApplicationName: "a"}
	request.Products = []any{NewFields().Set("name", "Produkt").Set("price", 29.99)}
	request.Efaktura = Bool(true)
	request.Invoice = &InvoiceDetails{PayerNIP: String("1234563218")}

	body, err := request.toBody("svc")
	if err != nil {
		t.Fatal(err)
	}
	got := bodyJSON(t, body)
	for _, fragment := range []string{
		`"register_blik_recurring_alias":{"label":"Subskrypcja","type":"PAYID","model":"A","frequency":"1M","value":"10.00"}`,
		`"payout":{"fee_mode":"gross","positions":[{"iban":"PL61","title":"Wypłata 1","amount":10.5}]}`,
		`"billing_address":{"street":"Testowa 1","city":"Warszawa"}`,
		`"products":[{"name":"Produkt","price":29.99}]`,
		`"efaktura":true`,
		`"invoice":{"payer_nip":"1234563218"}`,
	} {
		if !strings.Contains(got, fragment) {
			t.Fatalf("missing %s\nin %s", fragment, got)
		}
	}
}

func TestRegisterRequestValidate(t *testing.T) {
	bad := minimalRequest()
	bad.TransactionType = "nope"
	if err := bad.Validate(); err == nil || err.Error() != `dpay: Invalid transaction type "nope"` {
		t.Fatalf("err = %v", err)
	}

	bad = minimalRequest()
	bad.URLs.Success = "x"
	if err := bad.Validate(); err == nil || !strings.Contains(err.Error(), "Invalid success URL") {
		t.Fatalf("err = %v", err)
	}

	bad = minimalRequest()
	bad.CurrencyCode = "pln"
	if err := bad.Validate(); err == nil || err.Error() != `dpay: Invalid currency code "pln"` {
		t.Fatalf("err = %v", err)
	}

	bad = minimalRequest()
	bad.PartnerPlatform = String("shopify")
	if err := bad.Validate(); err == nil || err.Error() != "dpay: Partner platform must match ^[A-Z0-9]{1,64}$" {
		t.Fatalf("err = %v", err)
	}

	bad = minimalRequest()
	bad.BlikCode = String("12345")
	if err := bad.Validate(); err == nil || err.Error() != "dpay: BLIK code must be exactly 6 digits" {
		t.Fatalf("err = %v", err)
	}

	bad = minimalRequest()
	bad.AliasIPNURL = String("not-a-url")
	if err := bad.Validate(); err == nil || err.Error() != `dpay: Invalid alias IPN URL "not-a-url"` {
		t.Fatalf("err = %v", err)
	}

	bad = minimalRequest()
	bad.TransactionType = TransactionTypeCardAuth
	bad.Efaktura = Bool(true)
	if err := bad.Validate(); err == nil || err.Error() != `dpay: efaktura is allowed only for transactionType "transfers"` {
		t.Fatalf("err = %v", err)
	}
}

func TestRegisterRequestMutualExclusions(t *testing.T) {
	bad := minimalRequest()
	bad.BlikCode = String("123456")
	bad.BlikAlias = String("alias-1")
	if err := bad.Validate(); err == nil || err.Error() != "dpay: blik_code cannot be combined with blik_alias" {
		t.Fatalf("err = %v", err)
	}

	bad = minimalRequest()
	bad.BlikAlias = String("alias-1")
	bad.RegisterBlikAlias = &BlikAliasRegistration{Label: "L", Type: BlikAliasTypeUID}
	if err := bad.Validate(); err == nil ||
		err.Error() != "dpay: blik_alias cannot be combined with blik_code or alias registration" {
		t.Fatalf("err = %v", err)
	}

	bad = minimalRequest()
	bad.CardRecurring = &CardRecurringRegistration{Label: "M"}
	bad.CardRecurringAlias = String("card-alias")
	if err := bad.Validate(); err == nil ||
		err.Error() != "dpay: register_card_recurring cannot be combined with card_recurring_alias" {
		t.Fatalf("err = %v", err)
	}
}

func TestPaymentsRegister(t *testing.T) {
	server, sent, path := recordingServer(t, 200,
		`{"transactionId":"tx-1","msg":"https://secure.dpay.pl/pay/1","ipksef":"KSEF-1"}`)
	client, _ := New("test_service", "secret_hash", WithBaseURLs(BaseURLs{APIPayments: server.URL}))

	payment, err := client.Payments.Register(context.Background(), minimalRequest())
	if err != nil {
		t.Fatal(err)
	}
	if *path != "/api/v1_0/payments/register" {
		t.Fatalf("path = %q", *path)
	}
	if payment.TransactionID() != "tx-1" || payment.RedirectURL() != "https://secure.dpay.pl/pay/1" {
		t.Fatalf("payment = %+v", payment)
	}
	if payment.IPKSeF() != "KSEF-1" {
		t.Fatalf("IPKSeF = %q", payment.IPKSeF())
	}

	decoded := decodeBody(t, *sent)
	want := expectedRegisterChecksum(t, "test_service", "secret_hash",
		"29.99", "https://shop.test/ok", "https://shop.test/fail", "https://shop.test/ipn")
	if decoded["checksum"] != want {
		t.Fatalf("checksum = %v, want %v", decoded["checksum"], want)
	}
}

func TestPaymentsRegisterRejection(t *testing.T) {
	server, _, _ := recordingServer(t, 200,
		`{"error":true,"msg":"Payment rejected","transactionId":"tx-9","additionalInfo":{"error":"err05"}}`)
	client, _ := New("svc", "hash", WithBaseURLs(BaseURLs{APIPayments: server.URL}))

	_, err := client.Payments.Register(context.Background(), minimalRequest())
	if !errors.Is(err, ErrPaymentRejected) {
		t.Fatalf("err = %v", err)
	}
	var rejected *PaymentRejectedError
	if !errors.As(err, &rejected) {
		t.Fatal("must be a *PaymentRejectedError")
	}
	if rejected.TransactionID != "tx-9" || rejected.ErrorCode != "err05" || rejected.HTTPStatus != 200 {
		t.Fatalf("rejected = %+v", rejected)
	}
}

func TestPaymentsRegisterStatusFalseIsRejection(t *testing.T) {
	server, _, _ := recordingServer(t, 200, `{"status":false,"msg":"nope"}`)
	client, _ := New("svc", "hash", WithBaseURLs(BaseURLs{APIPayments: server.URL}))
	if _, err := client.Payments.Register(context.Background(), minimalRequest()); !errors.Is(err, ErrPaymentRejected) {
		t.Fatalf("err = %v", err)
	}
}

func TestPaymentsRegisterValidationRunsBeforeSending(t *testing.T) {
	client, _ := New("svc", "hash", WithBaseURLs(BaseURLs{APIPayments: "http://127.0.0.1:1"}))
	bad := minimalRequest()
	bad.TransactionType = "nope"
	if _, err := client.Payments.Register(context.Background(), bad); !errors.Is(err, ErrInvalidArgument) {
		t.Fatalf("err = %v, want a validation error and no network call", err)
	}
}

func TestPaymentsDetails(t *testing.T) {
	server, sent, path := recordingServer(t, 200, `{
		"transaction":{"id":"tx-1","value":"29.99","status":"paid","payment_method":"blik",
			"creation_date":"2026-07-01 10:00:00","payment_date":"2026-07-01 10:01:00",
			"settled":true,"refunded":false,"refunded_amount":0,"available_refund_amount":"29.99",
			"fully_refunded":false,"direct":true,"gateway_id":"gw-1"},
		"payer":{"email":"jan@example.com"},
		"refunds":[{"payment_id":"rf-1","value":"5.00","status":"paid"}]
	}`)
	client, _ := New("test_service", "secret_hash", WithBaseURLs(BaseURLs{Panel: server.URL}))

	transaction, err := client.Payments.Details(context.Background(), "tx-1")
	if err != nil {
		t.Fatal(err)
	}
	if *path != "/api/v1/pbl/details" {
		t.Fatalf("path = %q", *path)
	}
	if transaction.ID() != "tx-1" || !transaction.IsPaid() || transaction.Status() != TransactionStatusPaid {
		t.Fatalf("transaction = %+v", transaction)
	}
	if transaction.Value().Minor() != 2999 || transaction.AvailableRefundAmount().Minor() != 2999 {
		t.Fatal("amounts broken")
	}
	if !transaction.IsSettled() || !transaction.IsDirect() || transaction.GatewayID() != "gw-1" {
		t.Fatal("flags broken")
	}
	if transaction.Payer()["email"] != "jan@example.com" {
		t.Fatal("payer lost")
	}
	if len(transaction.Refunds()) != 1 || transaction.Refunds()[0].PaymentID() != "rf-1" {
		t.Fatal("refunds lost")
	}

	decoded := decodeBody(t, *sent)
	if decoded["checksum"] != expectedOrderedChecksum(t, "secret_hash", "test_service", "tx-1") {
		t.Fatalf("checksum = %v", decoded["checksum"])
	}
}

func TestTransactionCapturedCountsAsPaid(t *testing.T) {
	server, _, _ := recordingServer(t, 200, `{"transaction":{"id":"t","status":"captured"}}`)
	client, _ := New("svc", "hash", WithBaseURLs(BaseURLs{Panel: server.URL}))
	transaction, _ := client.Payments.Details(context.Background(), "t")
	if !transaction.IsPaid() {
		t.Fatal("captured must count as paid")
	}
}

func TestPaymentsDetailsMapsHTTPErrors(t *testing.T) {
	server, _, _ := recordingServer(t, 401, `{"message":"Unauthorized request"}`)
	client, _ := New("svc", "hash", WithBaseURLs(BaseURLs{Panel: server.URL}))
	if _, err := client.Payments.Details(context.Background(), "tx-1"); !errors.Is(err, ErrAuthentication) {
		t.Fatalf("err = %v", err)
	}
}

func TestRegisteredPaymentMessageHelpers(t *testing.T) {
	paid := registeredPaymentFromAPI(map[string]any{"msg": "Transaction paid"})
	if !paid.IsPaid() || paid.RedirectURL() != "" {
		t.Fatal("Transaction paid must not look like a redirect")
	}
	inline := registeredPaymentFromAPI(map[string]any{"msg": "Internal processing"})
	if !inline.IsInlineProcessing() {
		t.Fatal("IsInlineProcessing broken")
	}
	alias := registeredPaymentFromAPI(map[string]any{
		"msg":            "Transaction paid",
		"additionalInfo": map[string]any{"card_recurring_alias": "ca-1"},
	})
	if alias.CardRecurringAlias() != "ca-1" {
		t.Fatal("CardRecurringAlias broken")
	}
}
