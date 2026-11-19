package dpay

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
)

func mustMoney(t *testing.T, minor int64, currency Currency) Money {
	t.Helper()
	money, err := NewMoney(minor, currency)
	if err != nil {
		t.Fatal(err)
	}
	return money
}

func urlsWithIPN() ReturnURLs {
	return ReturnURLs{Success: "https://shop.example/ok", Fail: "https://shop.example/fail", IPN: "https://shop.example/ipn"}
}

func mbWayDevice(javaEnabled *bool) *DeviceInfo {
	return &DeviceInfo{
		BrowserAcceptHeader: "text/html", BrowserLanguage: "pt-PT", BrowserColorDepth: 24, BrowserScreenHeight: 1200,
		BrowserScreenWidth: 1920, BrowserTZ: -60, BrowserUserAgent: "Mozilla/5.0", SystemFamily: "Win32",
		GeoLocalization: "38.72,-9.14", DeviceID: "device-1", ApplicationName: "Netscape", BrowserJavaEnabled: javaEnabled,
	}
}

func mbWayRequest(t *testing.T) *RegisterPaymentRequest {
	return &RegisterPaymentRequest{
		Amount: mustMoney(t, 500, CurrencyEUR), TransactionType: TransactionTypeMBWayDirect, URLs: urlsWithIPN(),
		PhoneNumber: String("+351912345678"), CurrencyCode: CurrencyEUR,
		Channel:    String("373"),
		Payer:      &Payer{Email: String("client@example.com"), FirstName: String("Joao"), LastName: String("Silva")},
		AcceptTos:  Bool(true),
		DeviceInfo: mbWayDevice(Bool(false)),
	}
}

func assertValidation(t *testing.T, err error, fragment string) {
	t.Helper()
	if !errors.Is(err, ErrInvalidArgument) || !strings.Contains(err.Error(), fragment) {
		t.Fatalf("err = %v, want a validation error containing %q", err, fragment)
	}
}

func TestMBWayRegistrationBody(t *testing.T) {
	body, err := mbWayRequest(t).toBody("MyShop")
	if err != nil {
		t.Fatal(err)
	}
	got := bodyJSON(t, body)
	for _, fragment := range []string{`"phone_number":"+351912345678"`, `"currency_code":"EUR"`, `"channel":"373"`, `"browserJavaEnabled":"false"`, `"value":"5.00"`} {
		if !strings.Contains(got, fragment) {
			t.Fatalf("missing %s in %s", fragment, got)
		}
	}
}

func TestNonPLNAmountSendsItsCurrency(t *testing.T) {
	request := &RegisterPaymentRequest{Amount: mustMoney(t, 49900, CurrencyCZK), TransactionType: TransactionTypeTransfers, URLs: urlsWithIPN()}
	body, err := request.toBody("MyShop")
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"service", "value", "transactionType", "url_success", "url_fail", "url_ipn", "currency_code"}; !reflect.DeepEqual(body.Keys(), want) {
		t.Fatalf("keys = %v", body.Keys())
	}
	if value, _ := body.Get("value"); value != "499.00" {
		t.Fatalf("value = %v", value)
	}
	if code, _ := body.Get("currency_code"); code != "CZK" {
		t.Fatalf("currency_code = %v", code)
	}

	ron := &RegisterPaymentRequest{Amount: mustMoney(t, 100, CurrencyRON), TransactionType: TransactionTypeTransfers, URLs: urlsWithIPN()}
	body, _ = ron.toBody("S")
	if code, _ := body.Get("currency_code"); code != "RON" || CurrencyUSD != "USD" || CurrencyRON != "RON" {
		t.Fatalf("currency_code = %v", code)
	}
}

func TestPLNAmountSendsNoCurrencyUnlessAskedTo(t *testing.T) {
	plain := &RegisterPaymentRequest{Amount: PLN(1000), TransactionType: TransactionTypeTransfers, URLs: urlsWithIPN()}
	body, _ := plain.toBody("MyShop")
	if _, present := body.Get("currency_code"); present {
		t.Fatal("a PLN amount keeps the 0.2.0 body")
	}
	plain.CurrencyCode = CurrencyPLN
	body, _ = plain.toBody("MyShop")
	if code, _ := body.Get("currency_code"); code != "PLN" {
		t.Fatalf("currency_code = %v", code)
	}
}

