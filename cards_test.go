package dpay

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func cardDevice() DeviceInfo {
	return DeviceInfo{
		BrowserAcceptHeader: "text/html", BrowserLanguage: "pl-PL", BrowserColorDepth: 24,
		BrowserScreenHeight: 1080, BrowserScreenWidth: 1920, BrowserTZ: -60,
		BrowserUserAgent: "Mozilla/5.0", SystemFamily: "Windows", GeoLocalization: "52.2,21.0",
		DeviceID: "device-abc", ApplicationName: "Sklep",
	}
}

func keysInOrder(t *testing.T, raw string) []string {
	t.Helper()
	decoder := json.NewDecoder(strings.NewReader(raw))
	if _, err := decoder.Token(); err != nil {
		t.Fatal(err)
	}
	var keys []string
	for decoder.More() {
		token, err := decoder.Token()
		if err != nil {
			t.Fatal(err)
		}
		key, ok := token.(string)
		if !ok {
			t.Fatalf("expected a key, got %v", token)
		}
		keys = append(keys, key)
		var discard json.RawMessage
		if err := decoder.Decode(&discard); err != nil {
			t.Fatal(err)
		}
	}
	return keys
}

func TestCardsPublicKeyTrimsWhitespace(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1_0/cards/public-key" {
			t.Errorf("path = %q", r.URL.Path)
		}
		w.Write([]byte("\n-----BEGIN PUBLIC KEY-----\nabc\n-----END PUBLIC KEY-----\n\n"))
	}))
	defer server.Close()

	client, _ := New("svc", "hash", WithBaseURLs(BaseURLs{APIPayments: server.URL}))
	key, err := client.Cards.PublicKey(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if key != "-----BEGIN PUBLIC KEY-----\nabc\n-----END PUBLIC KEY-----" {
		t.Fatalf("key = %q", key)
	}
}

func TestCardPaymentRequestBodyOrder(t *testing.T) {
	request := &CardPaymentRequest{
		DeviceInfo:          cardDevice(),
		Email:               String("jan@example.com"),
		ChannelID:           Int(86),
		CardHolderFirstName: String("Jan"),
		CardHolderLastName:  String("Kowalski"),
		EncryptedCardData:   String("BASE64=="),
		ThreeDSConfirmed:    Bool(true),
		DCCDecision:         DCCDecisionAccept,
	}
	if err := request.Validate(); err != nil {
		t.Fatal(err)
	}
	want := []string{"email", "channelId", "cardHolderFirstName", "cardHolderLastName",
		"encryptedCardData", "deviceInfo", "threeDsConfirmed", "dccDecision"}
	got := request.toBody().Keys()
	if len(got) != len(want) {
		t.Fatalf("keys = %v", got)
	}
	for index := range want {
		if got[index] != want[index] {
			t.Fatalf("key %d = %q, want %q", index, got[index], want[index])
		}
	}
}

func TestCardPaymentRequestMinimalCarriesOnlyDeviceInfo(t *testing.T) {
	request := &CardPaymentRequest{DeviceInfo: cardDevice()}
	if keys := request.toBody().Keys(); len(keys) != 1 || keys[0] != "deviceInfo" {
		t.Fatalf("keys = %v", keys)
	}
}

func TestCardPaymentRequestRejectsBadDCCDecision(t *testing.T) {
	request := &CardPaymentRequest{DeviceInfo: cardDevice(), DCCDecision: "maybe"}
	if err := request.Validate(); err == nil || err.Error() != `dpay: Invalid DCC decision "maybe"` {
		t.Fatalf("err = %v", err)
	}
}

func TestCardsPayOTPAndPreAuth(t *testing.T) {
	for _, testCase := range []struct {
		name string
		call func(*Client, context.Context, *CardPaymentRequest) (*CardPaymentResult, error)
		path string
	}{
		{"payOTP", func(c *Client, ctx context.Context, r *CardPaymentRequest) (*CardPaymentResult, error) {
			return c.Cards.PayOTP(ctx, "tx 1/2", r)
		}, "/api/v1_0/cards/payment/tx%201%2F2/pay/card-otp"},
		{"preAuth", func(c *Client, ctx context.Context, r *CardPaymentRequest) (*CardPaymentResult, error) {
			return c.Cards.PreAuth(ctx, "tx 1/2", r)
		}, "/api/v1_0/cards/payment/tx%201%2F2/pay/card-pre-auth"},
	} {
		server, _, gotPath := recordingServer(t, 200, `{"success":true,"message":{"redirectType":"SUCCESS"}}`)
		client, _ := New("svc", "hash", WithBaseURLs(BaseURLs{APIPayments: server.URL}))

		result, err := testCase.call(client, context.Background(), &CardPaymentRequest{DeviceInfo: cardDevice()})
		if err != nil {
			t.Fatalf("%s: %v", testCase.name, err)
		}
		if *gotPath != testCase.path {
			t.Fatalf("%s path = %q, want %q", testCase.name, *gotPath, testCase.path)
		}
		if !result.IsSuccess() {
			t.Fatalf("%s: result must be a success", testCase.name)
		}
	}
}

