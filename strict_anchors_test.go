package dpay

import (
	"context"
	"errors"
	"regexp"
	"testing"
)

// Every validation pattern matches the whole string: Go's $ without (?m) is
// the absolute end and \d is ASCII, so a trailing "\n" never passes (PHP's $
// without D matched before it; fixed there with \z).
func TestValidationPatternsMatchTheWholeString(t *testing.T) {
	patterns := map[string]struct {
		pattern *regexp.Regexp
		valid   string
	}{
		"date":        {datePattern, "2026-12-01"},
		"frequency":   {recurringFrequencyPattern, "1M"},
		"partner":     {partnerPattern, "SHOPER"},
		"blik code":   {blikCodePattern, "123456"},
		"pan":         {panPattern, "4111111111111111"},
		"cvv":         {cvvPattern, "123"},
		"expiry":      {expiryPattern, "12/30"},
		"event id":    {eventIDPattern, "evt_01k6a8q2m4pz7h8c3v5n9t2x6y"},
		"iban":        {polishIBANPattern, "PL61109010140000071219812874"},
		"currency":    {currencyPattern, "PLN"},
		"decimal":     {decimalPattern, "10.00"},
		"integer":     {integerPattern, "-30"},
		"header name": {headerNamePattern, "X-Id"},
		"message id":  {refundMessageIDPattern, "RET"},
		"email":       {emailPattern, "jan@example.com"},
	}
	for name, testCase := range patterns {
		if !testCase.pattern.MatchString(testCase.valid) {
			t.Errorf("%s: %q must match", name, testCase.valid)
		}
		for _, invalid := range []string{testCase.valid + "\n", "\n" + testCase.valid, "x\n" + testCase.valid, testCase.valid + "\nx"} {
			if testCase.pattern.MatchString(invalid) {
				t.Errorf("%s: %q must not match", name, invalid)
			}
		}
	}
	// \d is ASCII: other Unicode digits are not digits
	if blikCodePattern.MatchString("\u0661\u0662\u0663\u0664\u0665\u0666") || datePattern.MatchString("\uff12026-12-01") {
		t.Fatal("only ASCII digits are digits")
	}
}

func TestTrailingNewlineIsRejected(t *testing.T) {
	for _, card := range []CardData{
		{PAN: "4111111111111111\n", CVV: "123", Expiry: "12/30"},
		{PAN: "4111111111111111", CVV: "123\n", Expiry: "12/30"},
		{PAN: "4111111111111111", CVV: "123", Expiry: "12/30\n"},
	} {
		if err := card.Validate(); !errors.Is(err, ErrInvalidArgument) {
			t.Errorf("%+v: err = %v", card, err)
		}
	}
	if err := validateDate("2026-12-01\n"); !errors.Is(err, ErrInvalidArgument) {
		t.Errorf("date: err = %v", err)
	}
	if err := (InvoiceDetails{PaymentDueDate: String("2026-12-01\n")}).Validate(); err == nil ||
		err.Error() != "dpay: Payment due date must be in YYYY-MM-DD format" {
		t.Errorf("due date: err = %v", err)
	}
	if _, err := ParseMoney("10.00\n", CurrencyPLN); !errors.Is(err, ErrInvalidArgument) {
		t.Errorf("money: err = %v", err)
	}
	if Currency("PLN\n").Valid() {
		t.Error(`Currency("PLN\n") must be invalid`)
	}
	if _, ok := moneyFromAPI("10.00\n", CurrencyPLN); ok {
		t.Error(`moneyFromAPI("10.00\n") must fail`)
	}
	transaction := transactionFromAPI(map[string]any{"transaction": map[string]any{"minimal_fee": "30\n", "permanent_fee": "30"}})
	if transaction.MinimalFee() != nil || transaction.PermanentFee() == nil || *transaction.PermanentFee() != 30 {
		t.Errorf("fees = %v %v", transaction.MinimalFee(), transaction.PermanentFee())
	}

	client, doer := scriptedClient(t)
	_, err := client.send(context.Background(), "POST", hostPanel, "/x", []byte(`{}`), requestHeader{name: "X-Id\n", value: "x"})
	assertInvalidArgument(t, err, "Invalid request header \"X-Id\n\"", doer)
	_, err = client.Events.List(context.Background(), &EventListParams{StartingAfter: String("evt_01k6a8q2m4pz7h8c3v5n9t2x6y\n")})
	assertInvalidArgument(t, err, "starting_after must be an event id (evt_...)", doer)

	vector := loadAPIVectors(t).Webhook
	headers := webhookHeaders(vector, "")
	headers["WEBHOOK-TIMESTAMP"] = []string{"1790503500\n"}
	if err := verifierAt(vector, 0, vector.Secret).Verify([]byte(vector.Body), headers); !errors.Is(err, ErrSignature) ||
		err.Error() != "dpay: Invalid webhook-timestamp header" {
		t.Errorf("webhook-timestamp: err = %v", err)
	}
}
