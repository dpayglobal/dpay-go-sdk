package dpay

// Typed views of data.object of a webhook event (WebhookEvent.Payment,
// Refund, RecurringPayment and Payout). Common parsing rules: a string field is
// taken when it is a JSON string or integer, else empty; an amount is a JSON
// integer in minor units (else 0) in the currency of the object (PLN when it is
// missing or invalid); a nested object is a map or nil.

// Payment returns data.object as a payment, nil unless the object is a payment
// (payment.succeeded, payment.failed, payment.captured).
func (e *WebhookEvent) Payment() *PaymentObject {
	if e.ObjectType() != "payment" {
		return nil
	}
	return &PaymentObject{raw: e.Object()}
}

// Refund returns data.object as a refund, nil unless the object is a refund
// (refund.succeeded, refund.failed).
func (e *WebhookEvent) Refund() *RefundObject {
	if e.ObjectType() != "refund" {
		return nil
	}
	return &RefundObject{raw: e.Object()}
}

// RecurringPayment returns data.object as a recurring payment, nil unless the
// object is a recurring_payment (recurring_payment.*).
func (e *WebhookEvent) RecurringPayment() *RecurringPaymentObject {
	if e.ObjectType() != "recurring_payment" {
		return nil
	}
	return &RecurringPaymentObject{raw: e.Object()}
}

// Payout returns data.object as a payout, nil unless the object is a payout
// (payout.paid, payout.failed).
func (e *WebhookEvent) Payout() *PayoutObject {
	if e.ObjectType() != "payout" {
		return nil
	}
	return &PayoutObject{raw: e.Object()}
}

func objectCurrency(data map[string]any) Currency {
	return currencyCode(data, "currency", CurrencyPLN)
}

func objectMoney(data map[string]any, key string, currency Currency) Money {
	minor, _ := jsonInt(data[key])
	return Money{minor: minor, currency: currency}
}

func objectFailure(data map[string]any) *Failure {
	if failure := objectOrNil(data, "failure"); failure != nil {
		return &Failure{raw: failure}
	}
	return nil
}

// PaymentObject is the payment object of a webhook event.
type PaymentObject struct {
	raw map[string]any
}

// ID returns the payment id (the transactionId of the registration), empty when missing.
func (p *PaymentObject) ID() string { return stringOrInt(p.raw, "id") }

// Status returns the payment status, preserved verbatim even when unknown.
func (p *PaymentObject) Status() PaymentObjectStatus {
	return PaymentObjectStatus(stringOrInt(p.raw, "status"))
}

// Currency returns the currency of the amounts.
func (p *PaymentObject) Currency() Currency { return objectCurrency(p.raw) }

// Amount returns the payment amount.
func (p *PaymentObject) Amount() Money { return objectMoney(p.raw, "amount", p.Currency()) }

// AmountRefunded returns the amount refunded so far.
func (p *PaymentObject) AmountRefunded() Money {
	return objectMoney(p.raw, "amount_refunded", p.Currency())
}

// AmountCaptured returns the amount captured so far (card pre-authorizations).
func (p *PaymentObject) AmountCaptured() Money {
	return objectMoney(p.raw, "amount_captured", p.Currency())
}

// Description returns the payment description.
func (p *PaymentObject) Description() string { return stringOrInt(p.raw, "description") }

// Created returns the creation time in UTC.
func (p *PaymentObject) Created() string { return stringOrInt(p.raw, "created") }

// PaidAt returns the payment time in UTC, empty when not paid.
func (p *PaymentObject) PaidAt() string { return stringOrInt(p.raw, "paid_at") }

// PaymentMethod returns the payment_method object, e.g. {type: blik, blik: {flow: code}}, nil when absent.
func (p *PaymentObject) PaymentMethod() map[string]any { return objectOrNil(p.raw, "payment_method") }

// PaymentMethodType returns payment_method.type, e.g. blik, card or pbl.
func (p *PaymentObject) PaymentMethodType() string {
	return stringOrInt(phpObject(p.raw["payment_method"]), "type")
}

// Failure returns why the payment failed, nil when it did not.
func (p *PaymentObject) Failure() *Failure { return objectFailure(p.raw) }

// IsRecurring reports whether the payment belongs to a recurring payment.
func (p *PaymentObject) IsRecurring() bool { return objectOrNil(p.raw, "recurring") != nil }

