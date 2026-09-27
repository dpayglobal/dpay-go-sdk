package dpay

import (
	"context"
	"encoding/json"
	"io"
	"math"
	"net/http"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/dpayglobal/dpay-go-sdk/internal/php"
	"github.com/dpayglobal/dpay-go-sdk/internal/wire"
)

type goldenCall struct {
	Method  string            `json:"method"`
	URL     string            `json:"url"`
	Headers map[string]string `json:"headers"`
	Body    *string           `json:"body"`
}

type recordingDoer struct {
	calls     []*http.Request
	bodies    []string
	responses []string
	index     int
}

func (d *recordingDoer) Do(request *http.Request) (*http.Response, error) {
	body := ""
	if request.Body != nil {
		raw, err := io.ReadAll(request.Body)
		if err != nil {
			return nil, err
		}
		body = string(raw)
	}
	d.calls = append(d.calls, request)
	d.bodies = append(d.bodies, body)

	payload := "{}"
	if d.index < len(d.responses) {
		payload = d.responses[d.index]
	}
	d.index++
	return &http.Response{
		StatusCode: 200,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(payload)),
	}, nil
}

func goldenResponses() []string {
	okCard := `{"success":true,"message":{"redirectType":"SUCCESS"}}`
	return []string{
		`{"transactionId":"tx-1","msg":"https://secure.dpay.pl/pay/1"}`,
		`{"transactionId":"tx-2","msg":"Transaction paid"}`,
		`{"transactionId":"tx-3","msg":"Internal processing"}`,
		`{"transaction":{"id":"tx-1","status":"paid","value":"29.99"}}`,
		`{"status":"success","refund":true}`,
		`{"status":"success","refund":true}`,
		`{"refund":true,"message":"ok"}`,
		`{"refund":true,"message":"ok"}`,
		`[{"id":"1","name":"Bank A"}]`,
		`[{"id":"1","name":"Bank A"}]`,
		`{"data":{"alias_value":"alias-1","alias_type":"UID","status":"ACTIVE"}}`,
		`{"data":{}}`,
		`{"data":{}}`,
		`{"data":{"alias":"payid-1","status":"ACTIVE"}}`,
		`-----BEGIN PUBLIC KEY-----`,
		okCard, okCard, okCard, okCard, okCard, okCard, okCard, okCard,
		`{"id":42,"state":1,"net":"100.00"}`,
		`{"error":false,"msg":"Internal processing","status":true,"transactionId":"tx-4"}`,
		`{"status":"success","data":{"transactionId":"tx-4","retry":{"status":"pending","count":1}}}`,
		`{"status":"success","data":{"alias":"SUB-1","status":"UNREGISTERED"}}`,
		`{"status":"success","refund":true}`,
		okCard,
		`{"status":"success","data":[],"has_more":false,"next_starting_after":null}`,
	}
}

func goldenDevice() DeviceInfo {
	return DeviceInfo{
		BrowserAcceptHeader: "text/html",
		BrowserLanguage:     "pl-PL",
		BrowserColorDepth:   24,
		BrowserScreenHeight: 1080,
		BrowserScreenWidth:  1920,
		BrowserTZ:           -60,
		BrowserUserAgent:    "Mozilla/5.0",
		SystemFamily:        "Windows",
		GeoLocalization:     "52.2297,21.0122",
		DeviceID:            "device-abc",
		ApplicationName:     "Sklep Testowy",
		BrowserJavaEnabled:  Bool(true),
	}
}

func mustRegister(t *testing.T, client *Client, ctx context.Context, request *RegisterPaymentRequest) {
	t.Helper()
	if _, err := client.Payments.Register(ctx, request); err != nil {
		t.Fatal(err)
	}
}

func must(t *testing.T, call func() error) {
	t.Helper()
	if err := call(); err != nil {
		t.Fatal(err)
	}
}

