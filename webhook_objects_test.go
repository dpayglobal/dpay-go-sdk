package dpay

import (
	"encoding/json"
	"reflect"
	"testing"
)

func webhookEventWith(t *testing.T, eventType string, object string) *WebhookEvent {
	t.Helper()
	raw := `{"id":"evt_01k6a8q2m4pz7h8c3v5n9t2x6y","type":"` + eventType + `","api_version":"2026-10-01",` +
		`"created":"2026-09-27T10:05:00Z","livemode":true,"service":"abc123","data":{"object":` + object + `}}`
	var decoded any
	if err := json.Unmarshal([]byte(raw), &decoded); err != nil {
		t.Fatal(err)
	}
	event, ok := webhookEventFromJSON(decoded)
	if !ok {
		t.Fatal("not an event")
	}
	return event
}

func TestWebhookPaymentSucceeded(t *testing.T) {
	event := webhookEventWith(t, "payment.succeeded", `{"object":"payment","id":"8F2A11C4-5B6D-4E7F-8A9B-0C1D2E3F4A5B",
		"status":"succeeded","amount":14900,"currency":"PLN","amount_refunded":0,"amount_captured":0,
		"description":"Order 2026/09/123","created":"2026-09-27T10:03:12Z","paid_at":"2026-09-27T10:05:00Z",
		"payment_method":{"type":"blik","blik":{"flow":"code"}},"failure":null,"recurring":null,
		"references":{"merchant":"2026/09/123","provider":"1234567890"},"custom":"order-789"}`)
	payment := event.Payment()
	if payment == nil || event.Refund() != nil || event.RecurringPayment() != nil || event.Payout() != nil {
		t.Fatal("only the payment accessor applies")
	}
	if payment.ID() != "8F2A11C4-5B6D-4E7F-8A9B-0C1D2E3F4A5B" || payment.Status() != PaymentObjectStatusSucceeded {
		t.Fatalf("payment = %+v", payment.Raw())
	}
	if !payment.Amount().Equal(PLN(14900)) || !payment.AmountRefunded().Equal(PLN(0)) || payment.AmountCaptured().Minor() != 0 {
		t.Fatal("amounts broken")
	}
	if payment.Description() != "Order 2026/09/123" || payment.Created() != "2026-09-27T10:03:12Z" || payment.PaidAt() != "2026-09-27T10:05:00Z" {
		t.Fatal("strings broken")
	}
	if payment.PaymentMethodType() != "blik" ||
		!reflect.DeepEqual(payment.PaymentMethod(), map[string]any{"type": "blik", "blik": map[string]any{"flow": "code"}}) {
		t.Fatalf("payment method = %v", payment.PaymentMethod())
	}
	if payment.Failure() != nil || payment.IsRecurring() || payment.RecurringAlias() != "" {
		t.Fatal("a plain payment has no failure and no recurring")
	}
	if payment.MerchantReference() != "2026/09/123" || payment.ProviderReference() != "1234567890" || payment.Custom() != "order-789" {
		t.Fatal("references broken")
	}
}

func TestWebhookPaymentFailedRecurringChargeInEuro(t *testing.T) {
	payment := webhookEventWith(t, "payment.failed", `{"object":"payment","id":"P-2","status":"failed","amount":4999,
		"currency":"EUR","amount_refunded":0,"amount_captured":0,"description":null,"created":"2026-10-01T08:00:00Z",
		"paid_at":null,"payment_method":null,
		"failure":{"code":"insufficient_funds","message":"Insufficient funds","category":"customer","retryable":true,"provider_code":"INSUFFICIENT_FUNDS"},
		"recurring":{"alias":"SUB-0001","registration":"P-1","sequence":"subsequent"},
		"references":{"merchant":null,"provider":null},"custom":null}`).Payment()
	eur, _ := NewMoney(4999, CurrencyEUR)
	if payment.Status() != PaymentObjectStatusFailed || !payment.Amount().Equal(eur) || payment.PaidAt() != "" ||
		payment.PaymentMethodType() != "" || payment.PaymentMethod() != nil || payment.MerchantReference() != "" {
		t.Fatalf("payment = %+v", payment.Raw())
	}
	if !payment.IsRecurring() || payment.RecurringAlias() != "SUB-0001" || payment.RecurringRegistration() != "P-1" ||
		payment.RecurringSequence() != RecurringSequenceSubsequent {
		t.Fatal("recurring broken")
	}
	failure := payment.Failure()
	if failure == nil || failure.Code() != DeclineCodeInsufficientFunds || failure.Message() != "Insufficient funds" ||
		failure.Category() != FailureCategoryCustomer || !failure.IsRetryable() || failure.ProviderCode() != "INSUFFICIENT_FUNDS" {
		t.Fatalf("failure = %+v", failure)
	}
}

