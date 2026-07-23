package dpay

import (
	"strings"
	"testing"

	"github.com/dpayglobal/dpay-go-sdk/internal/php"
)

func bodyJSON(t *testing.T, value any) string {
	t.Helper()
	encoded, err := php.Encode(value, false)
	if err != nil {
		t.Fatal(err)
	}
	return string(encoded)
}

func TestFieldsPreserveOrder(t *testing.T) {
	fields := NewFields().Set("street", "Testowa 1").Set("city", "Warszawa")
	if got := bodyJSON(t, fields); got != `{"street":"Testowa 1","city":"Warszawa"}` {
		t.Fatalf("Fields = %s", got)
	}
}

func TestReturnURLsValidate(t *testing.T) {
	valid := ReturnURLs{Success: "https://a/ok", Fail: "https://a/fail", IPN: "https://a/ipn"}
	if err := valid.Validate(); err != nil {
		t.Fatalf("valid URLs rejected: %v", err)
	}
	bad := ReturnURLs{Success: "not a url", Fail: "https://a/fail", IPN: "https://a/ipn"}
	err := bad.Validate()
	if err == nil || err.Error() != `dpay: Invalid success URL "not a url"` {
		t.Fatalf("err = %v", err)
	}
	bad = ReturnURLs{Success: "https://a/ok", Fail: "https://a/fail", IPN: ""}
	if err := bad.Validate(); err == nil || !strings.Contains(err.Error(), "Invalid ipn URL") {
		t.Fatalf("err = %v", err)
	}
}

func TestPayerValidate(t *testing.T) {
	valid := Payer{Email: String("jan@example.com")}
	if err := valid.Validate(); err != nil {
		t.Fatal(err)
	}
	bad := Payer{Email: String("nope")}
	if err := bad.Validate(); err == nil || err.Error() != `dpay: Invalid email "nope"` {
		t.Fatalf("err = %v", err)
	}
	if err := (Payer{}).Validate(); err != nil {
		t.Fatalf("empty payer must be valid: %v", err)
	}
}