func TestCardsCaptureAndCancel(t *testing.T) {
	amount := PLN(2999)
	cases := []struct {
		name     string
		amount   *Money
		wantBody string
		path     string
		call     func(*Client, *Money) error
	}{
		{"capture with amount", &amount, `{"amount":29.99}`, "/api/v1_0/cards/payment/tx-1/capture",
			func(c *Client, m *Money) error {
				_, err := c.Cards.Capture(context.Background(), "tx-1", m)
				return err
			}},
		{"capture full", nil, `{}`, "/api/v1_0/cards/payment/tx-1/capture",
			func(c *Client, m *Money) error {
				_, err := c.Cards.Capture(context.Background(), "tx-1", m)
				return err
			}},
		{"cancel with amount", &amount, `{"amount":29.99}`, "/api/v1_0/cards/payment/tx-1/cancellation",
			func(c *Client, m *Money) error { _, err := c.Cards.Cancel(context.Background(), "tx-1", m); return err }},
		{"cancel full", nil, `{}`, "/api/v1_0/cards/payment/tx-1/cancellation",
			func(c *Client, m *Money) error { _, err := c.Cards.Cancel(context.Background(), "tx-1", m); return err }},
	}
	for _, testCase := range cases {
		server, sent, path := recordingServer(t, 200, `{"success":true,"message":{"redirectType":"SUCCESS"}}`)
		client, _ := New("svc", "hash", WithBaseURLs(BaseURLs{APIPayments: server.URL}))
		if err := testCase.call(client, testCase.amount); err != nil {
			t.Fatalf("%s: %v", testCase.name, err)
		}
		if *sent != testCase.wantBody {
			t.Fatalf("%s body = %s, want %s", testCase.name, *sent, testCase.wantBody)
		}
		if *path != testCase.path {
			t.Fatalf("%s path = %q", testCase.name, *path)
		}
	}
}

func TestCardsGooglePayBody(t *testing.T) {
	server, sent, path := recordingServer(t, 200, `{"success":true,"message":{"redirectType":"SUCCESS"}}`)
	client, _ := New("svc", "hash", WithBaseURLs(BaseURLs{APIPayments: server.URL}))

	if _, err := client.Cards.GooglePay(context.Background(), "tx-1", &GooglePayRequest{
		Token: "gp-token", DeviceInfo: cardDevice(), Email: String("jan@example.com"), ChannelID: Int(86),
	}); err != nil {
		t.Fatal(err)
	}
	if *path != "/api/v1_0/cards/payment/tx-1/pay/google-pay" {
		t.Fatalf("path = %q", *path)
	}
	decoded := decodeBody(t, *sent)
	if decoded["xPayType"] != "GOOGLE_PAY" || decoded["xPayToken"] != "gp-token" {
		t.Fatalf("body = %v", decoded)
	}
	want := []string{"email", "channelId", "xPayType", "xPayToken", "deviceInfo"}
	order := keysInOrder(t, *sent)
	for index := range want {
		if order[index] != want[index] {
			t.Fatalf("key %d = %q, want %q", index, order[index], want[index])
		}
	}
}

func TestCardsApplePayInitAndPay(t *testing.T) {
	server, sent, _ := recordingServer(t, 200, `{"success":true,"message":{"redirectType":"SUCCESS"}}`)
	client, _ := New("svc", "hash", WithBaseURLs(BaseURLs{APIPayments: server.URL}))

	if _, err := client.Cards.ApplePay(context.Background(), "tx-1",
		&ApplePayRequest{DeviceInfo: cardDevice(), Init: true}); err != nil {
		t.Fatal(err)
	}
	decoded := decodeBody(t, *sent)
	if decoded["xPayType"] != "APPLE_PAY_INIT" {
		t.Fatalf("init body = %v", decoded)
	}
	if _, present := decoded["xPayToken"]; present {
		t.Fatal("init must not send a token")
	}

	server, sent, _ = recordingServer(t, 200, `{"success":true,"message":{"redirectType":"SUCCESS"}}`)
	client, _ = New("svc", "hash", WithBaseURLs(BaseURLs{APIPayments: server.URL}))
	if _, err := client.Cards.ApplePay(context.Background(), "tx-1",
		&ApplePayRequest{DeviceInfo: cardDevice(), Token: String("ap-token")}); err != nil {
		t.Fatal(err)
	}
	decoded = decodeBody(t, *sent)
	if decoded["xPayType"] != "APPLE_PAY" || decoded["xPayToken"] != "ap-token" {
		t.Fatalf("pay body = %v", decoded)
	}
}