func TestCurrencyCodeMustMatchTheAmount(t *testing.T) {
	request := &RegisterPaymentRequest{Amount: PLN(500), TransactionType: TransactionTypeTransfers, URLs: urlsWithIPN(), CurrencyCode: CurrencyEUR}
	err := request.Validate()
	if !errors.Is(err, ErrInvalidArgument) ||
		err.Error() != `dpay: Currency code "EUR" does not match the amount currency "PLN" - create the amount with dpay.NewMoney(..., "EUR")` {
		t.Fatalf("err = %v", err)
	}
	// MB WAY with a PLN amount and the EUR phone currency (the 0.2.0 way) is refused
	phone := &RegisterPaymentRequest{Amount: PLN(500), TransactionType: TransactionTypeMBWayDirect, URLs: urlsWithIPN(),
		PhoneNumber: String("+351912345678"), CurrencyCode: CurrencyEUR}
	assertValidation(t, phone.Validate(), `does not match the amount currency "PLN"`)
}

func TestPhoneNumberLength(t *testing.T) {
	for _, phone := range []string{"+3519123", "+351912345678901"} {
		request := mbWayRequest(t)
		request.PhoneNumber = String(phone)
		if err := request.Validate(); !errors.Is(err, ErrInvalidArgument) || err.Error() != "dpay: Phone number must be 9-15 characters" {
			t.Errorf("%q: err = %v", phone, err)
		}
	}
}

func TestMBWayRequiresAChannelAndBrowserJavaEnabled(t *testing.T) {
	withoutChannel := mbWayRequest(t)
	withoutChannel.Channel = nil
	assertValidation(t, withoutChannel.Validate(), "requires a channel")

	emptyChannel := mbWayRequest(t)
	emptyChannel.Channel = String("")
	assertValidation(t, emptyChannel.Validate(), "requires a channel")

	withoutJava := mbWayRequest(t)
	withoutJava.DeviceInfo = mbWayDevice(nil)
	assertValidation(t, withoutJava.Validate(), "browserJavaEnabled")

	withoutDevice := mbWayRequest(t)
	withoutDevice.DeviceInfo = nil
	assertValidation(t, withoutDevice.Validate(), "browserJavaEnabled")
}

func TestChannelRequiresPayerAndTerms(t *testing.T) {
	direct := func() *RegisterPaymentRequest {
		return &RegisterPaymentRequest{Amount: PLN(1000), TransactionType: TransactionTypeTransfers, URLs: urlsWithIPN(),
			Channel: String("24900005"), AcceptTos: Bool(true),
			Payer: &Payer{Email: String("client@example.com"), FirstName: String("Jan"), LastName: String("Kowalski")}}
	}
	withoutPayer := direct()
	withoutPayer.Payer = nil
	assertValidation(t, withoutPayer.Validate(), "A direct channel requires the payer's email, first name and last name (Payer)")

	withoutName := direct()
	withoutName.Payer = &Payer{Email: String("client@example.com")}
	assertValidation(t, withoutName.Validate(), "first name")

	emptyName := direct()
	emptyName.Payer.LastName = String("")
	assertValidation(t, emptyName.Validate(), "last name")

	withoutEmail := direct()
	withoutEmail.Payer.Email = nil
	assertValidation(t, withoutEmail.Validate(), "email")

	withoutTerms := direct()
	withoutTerms.AcceptTos = nil
	assertValidation(t, withoutTerms.Validate(), "A direct channel requires the terms acceptance (AcceptTos)")

	// Laravel required_with + boolean accepts false
	declined := direct()
	declined.AcceptTos = Bool(false)
	body, err := declined.toBody("MyShop")
	if err != nil {
		t.Fatal(err)
	}
	if accepted, _ := body.Get("accept_tos"); accepted != false {
		t.Fatalf("accept_tos = %v", accepted)
	}
}