func TestWebhookRefundFailed(t *testing.T) {
	refund := webhookEventWith(t, "refund.failed", `{"object":"refund","id":null,"payment":"8F2A11C4-5B6D-4E7F-8A9B-0C1D2E3F4A5B",
		"status":"failed","amount":2000,"currency":"PLN","created":"2026-10-01T08:00:00Z","succeeded_at":null,
		"failure":{"code":"refund_window_expired","message":"The refund period has expired","category":"merchant","retryable":false,"provider_code":null}}`).Refund()
	if refund.ID() != "" || refund.Payment() != "8F2A11C4-5B6D-4E7F-8A9B-0C1D2E3F4A5B" || refund.Status() != RefundObjectStatusFailed ||
		refund.Amount().Minor() != 2000 || refund.Currency() != CurrencyPLN || refund.SucceededAt() != "" || refund.Created() != "2026-10-01T08:00:00Z" {
		t.Fatalf("refund = %+v", refund.Raw())
	}
	failure := refund.Failure()
	if failure.Code() != DeclineCodeRefundWindowExpired || failure.Category() != FailureCategoryMerchant || failure.IsRetryable() || failure.ProviderCode() != "" {
		t.Fatalf("failure = %+v", failure.Raw())
	}
}

func TestWebhookRecurringPaymentCanceled(t *testing.T) {
	recurring := webhookEventWith(t, "recurring_payment.canceled", `{"object":"recurring_payment","id":"P-1","alias":"SUB-0001",
		"status":"canceled","canceled_by":"customer","payment_method":{"type":"blik","blik":{"flow":"recurring"}},
		"terms":{"model":"A","frequency":"1M","limit_amount":5999,"total_limit_amount":71988,"starts_on":"2026-11-01","expires_on":"2027-09-30"},
		"created":"2026-09-26T10:00:00Z"}`).RecurringPayment()
	if recurring.ID() != "P-1" || recurring.Alias() != "SUB-0001" || recurring.Status() != RecurringPaymentObjectStatusCanceled ||
		recurring.CanceledBy() != RecurringCanceledByCustomer || recurring.PaymentMethodType() != "blik" || recurring.PaymentMethod() == nil {
		t.Fatalf("recurring = %+v", recurring.Raw())
	}
	if recurring.Model() != "A" || recurring.Frequency() != "1M" || *recurring.LimitAmount() != 5999 || *recurring.TotalLimitAmount() != 71988 ||
		recurring.StartsOn() != "2026-11-01" || recurring.ExpiresOn() != "2027-09-30" || recurring.Created() != "2026-09-26T10:00:00Z" {
		t.Fatal("terms broken")
	}
}

func TestWebhookRecurringPaymentModelOHasNoLimits(t *testing.T) {
	recurring := webhookEventWith(t, "recurring_payment.activated", `{"object":"recurring_payment","id":"P-1","alias":"SUB-0002",
		"status":"active","canceled_by":null,"payment_method":null,
		"terms":{"model":"O","frequency":null,"limit_amount":null,"total_limit_amount":"500","starts_on":null,"expires_on":null},
		"created":"2026-09-26T10:00:00Z"}`).RecurringPayment()
	if recurring.Status() != RecurringPaymentObjectStatusActive || recurring.CanceledBy() != "" || recurring.LimitAmount() != nil ||
		recurring.TotalLimitAmount() != nil || recurring.Frequency() != "" {
		t.Fatalf("recurring = %+v", recurring.Raw())
	}
}