func runGoldenScenario(t *testing.T, client *Client) {
	t.Helper()
	ctx := context.Background()
	urls := ReturnURLs{Success: "https://shop.test/ok", Fail: "https://shop.test/fail", IPN: "https://shop.test/ipn"}
	device := goldenDevice()

	limit, total := PLN(50000), PLN(600000)
	vat := PLN(560)

	full := &RegisterPaymentRequest{
		Amount: PLN(2999), TransactionType: TransactionTypeTransfers, URLs: urls,
		Description: String("Zamówienie #1234 / ĄĘŚŻ <b>&</b>"),
		Custom:      String("order-1234"),
		Payer:       &Payer{Email: String("jan@example.com"), FirstName: String("Jan"), LastName: String("Kowalski")},
		AcceptTos:   Bool(true),
		Channel:     String("86"),
		CreditCard:  Bool(true), Paysafecard: Bool(false), Blik: Bool(true),
		Installment: Bool(false), PayPal: Bool(true), NoBanks: Bool(false),
		PhoneNumber: String("+48123456789"), CurrencyCode: CurrencyEUR,
		PartnerPlatform: String("SHOPIFY01"),
		AliasIPNURL:     String("https://shop.test/alias-ipn"),
		NoDelay:         Bool(true), AuthorizeOnly: Bool(false),
		CardRecurringOperation: CardRecurringOperationCharge,
		Payout: &PayoutInstruction{FeeMode: PayoutFeeModeGross, Positions: []PayoutPosition{
			{IBAN: "PL61109010140000071219812874", Title: "Wypłata 1", Amount: PLN(1050)},
		}},
		BillingAddress:  NewFields().Set("street", "Testowa 1").Set("city", "Warszawa"),
		ShippingAddress: NewFields().Set("street", "Inna 2"),
		DeviceInfo:      &device,
		Products:        []any{NewFields().Set("name", "Produkt").Set("price", 29.99)},
		Efaktura:        Bool(true),
		Invoice: &InvoiceDetails{
			PayerNIP: String("1234563218"), PayerName: String("Firma sp. z o.o."),
			InvoiceNumber: String("FV/2026/07/1"), PaymentDueDate: String("2026-08-15"), VatAmount: &vat,
		},
	}
	mustRegister(t, client, ctx, full)

	recurringRegistration := &RegisterPaymentRequest{
		Amount: PLN(1000), TransactionType: TransactionTypeTransfers, URLs: urls,
		UserAgent: String("UA/1.0"), UserIP: String("10.0.0.1"), BlikCode: String("123456"),
		RecurringRegistration: &RecurringRegistration{
			Label: "Subskrypcja", Model: RecurringModelAutomatic, TermsURL: "https://shop.test/regulamin",
			Frequency: String("1M"), LimitAmt: Int(5000), TotLimitAmt: Int(60000),
			LimitAmtFixed: Bool(true), ExpirationDate: String("2027-01-01"), InitDate: String("2026-08-01"),
		},
	}
	mustRegister(t, client, ctx, recurringRegistration)

	cardRecurring := &RegisterPaymentRequest{
		Amount: PLN(500), TransactionType: TransactionTypeCardRecurring, URLs: urls,
		CardRecurring: &CardRecurringRegistration{
			Label: "Mandat", Frequency: CardRecurringFrequencyMonthly,
			LimitAmt: &limit, TotLimitAmt: &total, LimitAmtFixed: Bool(false),
			ExpirationDate: String("2027-01-01"),
		},
	}
	mustRegister(t, client, ctx, cardRecurring)

	must(t, func() error { _, err := client.Payments.Details(ctx, "tx-1"); return err })
	must(t, func() error { _, err := client.Refunds.Create(ctx, "tx-1"); return err })
	must(t, func() error {
		_, err := client.Refunds.Create(ctx, "tx-1", WithRefundAmount(PLN(500)), WithRefundReason("reklamacja"))
		return err
	})
	must(t, func() error { _, err := client.Refunds.CheckAvailability(ctx, "tx-1"); return err })
	must(t, func() error {
		_, err := client.Refunds.CheckAvailability(ctx, "tx-1", WithRefundAmount(PLN(500)), WithRefundReason("reklamacja"))
		return err
	})
	must(t, func() error { _, err := client.Banks.All(ctx); return err })
	must(t, func() error { _, err := client.Banks.ForService(ctx, WithTimestamp(1784700000)); return err })
	must(t, func() error { _, err := client.Blik.Alias(ctx, "alias-1", BlikAliasTypeUID); return err })
	must(t, func() error { return client.Blik.UnregisterAlias(ctx, "alias-1", BlikAliasTypeUID) })
	must(t, func() error {
		return client.Blik.UnregisterAlias(ctx, "alias-1", BlikAliasTypeUID, WithUnregisterReason("na życzenie klienta"))
	})
	must(t, func() error { _, err := client.Recurring.Status(ctx, "payid-1"); return err })
	must(t, func() error { _, err := client.Cards.PublicKey(ctx); return err })

	cardRequest := &CardPaymentRequest{
		DeviceInfo: device, Email: String("jan@example.com"), ChannelID: Int(86),
		CardHolderFirstName: String("Jan"), CardHolderLastName: String("Kowalski"),
		EncryptedCardData: String("BASE64ENCRYPTED=="), ThreeDSConfirmed: Bool(true),
		DCCDecision: DCCDecisionAccept,
	}
	cancelAmount := PLN(1000)
	must(t, func() error { _, err := client.Cards.PayOTP(ctx, "tx-1", cardRequest); return err })
	must(t, func() error {
		_, err := client.Cards.PreAuth(ctx, "tx 1/2", &CardPaymentRequest{DeviceInfo: device})
		return err
	})
	must(t, func() error { _, err := client.Cards.Capture(ctx, "tx-1", PLN(2999)); return err })
	must(t, func() error { _, err := client.Cards.Cancel(ctx, "tx-1", nil); return err })
	must(t, func() error { _, err := client.Cards.Cancel(ctx, "tx-1", &cancelAmount); return err })
	must(t, func() error {
		_, err := client.Cards.GooglePay(ctx, "tx-1", &GooglePayRequest{
			Token: "gp-token", DeviceInfo: device, Email: String("jan@example.com"), ChannelID: Int(86),
		})
		return err
	})
	must(t, func() error {
		_, err := client.Cards.ApplePay(ctx, "tx-1", &ApplePayRequest{DeviceInfo: device, Init: true})
		return err
	})
	must(t, func() error {
		_, err := client.Cards.ApplePay(ctx, "tx-1", &ApplePayRequest{
			DeviceInfo: device, Token: String("ap-token"), ChannelID: Int(86),
		})
		return err
	})
	must(t, func() error { _, err := client.Payouts.Details(ctx, 42, WithTimestamp(1784700000)); return err })

	// 0.2.0: recurring charge without IPN, recurring service, webhook targets and the Events API
	recurringCharge := &RegisterPaymentRequest{
		Amount: PLN(4999), TransactionType: TransactionTypeTransfers,
		URLs:           ReturnURLs{Success: "https://shop.test/ok", Fail: "https://shop.test/fail"},
		RecurringAlias: String("SUB-1"),
		UserAgent:      String("UA/1.0"), UserIP: String("10.0.0.1"),
		Description: String("Abonament 10/2026"),
		Webhook: &WebhookTarget{URL: "https://shop.test/webhooks", Events: []WebhookEventType{
			WebhookEventTypePaymentSucceeded, WebhookEventTypePaymentFailed,
		}},
		Reference: String("order-77"),
	}
	mustRegister(t, client, ctx, recurringCharge)
	must(t, func() error { _, err := client.Recurring.Retry(ctx, "tx-4"); return err })
	must(t, func() error {
		_, err := client.Recurring.Cancel(ctx, "SUB-1", WithCancelReason("Rezygnacja"))
		return err
	})
	must(t, func() error {
		_, err := client.Refunds.Create(ctx, "tx-1", WithRefundAmount(PLN(500)), WithRefundReason("reklamacja"),
			WithRefundWebhook(WebhookTarget{URL: "https://shop.test/webhooks/refunds", Events: []WebhookEventType{
				WebhookEventTypeRefundSucceeded, WebhookEventTypeRefundFailed,
			}}))
		return err
	})
	must(t, func() error {
		_, err := client.Cards.Capture(ctx, "tx-1", PLN(1500), WithCaptureWebhook(WebhookTarget{
			URL: "https://shop.test/webhooks/captures", Events: []WebhookEventType{WebhookEventTypePaymentCaptured},
		}))
		return err
	})
	must(t, func() error {
		_, err := client.Events.List(ctx, &EventListParams{
			Types:       []WebhookEventType{WebhookEventTypePaymentSucceeded, WebhookEventTypeRefundFailed},
			CreatedFrom: String("2026-09-01T00:00:00Z"),
			Limit:       Int(10),
		}, WithTimestamp(1784700000))
		return err
	})
}