func TestForbiddenDirectChannels(t *testing.T) {
	for _, channel := range []string{"tester", "paypal", "installment", "paysafecard"} {
		request := &RegisterPaymentRequest{Amount: PLN(1000), TransactionType: TransactionTypeTransfers, URLs: urlsWithIPN(), Channel: String(channel)}
		err := request.Validate()
		if !errors.Is(err, ErrInvalidArgument) || err.Error() != `dpay: Channel "`+channel+`" cannot be used as a direct payment channel` {
			t.Errorf("%s: err = %v", channel, err)
		}
	}
}

// The API turns "" into null (ConvertEmptyStringsToNull), so it needs no payer or terms - sent as in 0.2.0.
func TestEmptyChannelMeansNoDirectChannel(t *testing.T) {
	request := &RegisterPaymentRequest{Amount: PLN(1000), TransactionType: TransactionTypeTransfers, URLs: urlsWithIPN(), Channel: String("")}
	body, err := request.toBody("MyShop")
	if err != nil {
		t.Fatal(err)
	}
	if channel, present := body.Get("channel"); !present || channel != "" {
		t.Fatalf("channel = %v, %v", channel, present)
	}

	// nor is it a channel that conflicts with a recurring registration
	recurring := &RegisterPaymentRequest{Amount: PLN(0), TransactionType: TransactionTypeTransfers, URLs: urlsWithIPN(),
		Channel: String(""), BlikCode: String("777123"), UserAgent: String("UA"), UserIP: String("1.2.3.4"),
		RecurringRegistration: &RecurringRegistration{Label: "Abonament", Model: RecurringModelOnDemand, TermsURL: "https://shop.example/terms"}}
	if err := recurring.Validate(); err != nil {
		t.Fatalf("empty channel with recurring_registration: %v", err)
	}
	recurring.Channel = String("86")
	assertValidation(t, recurring.Validate(), "channel cannot be combined with a recurring payment")
}

func TestAliasIPNURLIsAtMost500Characters(t *testing.T) {
	url := "https://shop.example/" + strings.Repeat("a", 479)
	if len(url) != 500 {
		t.Fatalf("len = %d", len(url))
	}
	request := &RegisterPaymentRequest{Amount: PLN(1000), TransactionType: TransactionTypeTransfers, URLs: urlsWithIPN(), AliasIPNURL: String(url)}
	if err := request.Validate(); err != nil {
		t.Fatal(err)
	}
	request.AliasIPNURL = String(url + "a")
	assertValidation(t, request.Validate(), "Invalid alias IPN URL")
}

func TestEfakturaCannotBeCombinedWithAliasesOrRecurring(t *testing.T) {
	cases := []struct {
		field     string
		configure func(*RegisterPaymentRequest)
	}{
		{"blik_code", func(r *RegisterPaymentRequest) {
			r.BlikCode, r.UserAgent, r.UserIP = String("777123"), String("UA"), String("1.2.3.4")
		}},
		{"blik_alias", func(r *RegisterPaymentRequest) {
			r.BlikAlias, r.UserAgent, r.UserIP = String("DPAY.UID.1.a"), String("UA"), String("1.2.3.4")
		}},
		{"register_blik_alias", func(r *RegisterPaymentRequest) {
			r.RegisterBlikAlias = &BlikAliasRegistration{Label: "Sklep", Type: BlikAliasTypeUID}
		}},
		{"recurring_alias", func(r *RegisterPaymentRequest) { r.RecurringAlias = String("SUB-0001") }},
		{"register_card_recurring", func(r *RegisterPaymentRequest) { r.CardRecurring = &CardRecurringRegistration{Label: "Mandat"} }},
		{"card_recurring_alias", func(r *RegisterPaymentRequest) { r.CardRecurringAlias = String("card-1") }},
	}
	for _, testCase := range cases {
		request := &RegisterPaymentRequest{Amount: PLN(1000), TransactionType: TransactionTypeTransfers, URLs: urlsWithIPN(), Efaktura: Bool(true)}
		testCase.configure(request)
		err := request.Validate()
		if !errors.Is(err, ErrInvalidArgument) || err.Error() != "dpay: efaktura cannot be combined with "+testCase.field {
			t.Errorf("%s: err = %v", testCase.field, err)
		}
	}
}