func TestWebhookPayoutFailed(t *testing.T) {
	payout := webhookEventWith(t, "payout.failed", `{"object":"payout","id":"12345","status":"failed","amount":2715,"fee":306,
		"currency":"PLN","bank_account":{"last4":"2874"},"created":"2026-08-01T08:00:00Z","paid_at":null,
		"failure":{"code":"payout_failed","message":"The payout could not be completed","category":"merchant","retryable":false,"provider_code":"RETURNED"}}`).Payout()
	if payout.ID() != "12345" || payout.Status() != PayoutObjectStatusFailed || payout.Amount().Minor() != 2715 || payout.Fee().Minor() != 306 ||
		payout.BankAccountLast4() != "2874" || payout.PaidAt() != "" || payout.Created() != "2026-08-01T08:00:00Z" {
		t.Fatalf("payout = %+v", payout.Raw())
	}
	if failure := payout.Failure(); failure.Code() != DeclineCodePayoutFailed || failure.ProviderCode() != "RETURNED" {
		t.Fatalf("failure = %+v", failure.Raw())
	}
}

func TestWebhookTestEventHasNoTypedObject(t *testing.T) {
	event := webhookEventWith(t, "webhook.test", `{"object":"webhook_endpoint","id":"we_1"}`)
	if event.Payment() != nil || event.Refund() != nil || event.RecurringPayment() != nil || event.Payout() != nil ||
		event.ObjectType() != "webhook_endpoint" {
		t.Fatal("a test event has no typed object")
	}
}

func TestWebhookObjectMissingFieldsFallBack(t *testing.T) {
	payment := &PaymentObject{raw: map[string]any{"object": "payment", "currency": "xx", "failure": map[string]any{"code": ""}, "id": float64(17), "amount": 12.5}}
	if payment.ID() != "17" || payment.Currency() != CurrencyPLN || payment.Amount().Minor() != 0 || payment.Amount().Currency() != CurrencyPLN {
		t.Fatalf("payment = %+v", payment.Raw())
	}
	failure := payment.Failure()
	if failure == nil || failure.Code() != DeclineCodeGenericDecline || failure.IsRetryable() || failure.Category() != "" {
		t.Fatalf("failure = %+v", failure)
	}
	retryable := &Failure{raw: map[string]any{"retryable": "true", "provider_code": float64(5)}}
	if retryable.IsRetryable() || retryable.ProviderCode() != "5" || retryable.Code() != DeclineCodeGenericDecline {
		t.Fatal("only JSON true is retryable; an integer provider code is a string")
	}
}

func TestDeclineCatalog(t *testing.T) {
	codes := DeclineCodes()
	if len(codes) != 22 || codes[0] != DeclineCodeInsufficientFunds || codes[21] != DeclineCodeProcessingError {
		t.Fatalf("codes = %v", codes)
	}
	seen := map[DeclineCode]bool{}
	for _, code := range codes {
		if seen[code] || !code.Valid() {
			t.Fatalf("%q duplicated or invalid", code)
		}
		seen[code] = true
	}
	if DeclineCode("nope").Valid() {
		t.Fatal("unknown code accepted")
	}
	if !reflect.DeepEqual(FailureCategories(), []FailureCategory{"customer", "merchant", "risk", "system"}) {
		t.Fatalf("categories = %v", FailureCategories())
	}
	if refusals := RetryRefusals(); len(refusals) != 8 || refusals[7] != RetryRefusalServiceUnavailable || RetryRefusalServiceUnavailable != "Service unavailable" {
		t.Fatalf("refusals = %v", refusals)
	}
}