// RecurringAlias returns recurring.alias, the alias of the recurring payment.
func (p *PaymentObject) RecurringAlias() string { return stringOrInt(p.recurring(), "alias") }

// RecurringRegistration returns recurring.registration, the payment that registered the recurring payment.
func (p *PaymentObject) RecurringRegistration() string {
	return stringOrInt(p.recurring(), "registration")
}

// RecurringSequence returns recurring.sequence: first (the registering payment) or subsequent (a charge).
func (p *PaymentObject) RecurringSequence() RecurringSequence {
	return RecurringSequence(stringOrInt(p.recurring(), "sequence"))
}

// MerchantReference returns references.merchant (RegisterPaymentRequest.Reference).
func (p *PaymentObject) MerchantReference() string {
	return stringOrInt(phpObject(p.raw["references"]), "merchant")
}

// ProviderReference returns references.provider, the identifier at the payment provider.
func (p *PaymentObject) ProviderReference() string {
	return stringOrInt(phpObject(p.raw["references"]), "provider")
}

// Custom returns the custom value of the registration.
func (p *PaymentObject) Custom() string { return stringOrInt(p.raw, "custom") }

// Raw returns the object exactly as the API sent it.
func (p *PaymentObject) Raw() map[string]any { return p.raw }

func (p *PaymentObject) recurring() map[string]any { return phpObject(p.raw["recurring"]) }

// RefundObject is the refund object of a webhook event.
type RefundObject struct {
	raw map[string]any
}

// ID returns the refund id, empty for a refund rejected before it was created.
func (r *RefundObject) ID() string { return stringOrInt(r.raw, "id") }

// Payment returns the id of the refunded payment.
func (r *RefundObject) Payment() string { return stringOrInt(r.raw, "payment") }

// Status returns the refund status, preserved verbatim even when unknown.
func (r *RefundObject) Status() RefundObjectStatus {
	return RefundObjectStatus(stringOrInt(r.raw, "status"))
}

// Currency returns the currency of the amount.
func (r *RefundObject) Currency() Currency { return objectCurrency(r.raw) }

// Amount returns the refunded amount (positive minor units).
func (r *RefundObject) Amount() Money { return objectMoney(r.raw, "amount", r.Currency()) }

// Created returns the creation time in UTC.
func (r *RefundObject) Created() string { return stringOrInt(r.raw, "created") }

// SucceededAt returns when the refund succeeded, empty otherwise.
func (r *RefundObject) SucceededAt() string { return stringOrInt(r.raw, "succeeded_at") }

// Failure returns why the refund failed, nil when it did not.
func (r *RefundObject) Failure() *Failure { return objectFailure(r.raw) }

// Raw returns the object exactly as the API sent it.
func (r *RefundObject) Raw() map[string]any { return r.raw }

// RecurringPaymentObject is the recurring_payment object of a webhook event.
// It carries no currency, so its limits are plain minor units.
type RecurringPaymentObject struct {
	raw map[string]any
}

// ID returns the recurring payment id, empty when missing.
func (r *RecurringPaymentObject) ID() string { return stringOrInt(r.raw, "id") }

// Alias returns the alias of the recurring payment (RegisterPaymentRequest.RecurringAlias).
func (r *RecurringPaymentObject) Alias() string { return stringOrInt(r.raw, "alias") }

// Status returns the recurring payment status, preserved verbatim even when unknown.
func (r *RecurringPaymentObject) Status() RecurringPaymentObjectStatus {
	return RecurringPaymentObjectStatus(stringOrInt(r.raw, "status"))
}

// CanceledBy returns who canceled the recurring payment, empty unless canceled.
func (r *RecurringPaymentObject) CanceledBy() RecurringCanceledBy {
	return RecurringCanceledBy(stringOrInt(r.raw, "canceled_by"))
}

// PaymentMethod returns the payment_method object, nil when absent.
func (r *RecurringPaymentObject) PaymentMethod() map[string]any {
	return objectOrNil(r.raw, "payment_method")
}

// PaymentMethodType returns payment_method.type, e.g. blik.
func (r *RecurringPaymentObject) PaymentMethodType() string {
	return stringOrInt(phpObject(r.raw["payment_method"]), "type")
}