func TestPayoutPositionRequiresAValidPolishIBAN(t *testing.T) {
	spaced := PayoutPosition{IBAN: "pl61 1090 1014 0000 0712 1981 2874", Title: "FV 1", Amount: PLN(100)}
	if err := spaced.Validate(); err != nil {
		t.Fatal(err)
	}
	if got := bodyJSON(t, spaced.toBody()); !strings.Contains(got, `"iban":"pl61 1090 1014 0000 0712 1981 2874"`) {
		t.Fatalf("the IBAN is sent as given: %s", got)
	}
	if !IsValidPolishIBAN("PL61109010140000071219812874") {
		t.Fatal("valid IBAN refused")
	}
	for _, iban := range []string{"PL61109010140000071219812875", "DE89370400440532013000", "61109010140000071219812874", ""} {
		position := PayoutPosition{IBAN: iban, Title: "FV 1", Amount: PLN(100)}
		assertValidation(t, position.Validate(), "valid Polish IBAN")
		if IsValidPolishIBAN(iban) {
			t.Errorf("%q accepted", iban)
		}
	}
	for _, amount := range []Money{PLN(0), PLN(-100)} {
		position := PayoutPosition{IBAN: "PL61109010140000071219812874", Title: "FV 1", Amount: amount}
		if err := position.Validate(); err == nil || err.Error() != "dpay: Payout amount must be above 0" {
			t.Errorf("%v: err = %v", amount, err)
		}
	}
}

func TestRegisterPaidReturnsThePaidTransaction(t *testing.T) {
	client, _ := scriptedClient(t, okResponse(`{"error":false,"msg":"Transaction paid","status":true,
		"transactionId":"30D9493D-1D73-3FBD-A5D4-633723CC7A68",
		"additionalInfo":{"transaction":{"payment_id":"30D9493D-1D73-3FBD-A5D4-633723CC7A68","pid":null,"mtid":null,
			"email":"client@example.com","description":"Zamowienie #1234","name":"Jan","surname":"Kowalski",
			"value":"10.00","status":"paid","creation_date":"2026-11-18 10:00:00","payment_date":"2026-11-18 10:01:00"}}}`))
	payment, err := client.Payments.Register(context.Background(), minimalRequest())
	if err != nil {
		t.Fatal(err)
	}
	transaction := payment.PaidTransaction()
	if transaction == nil {
		t.Fatal("PaidTransaction() = nil")
	}
	if transaction.PaymentID() != "30D9493D-1D73-3FBD-A5D4-633723CC7A68" || transaction.Status() != RegisteredTransactionStatusPaid || !transaction.IsPaid() {
		t.Fatalf("transaction = %+v", transaction.Raw())
	}
	if value, present := transaction.Value(); !present || !value.Equal(PLN(1000)) {
		t.Fatalf("value = %v", value)
	}
	if transaction.Email() != "client@example.com" || transaction.Name() != "Jan" || transaction.Surname() != "Kowalski" ||
		transaction.PID() != "" || transaction.MTID() != "" || transaction.Description() != "Zamowienie #1234" ||
		transaction.CreationDate() != "2026-11-18 10:00:00" || transaction.PaymentDate() != "2026-11-18 10:01:00" {
		t.Fatalf("transaction = %+v", transaction.Raw())
	}
	if payment.Multibanco() != nil {
		t.Fatal("no multibanco in this answer")
	}
	captured := registeredPaymentFromAPI(map[string]any{"additionalInfo": map[string]any{
		"transaction": map[string]any{"status": "captured", "value": 12.5}}}, CurrencyEUR).PaidTransaction()
	if value, _ := captured.Value(); !captured.IsPaid() || !value.Equal(mustMoney(t, 1250, CurrencyEUR)) {
		t.Fatalf("captured = %v", value)
	}
}