func TestDeviceInfoBodyOrderAndJavaFlag(t *testing.T) {
	device := DeviceInfo{
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
	if err := device.Validate(); err != nil {
		t.Fatal(err)
	}
	want := `{"browserAcceptHeader":"text/html","browserLanguage":"pl-PL","browserColorDepth":24,` +
		`"browserScreenHeight":1080,"browserScreenWidth":1920,"browserTZ":-60,"browserUserAgent":"Mozilla/5.0",` +
		`"systemFamily":"Windows","geoLocalization":"52.2297,21.0122","deviceID":"device-abc",` +
		`"applicationName":"Sklep Testowy","browserJavaEnabled":"true"}`
	if got := bodyJSON(t, device.toBody()); got != want {
		t.Fatalf("toBody() = %s\nwant       = %s", got, want)
	}

	device.BrowserJavaEnabled = Bool(false)
	if got := bodyJSON(t, device.toBody()); !strings.Contains(got, `"browserJavaEnabled":"false"`) {
		t.Fatalf("false must serialize as the string \"false\": %s", got)
	}
	device.BrowserJavaEnabled = nil
	if got := bodyJSON(t, device.toBody()); strings.Contains(got, "browserJavaEnabled") {
		t.Fatalf("nil must omit the field: %s", got)
	}
}

func TestDeviceInfoValidate(t *testing.T) {
	device := DeviceInfo{DeviceID: "", ApplicationName: "app"}
	if err := device.Validate(); err == nil || err.Error() != "dpay: Device ID must be 1-64 characters" {
		t.Fatalf("err = %v", err)
	}
	device = DeviceInfo{DeviceID: strings.Repeat("x", 65), ApplicationName: "app"}
	if err := device.Validate(); err == nil {
		t.Fatal("65 characters must fail")
	}
	device = DeviceInfo{DeviceID: "d", ApplicationName: ""}
	if err := device.Validate(); err == nil || err.Error() != "dpay: Application name must be 1-64 characters" {
		t.Fatalf("err = %v", err)
	}
	device = DeviceInfo{DeviceID: strings.Repeat("ą", 64), ApplicationName: strings.Repeat("ę", 64)}
	if err := device.Validate(); err != nil {
		t.Fatalf("64 runes must pass, length is counted in runes: %v", err)
	}
}

func TestInvoiceDetailsBody(t *testing.T) {
	vat := PLN(560)
	invoice := InvoiceDetails{
		PayerNIP:       String("1234563218"),
		PayerName:      String("Firma sp. z o.o."),
		InvoiceNumber:  String("FV/2026/07/1"),
		PaymentDueDate: String("2026-08-15"),
		VatAmount:      &vat,
	}
	if err := invoice.Validate(); err != nil {
		t.Fatal(err)
	}
	want := `{"payer_nip":"1234563218","payer_name":"Firma sp. z o.o.","invoice_number":"FV/2026/07/1",` +
		`"payment_due_date":"2026-08-15","vat_amount":560}`
	if got := bodyJSON(t, invoice.toBody()); got != want {
		t.Fatalf("toBody() = %s", got)
	}
	if got := bodyJSON(t, (InvoiceDetails{}).toBody()); got != `{}` {
		t.Fatalf("empty invoice = %s", got)
	}
	bad := InvoiceDetails{PaymentDueDate: String("15-08-2026")}
	if err := bad.Validate(); err == nil || err.Error() != "dpay: Payment due date must be in YYYY-MM-DD format" {
		t.Fatalf("err = %v", err)
	}
}

func TestPayoutInstructionBody(t *testing.T) {
	instruction := PayoutInstruction{
		FeeMode: PayoutFeeModeGross,
		Positions: []PayoutPosition{
			{IBAN: "PL61109010140000071219812874", Title: "Wypłata 1", Amount: PLN(1050)},
		},
	}
	if err := instruction.Validate(); err != nil {
		t.Fatal(err)
	}
	want := `{"fee_mode":"gross","positions":[{"iban":"PL61109010140000071219812874","title":"Wypłata 1","amount":10.5}]}`
	if got := bodyJSON(t, instruction.toBody()); got != want {
		t.Fatalf("toBody() = %s\nwant       = %s", got, want)
	}
}

func TestPayoutInstructionValidate(t *testing.T) {
	empty := PayoutInstruction{FeeMode: PayoutFeeModeNet}
	if err := empty.Validate(); err == nil || err.Error() != "dpay: Payout instruction requires at least one position" {
		t.Fatalf("err = %v", err)
	}
	badIBAN := PayoutInstruction{FeeMode: PayoutFeeModeNet, Positions: []PayoutPosition{{Title: "t", Amount: PLN(1)}}}
	if err := badIBAN.Validate(); err == nil || err.Error() != "dpay: Payout IBAN must not be empty" {
		t.Fatalf("err = %v", err)
	}
	badTitle := PayoutInstruction{FeeMode: PayoutFeeModeNet, Positions: []PayoutPosition{{IBAN: "PL1", Amount: PLN(1)}}}
	if err := badTitle.Validate(); err == nil || err.Error() != "dpay: Payout title must be 1-255 characters" {
		t.Fatalf("err = %v", err)
	}
	badMode := PayoutInstruction{FeeMode: "x", Positions: []PayoutPosition{{IBAN: "PL1", Title: "t", Amount: PLN(1)}}}
	if err := badMode.Validate(); err == nil || err.Error() != `dpay: Invalid payout fee mode "x"` {
		t.Fatalf("err = %v", err)
	}
}

func TestBlikAliasRegistrationBody(t *testing.T) {
	registration := BlikAliasRegistration{Label: "Mój sklep", Type: BlikAliasTypeUID}
	if err := registration.Validate(); err != nil {
		t.Fatal(err)
	}
	if got := bodyJSON(t, registration.toBody()); got != `{"label":"Mój sklep","type":"UID"}` {
		t.Fatalf("toBody() = %s", got)
	}
	bad := BlikAliasRegistration{Label: strings.Repeat("x", 51), Type: BlikAliasTypeUID}
	if err := bad.Validate(); err == nil || err.Error() != "dpay: Alias label must be 1-50 characters" {
		t.Fatalf("err = %v", err)
	}
}

func TestBlikRecurringRegistrationBody(t *testing.T) {
	value := PLN(1000)
	registration := BlikRecurringRegistration{
		Label:          "Subskrypcja",
		Model:          BlikRecurringModelAutomatic,
		Frequency:      "1M",
		Value:          &value,
		LimitAmt:       Int(5000),
		TotLimitAmt:    Int(60000),
		LimitAmtFixed:  Bool(true),
		ExpirationDate: String("2027-01-01"),
		InitDate:       String("2026-08-01"),
	}
	if err := registration.Validate(); err != nil {
		t.Fatal(err)
	}
	want := `{"label":"Subskrypcja","type":"PAYID","model":"A","frequency":"1M","value":"10.00",` +
		`"limit_amt":5000,"tot_limit_amt":60000,"is_limit_amt_fixed":true,` +
		`"expiration_date":"2027-01-01","init_date":"2026-08-01"}`
	if got := bodyJSON(t, registration.toBody()); got != want {
		t.Fatalf("toBody() = %s\nwant       = %s", got, want)
	}
}

func TestBlikRecurringRegistrationValidate(t *testing.T) {
	base := BlikRecurringRegistration{Label: "L", Model: BlikRecurringModelAutomatic, Frequency: "1M"}
	if err := base.Validate(); err != nil {
		t.Fatal(err)
	}
	bad := base
	bad.Model = "X"
	if err := bad.Validate(); err == nil || err.Error() != `dpay: Invalid recurring model "X"` {
		t.Fatalf("err = %v", err)
	}
	for _, frequency := range []string{"0D", "1X", "1000D", "M", "1MM"} {
		bad = base
		bad.Frequency = frequency
		if err := bad.Validate(); err == nil {
			t.Errorf("frequency %q must fail", frequency)
		}
	}
	for _, frequency := range []string{"1D", "2W", "12M", "999Q", "1Y"} {
		bad = base
		bad.Frequency = frequency
		if err := bad.Validate(); err != nil {
			t.Errorf("frequency %q must pass: %v", frequency, err)
		}
	}
	bad = base
	bad.InitDate = String("2026/08/01")
	if err := bad.Validate(); err == nil || err.Error() != `dpay: Date "2026/08/01" must be in YYYY-MM-DD format` {
		t.Fatalf("err = %v", err)
	}
}

func TestCardRecurringRegistrationBody(t *testing.T) {
	limit := PLN(50000)
	total := PLN(600000)
	registration := CardRecurringRegistration{
		Label:          "Mandat",
		Frequency:      CardRecurringFrequencyMonthly,
		LimitAmt:       &limit,
		TotLimitAmt:    &total,
		LimitAmtFixed:  Bool(false),
		ExpirationDate: String("2027-01-01"),
		InitDate:       String("2026-08-01"),
	}
	if err := registration.Validate(); err != nil {
		t.Fatal(err)
	}
	want := `{"label":"Mandat","frequency":"MONTHLY","limit_amt":50000,"tot_limit_amt":600000,` +
		`"is_limit_amt_fixed":false,"expiration_date":"2027-01-01","init_date":"2026-08-01"}`
	if got := bodyJSON(t, registration.toBody()); got != want {
		t.Fatalf("toBody() = %s\nwant       = %s", got, want)
	}
	minimal := CardRecurringRegistration{Label: "M"}
	if got := bodyJSON(t, minimal.toBody()); got != `{"label":"M"}` {
		t.Fatalf("minimal = %s", got)
	}
	bad := CardRecurringRegistration{Label: ""}
	if err := bad.Validate(); err == nil || err.Error() != "dpay: Mandate label must be 1-50 characters" {
		t.Fatalf("err = %v", err)
	}
}

func TestEnumWireValues(t *testing.T) {
	cases := map[string]string{
		string(TransactionTypeTransfers):         "transfers",
		string(TransactionTypeDCBGateway):        "dcb_gateway",
		string(TransactionTypeCardAuth):          "card_auth",
		string(TransactionTypeMBWayDirect):       "mb_way_direct",
		string(TransactionTypeBizumDirect):       "bizum_direct",
		string(TransactionTypeBlikRecurring):     "blik_recurring",
		string(TransactionTypeCardRecurring):     "card_recurring",
		string(TransactionStatusPaid):            "paid",
		string(TransactionStatusCaptured):        "captured",
		string(TransactionStatusCreated):         "created",
		string(TransactionStatusProcessing):      "processing",
		string(TransactionStatusExpired):         "expired",
		string(BlikAliasTypeUID):                 "UID",
		string(BlikAliasTypePayID):               "PAYID",
		string(RedirectTypeSuccess):              "SUCCESS",
		string(RedirectTypeForm):                 "FORM",
		string(RedirectTypeURL):                  "URL",
		string(RedirectTypeDCCOffer):             "DCC_OFFER",
		string(DCCDecisionAccept):                "accept",
		string(DCCDecisionReject):                "reject",
		string(CardRecurringFrequencyDaily):      "DAILY",
		string(CardRecurringFrequencyBiweekly):   "BIWEEKLY",
		string(CardRecurringFrequencyAnnual):     "ANNUAL",
		string(CardRecurringOperationAddCard):    "add_card",
		string(CardRecurringOperationCOFInitial): "cof_initial",
		string(CardRecurringOperationCharge):     "charge",
		string(PayoutFeeModeNet):                 "net",
		string(PayoutFeeModeGross):               "gross",
		string(BlikRecurringModelAutomatic):      "A",
		string(BlikRecurringModelManual):         "M",
		string(BlikRecurringModelOnDemand):       "O",
		string(IPNTypeTransfer):                  "transfer",
		string(IPNTypeCapture):                   "capture",
		string(IPNTypeDCB):                       "dcb",
	}
	for got, want := range cases {
		if got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	}
}

func TestEnumValidRejectsUnknown(t *testing.T) {
	if TransactionType("x").Valid() || TransactionStatus("x").Valid() || BlikAliasType("x").Valid() ||
		RedirectType("x").Valid() || DCCDecision("x").Valid() || CardRecurringFrequency("x").Valid() ||
		CardRecurringOperation("x").Valid() || PayoutFeeMode("x").Valid() ||
		BlikRecurringModel("x").Valid() || IPNType("x").Valid() {
		t.Fatal("unknown values must be invalid")
	}
	if !TransactionTypeTransfers.Valid() || !TransactionStatusPaid.Valid() || !IPNTypeDCB.Valid() ||
		!PayoutFeeModeGross.Valid() || !RedirectTypeForm.Valid() || !DCCDecisionReject.Valid() ||
		!CardRecurringFrequencyMonthly.Valid() || !CardRecurringOperationCharge.Valid() ||
		!BlikAliasTypePayID.Valid() || !BlikRecurringModelManual.Valid() {
		t.Fatal("known values must be valid")
	}
}