func TestRequestParityWithPHPSDK(t *testing.T) {
	raw, err := os.ReadFile("testdata/golden_requests.json")
	if err != nil {
		t.Fatalf("brakuje golden vectors w testdata - sa czescia repo, patrz docs/golden/README.md: %v", err)
	}
	var golden []goldenCall
	if err := json.Unmarshal(raw, &golden); err != nil {
		t.Fatal(err)
	}

	doer := &recordingDoer{responses: goldenResponses()}
	client, err := New("test_service", "secret_hash", WithHTTPClient(doer))
	if err != nil {
		t.Fatal(err)
	}
	runGoldenScenario(t, client)

	if len(doer.calls) != len(golden) {
		t.Fatalf("recorded %d calls, golden has %d", len(doer.calls), len(golden))
	}
	matched := 0
	for index, want := range golden {
		got := doer.calls[index]
		failed := false
		if got.Method != want.Method {
			t.Errorf("call %d method = %q, want %q", index, got.Method, want.Method)
			failed = true
		}
		if got.URL.String() != want.URL {
			t.Errorf("call %d URL = %q, want %q", index, got.URL.String(), want.URL)
			failed = true
		}
		for name, wantValue := range want.Headers {
			if name == "User-Agent" {
				continue
			}
			if gotValue := got.Header.Get(name); gotValue != wantValue {
				t.Errorf("call %d header %s = %q, want %q", index, name, gotValue, wantValue)
				failed = true
			}
		}
		wantBody := ""
		if want.Body != nil {
			wantBody = *want.Body
		}
		if doer.bodies[index] != wantBody {
			t.Errorf("call %d body mismatch\n got: %s\nwant: %s", index, doer.bodies[index], wantBody)
			failed = true
		}
		if !failed {
			matched++
		}
	}
	t.Logf("byte-identical calls: %d/%d", matched, len(golden))
}