func TestRegisterMultibancoReturnsTheReference(t *testing.T) {
	client, doer := scriptedClient(t, okResponse(`{"error":false,"msg":"https://secure.dpay.pl/transfer@pay@ABC","status":true,
		"transactionId":"ABC","multibanco":{"entity":"12345","reference":"123456789","amount":"12.50","currency":"EUR",
		"expires_at":"2026-11-21T10:00:00+00:00"}}`))
	payment, err := client.Payments.Register(context.Background(), &RegisterPaymentRequest{
		Amount: mustMoney(t, 1250, CurrencyEUR), TransactionType: TransactionTypeTransfers,
		URLs:    ReturnURLs{Success: "https://shop.example/ok", Fail: "https://shop.example/fail"},
		Channel: String("390"), AcceptTos: Bool(true),
		Payer: &Payer{Email: String("client@example.com"), FirstName: String("Joao"), LastName: String("Silva")},
	})
	if err != nil {
		t.Fatal(err)
	}
	if doer.lastBody(t)["currency_code"] != "EUR" {
		t.Fatal("currency_code must follow the amount")
	}
	multibanco := payment.Multibanco()
	if multibanco == nil {
		t.Fatal("Multibanco() = nil")
	}
	amount, present := multibanco.Amount()
	if multibanco.Entity() != "12345" || multibanco.Reference() != "123456789" || !present ||
		!amount.Equal(mustMoney(t, 1250, CurrencyEUR)) || multibanco.Currency() != CurrencyEUR ||
		multibanco.ExpiresAt() != "2026-11-21T10:00:00+00:00" {
		t.Fatalf("multibanco = %+v", multibanco.Raw())
	}
	if payment.PaidTransaction() != nil {
		t.Fatal("no paid transaction in this answer")
	}
	sparse := registeredPaymentFromAPI(map[string]any{"multibanco": map[string]any{"entity": 12345, "currency": "x"}}, CurrencyPLN).Multibanco()
	if _, present := sparse.Amount(); sparse.Entity() != "12345" || sparse.Reference() != "" || sparse.Currency() != CurrencyEUR || present {
		t.Fatalf("sparse = %+v", sparse.Raw())
	}
}

func TestTransactionFeesAndURLs(t *testing.T) {
	transaction := transactionFromAPI(map[string]any{"transaction": map[string]any{
		"id": "x", "rate": float64(2), "minimal_fee": float64(20), "permanent_fee": float64(0),
		"urls": map[string]any{"success": "https://shop.example/ok", "fail": "https://shop.example/fail", "ipn": "https://shop.example/ipn"},
	}})
	if transaction.Rate() != "2" || *transaction.MinimalFee() != 20 || *transaction.PermanentFee() != 0 {
		t.Fatalf("fees = %q %v %v", transaction.Rate(), transaction.MinimalFee(), transaction.PermanentFee())
	}
	urls := transaction.URLs()
	if urls == nil || urls.Success() != "https://shop.example/ok" || urls.Fail() != "https://shop.example/fail" || urls.IPN() != "https://shop.example/ipn" {
		t.Fatalf("urls = %+v", urls)
	}

	fromStrings := transactionFromAPI(map[string]any{"transaction": map[string]any{
		"rate": "1.49", "minimal_fee": "30",
		"urls": map[string]any{"success": "https://shop.example/ok", "fail": "https://shop.example/fail", "ipn": nil},
	}})
	if fromStrings.Rate() != "1.49" || *fromStrings.MinimalFee() != 30 || fromStrings.PermanentFee() != nil || fromStrings.URLs().IPN() != "" {
		t.Fatal("database strings broken")
	}
	odd := transactionFromAPI(map[string]any{"transaction": map[string]any{"rate": true, "minimal_fee": "2.5", "permanent_fee": "-3"}})
	if odd.Rate() != "" || odd.MinimalFee() != nil || *odd.PermanentFee() != -3 {
		t.Fatal("only scalars give a rate and only integers a fee")
	}

	missing := transactionFromAPI(map[string]any{"transaction": map[string]any{"id": "x", "value": "1.00", "status": "created"}})
	if missing.Rate() != "" || missing.MinimalFee() != nil || missing.URLs() != nil {
		t.Fatal("missing fees and urls must be empty")
	}
}