// Model returns terms.model, the recurring model.
func (r *RecurringPaymentObject) Model() string { return stringOrInt(r.terms(), "model") }

// Frequency returns terms.frequency, e.g. 1M.
func (r *RecurringPaymentObject) Frequency() string { return stringOrInt(r.terms(), "frequency") }

// LimitAmount returns terms.limit_amount in minor units, nil when it is not a JSON integer.
func (r *RecurringPaymentObject) LimitAmount() *int64 {
	return optionalJSONInt(r.terms(), "limit_amount")
}

// TotalLimitAmount returns terms.total_limit_amount in minor units, nil when it is not a JSON integer.
func (r *RecurringPaymentObject) TotalLimitAmount() *int64 {
	return optionalJSONInt(r.terms(), "total_limit_amount")
}

// StartsOn returns terms.starts_on.
func (r *RecurringPaymentObject) StartsOn() string { return stringOrInt(r.terms(), "starts_on") }

// ExpiresOn returns terms.expires_on.
func (r *RecurringPaymentObject) ExpiresOn() string { return stringOrInt(r.terms(), "expires_on") }

// Created returns the creation time in UTC.
func (r *RecurringPaymentObject) Created() string { return stringOrInt(r.raw, "created") }

// Raw returns the object exactly as the API sent it.
func (r *RecurringPaymentObject) Raw() map[string]any { return r.raw }

func (r *RecurringPaymentObject) terms() map[string]any { return phpObject(r.raw["terms"]) }

// PayoutObject is the payout object of a webhook event.
type PayoutObject struct {
	raw map[string]any
}

// ID returns the payout id, empty when missing.
func (p *PayoutObject) ID() string { return stringOrInt(p.raw, "id") }

// Status returns the payout status, preserved verbatim even when unknown.
func (p *PayoutObject) Status() PayoutObjectStatus {
	return PayoutObjectStatus(stringOrInt(p.raw, "status"))
}

// Currency returns the currency of the amounts.
func (p *PayoutObject) Currency() Currency { return objectCurrency(p.raw) }

// Amount returns the net payout amount.
func (p *PayoutObject) Amount() Money { return objectMoney(p.raw, "amount", p.Currency()) }

// Fee returns the payout fee.
func (p *PayoutObject) Fee() Money { return objectMoney(p.raw, "fee", p.Currency()) }

// BankAccountLast4 returns bank_account.last4, the last digits of the receiving account.
func (p *PayoutObject) BankAccountLast4() string {
	return stringOrInt(phpObject(p.raw["bank_account"]), "last4")
}

// Created returns the creation time in UTC.
func (p *PayoutObject) Created() string { return stringOrInt(p.raw, "created") }

// PaidAt returns when the payout was paid, empty otherwise.
func (p *PayoutObject) PaidAt() string { return stringOrInt(p.raw, "paid_at") }

// Failure returns why the payout failed, nil when it did not.
func (p *PayoutObject) Failure() *Failure { return objectFailure(p.raw) }

// Raw returns the object exactly as the API sent it.
func (p *PayoutObject) Raw() map[string]any { return p.raw }

// Failure is the failure object of a payment, refund or payout in a webhook event.
type Failure struct {
	raw map[string]any
}

// Code returns the decline code, DeclineCodeGenericDecline when missing or empty.
func (f *Failure) Code() DeclineCode {
	if code, ok := f.raw["code"].(string); ok && code != "" {
		return DeclineCode(code)
	}
	return DeclineCodeGenericDecline
}

// Message returns the description of the failure.
func (f *Failure) Message() string { return strictString(f.raw, "message") }

// Category returns customer, merchant, risk or system, empty when missing.
func (f *Failure) Category() FailureCategory { return FailureCategory(strictString(f.raw, "category")) }

// IsRetryable reports whether retrying may succeed; only an explicit true counts.
func (f *Failure) IsRetryable() bool {
	retryable, ok := f.raw["retryable"].(bool)
	return ok && retryable
}

// ProviderCode returns the raw code of the payment provider.
func (f *Failure) ProviderCode() string { return stringOrInt(f.raw, "provider_code") }

// Raw returns the failure object exactly as the API sent it.
func (f *Failure) Raw() map[string]any { return f.raw }