type goldenHelpers struct {
	Checksums map[string]string `json:"checksums"`
	IPN       []struct {
		Body      string `json:"body"`
		Signature string `json:"signature"`
	} `json:"ipn"`
	Strval []struct {
		JSON   string `json:"json"`
		Strval string `json:"strval"`
	} `json:"strval"`
	MoneyToDecimal map[string]string `json:"money_to_decimal"`
	FloatCast      map[string]string `json:"float_cast"`
	CardPayload    string            `json:"card_payload"`
}

func TestHelperParityWithPHPSDK(t *testing.T) {
	raw, err := os.ReadFile("testdata/golden_helpers.json")
	if err != nil {
		t.Fatalf("brakuje golden vectors w testdata - sa czescia repo, patrz docs/golden/README.md: %v", err)
	}
	var helpers goldenHelpers
	if err := json.Unmarshal(raw, &helpers); err != nil {
		t.Fatal(err)
	}

	checksum := wire.NewChecksum("secret_hash")
	if got := checksum.SecretSecond("test_service", nil); got != helpers.Checksums["secret_second_empty"] {
		t.Errorf("secret_second_empty = %s, want %s", got, helpers.Checksums["secret_second_empty"])
	}
	if got := checksum.SecretSecond("test_service", []any{"29.99", 10, true, 10.0}); got != helpers.Checksums["secret_second_mixed"] {
		t.Errorf("secret_second_mixed = %s, want %s", got, helpers.Checksums["secret_second_mixed"])
	}
	simple := wire.NewBody()
	simple.Set("service", "test_service")
	simple.Set("transaction_id", "tx-1")
	if got := checksum.OrderedBody(simple); got != helpers.Checksums["ordered_simple"] {
		t.Errorf("ordered_simple = %s", got)
	}
	mixed := wire.NewBody()
	mixed.Set("service", "test_service")
	mixed.Set("timestamp", int64(1784700000))
	mixed.Set("withdraw_id", int64(42))
	mixed.Set("value", "5.00")
	mixed.Set("reason", "reklamacja")
	if got := checksum.OrderedBody(mixed); got != helpers.Checksums["ordered_mixed"] {
		t.Errorf("ordered_mixed = %s", got)
	}

	for index, vector := range helpers.IPN {
		event, err := VerifyIPN([]byte(vector.Body), "secret_hash")
		if err != nil {
			t.Errorf("ipn %d: %v", index, err)
			continue
		}
		if event.Signature() != vector.Signature {
			t.Errorf("ipn %d signature mismatch", index)
		}
	}

	for minor, want := range helpers.MoneyToDecimal {
		parsed, err := strconv.ParseInt(minor, 10, 64)
		if err != nil {
			t.Fatal(err)
		}
		if got := PLN(parsed).String(); got != want {
			t.Errorf("PLN(%s).String() = %q, want %q", minor, got, want)
		}
	}

	for decimal, want := range helpers.FloatCast {
		money, err := ParseMoney(decimal, CurrencyPLN)
		if err != nil {
			t.Fatal(err)
		}
		body := wire.NewBody()
		body.Set("amount", moneyFloat(money))
		encoded, encodeErr := php.Encode(body, false)
		if encodeErr != nil {
			t.Fatal(encodeErr)
		}
		if string(encoded) != want {
			t.Errorf("float cast %s = %s, want %s", decimal, encoded, want)
		}
	}

	payload, err := cardPayload(CardData{PAN: "4111111111111111", CVV: "123", Expiry: "12/25"}, "tx-1", 1784700000)
	if err != nil {
		t.Fatal(err)
	}
	if string(payload) != helpers.CardPayload {
		t.Errorf("cardPayload = %s, want %s", payload, helpers.CardPayload)
	}

	for index, vector := range helpers.Strval {
		var value any
		if err := json.Unmarshal([]byte(vector.JSON), &value); err != nil {
			t.Fatal(err)
		}
		if got := php.Strval(normalizeGoldenNumber(value, vector.JSON)); got != vector.Strval {
			t.Errorf("strval %d (%s) = %q, want %q", index, vector.JSON, got, vector.Strval)
		}
	}
}

func normalizeGoldenNumber(value any, raw string) any {
	number, ok := value.(float64)
	if !ok {
		return value
	}
	if math.Signbit(number) && number == 0 {
		return number
	}
	if !strings.ContainsAny(raw, ".eE") {
		return int64(number)
	}
	return number
}