func TestCardPaymentFailureIsCardPaymentError(t *testing.T) {
	server, _, _ := recordingServer(t, 200, `{"success":false,"message":"DCC_OFFER_EXPIRED"}`)
	client, _ := New("svc", "hash", WithBaseURLs(BaseURLs{APIPayments: server.URL}))

	_, err := client.Cards.Capture(context.Background(), "tx-1", nil)
	if !errors.Is(err, ErrCardPayment) {
		t.Fatalf("err = %v", err)
	}
	var cardErr *CardPaymentError
	if !errors.As(err, &cardErr) {
		t.Fatal("must be a *CardPaymentError")
	}
	if cardErr.HTTPStatus != 200 || cardErr.ErrorCode != "DCC_OFFER_EXPIRED" {
		t.Fatalf("cardErr = %+v - PHP puts the message in the error code", cardErr)
	}
}

func TestCardPaymentResultForms(t *testing.T) {
	html := base64.StdEncoding.EncodeToString([]byte("<form>3ds</form>"))
	server, _, _ := recordingServer(t, 200,
		`{"success":true,"message":{"redirectType":"FORM","redirectText":"`+html+`"}}`)
	client, _ := New("svc", "hash", WithBaseURLs(BaseURLs{APIPayments: server.URL}))
	result, _ := client.Cards.Capture(context.Background(), "tx-1", nil)
	if !result.RequiresThreeDSForm() || result.ThreeDSFormHTML() != "<form>3ds</form>" {
		t.Fatalf("form handling: %q", result.ThreeDSFormHTML())
	}
	if result.RedirectURL() != "" {
		t.Fatal("a FORM result has no redirect URL")
	}

	url := base64.StdEncoding.EncodeToString([]byte("https://3ds.test/redirect"))
	server, _, _ = recordingServer(t, 200,
		`{"success":true,"message":{"redirectType":"URL","redirectText":"`+url+`"}}`)
	client, _ = New("svc", "hash", WithBaseURLs(BaseURLs{APIPayments: server.URL}))
	result, _ = client.Cards.Capture(context.Background(), "tx-1", nil)
	if !result.RequiresRedirect() || result.RedirectURL() != "https://3ds.test/redirect" {
		t.Fatalf("url handling: %q", result.RedirectURL())
	}

	server, _, _ = recordingServer(t, 200,
		`{"success":true,"message":{"redirectType":"URL","redirectText":"!!!not base64!!!"}}`)
	client, _ = New("svc", "hash", WithBaseURLs(BaseURLs{APIPayments: server.URL}))
	result, _ = client.Cards.Capture(context.Background(), "tx-1", nil)
	if result.RedirectURL() != "" {
		t.Fatal("undecodable base64 must yield an empty URL, not garbage")
	}
}

func TestCardPaymentResultDCCOffer(t *testing.T) {
	server, _, _ := recordingServer(t, 200, `{"success":true,"message":{"redirectType":"DCC_OFFER","dccOffer":{
		"currencyConversionId":"cc-1","originalAmount":"29.99","originalCurrency":"PLN",
		"convertedAmount":"6.99","convertedCurrency":"EUR","exchangeRate":4.29,
		"validUntil":"2026-07-22T12:00:00Z","declarationText":"Zgoda PSD2",
		"markup":[{"rate":0.03,"additionalInfo":"prowizja"}],"europeanEconomicArea":true}}}`)
	client, _ := New("svc", "hash", WithBaseURLs(BaseURLs{APIPayments: server.URL}))

	result, _ := client.Cards.Capture(context.Background(), "tx-1", nil)
	if !result.HasDCCOffer() {
		t.Fatal("HasDCCOffer broken")
	}
	offer := result.DCCOffer()
	if offer.CurrencyConversionID() != "cc-1" || offer.ExchangeRate() != 4.29 {
		t.Fatalf("offer = %+v", offer)
	}
	if offer.OriginalAmount().Minor() != 2999 || offer.OriginalAmount().Currency() != CurrencyPLN {
		t.Fatal("original amount broken")
	}
	if offer.ConvertedAmount().Minor() != 699 || offer.ConvertedAmount().Currency() != CurrencyEUR {
		t.Fatal("converted amount must use convertedCurrency")
	}
	if !offer.IsEuropeanEconomicArea() || offer.DeclarationText() != "Zgoda PSD2" {
		t.Fatal("offer metadata broken")
	}
	if len(offer.Markup()) != 1 || offer.Markup()[0].Rate() != 0.03 ||
		offer.Markup()[0].AdditionalInfo() != "prowizja" {
		t.Fatal("markup lost")
	}
	if offer.ValidUntil() != "2026-07-22T12:00:00Z" {
		t.Fatal("validUntil lost")
	}
}
