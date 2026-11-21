package dpay

import (
	"context"
	"math"
	"regexp"
	"strconv"
	"strings"

	"github.com/dpayglobal/dpay-go-sdk/internal/php"
	"github.com/dpayglobal/dpay-go-sdk/internal/wire"
)

// PaymentService registers payments and reads transaction details.
type PaymentService struct {
	client *Client
}

// Register creates a payment and returns the redirect target or the inline
// result. The checksum is sha256(service|hash|value|url_success|url_fail|url_ipn),
// with an empty url_ipn segment without an IPN URL; a recurring charge appends
// its alias, which binds the charge to that customer.
func (s *PaymentService) Register(ctx context.Context, request *RegisterPaymentRequest) (*RegisteredPayment, error) {
	body, err := request.toBody(s.client.service)
	if err != nil {
		return nil, err
	}
	fields := []any{
		bodyString(body, "value"),
		bodyString(body, "url_success"),
		bodyString(body, "url_fail"),
		bodyString(body, "url_ipn"),
	}
	if _, charge := body.Get("recurring_alias"); charge {
		fields = append(fields, bodyString(body, "recurring_alias"))
	}
	body.Set("checksum", s.client.checksum.SecretSecond(s.client.service, fields))

	data, err := s.client.postJSONObject(ctx, hostAPIPayments, "/api/v1_0/payments/register", body)
	if err != nil {
		return nil, err
	}
	if isRejection(data) {
		return nil, paymentRejectedFromAPI(data)
	}
	return registeredPaymentFromAPI(data, request.Amount.Currency()), nil
}

// Details reads the current state of a transaction.
func (s *PaymentService) Details(ctx context.Context, transactionID string) (*Transaction, error) {
	service, err := s.client.panelService()
	if err != nil {
		return nil, err
	}
	transactionID, err = requiredPanelString(transactionID, "transaction_id")
	if err != nil {
		return nil, err
	}
	body := wire.NewBody()
	body.Set("service", service)
	body.Set("transaction_id", transactionID)

	data, err := s.client.postJSONObject(ctx, hostPanel, "/api/v1/pbl/details", s.client.signPanelBody(body))
	if err != nil {
		return nil, err
	}
	return transactionFromAPI(data), nil
}

func bodyString(body *wire.Body, key string) string {
	value, exists := body.Get(key)
	if !exists {
		return ""
	}
	if text, ok := value.(string); ok {
		return text
	}
	return ""
}

func isRejection(data map[string]any) bool {
	if flag, ok := data["error"].(bool); ok && flag {
		return true
	}
	if flag, ok := data["status"].(bool); ok && !flag {
		return true
	}
	return false
}

func paymentRejectedFromAPI(data map[string]any) *PaymentRejectedError {
	message := "Payment rejected"
	if value, ok := data["msg"].(string); ok {
		message = value
	}
	additionalInfo := objectField(data, "additionalInfo")
	return &PaymentRejectedError{
		APIError: &APIError{
			kind:        ErrPaymentRejected,
			message:     message,
			HTTPStatus:  200,
			ErrorCode:   stringField(additionalInfo, "error"),
			FieldErrors: map[string][]string{},
			RawBody:     marshalRaw(data),
			// any scalar id, like PaymentRejectedError.TransactionID
			TransactionID: stringField(data, "transactionId"),
		},
		TransactionID:    stringField(data, "transactionId"),
		ErrorDescription: strictString(additionalInfo, "error_description"),
	}
}

// RegisteredPayment is the result of registering a payment.
type RegisteredPayment struct {
	raw           map[string]any
	currency      Currency
	transactionID string
	message       string
	ipksef        string
}

// registeredPaymentFromAPI maps a registration response; currency is the
// currency of the registered amount (for PaidTransaction).
func registeredPaymentFromAPI(data map[string]any, currency Currency) *RegisteredPayment {
	if !currency.Valid() {
		currency = CurrencyPLN
	}
	return &RegisteredPayment{
		raw:           data,
		currency:      currency,
		transactionID: stringField(data, "transactionId"),
		message:       stringField(data, "msg"),
		ipksef:        stringField(data, "ipksef"),
	}
}

// TransactionID returns the identifier assigned to the payment.
func (p *RegisteredPayment) TransactionID() string { return p.transactionID }

// Message returns the raw msg field, which carries either a redirect URL or a status phrase.
func (p *RegisteredPayment) Message() string { return p.message }

// RedirectURL returns the URL to send the payer to, or an empty string when the
// message is not a URL.
func (p *RegisteredPayment) RedirectURL() string {
	if strings.HasPrefix(p.message, "http://") || strings.HasPrefix(p.message, "https://") {
		return p.message
	}
	return ""
}

// IsPaid reports whether the payment completed during registration.
func (p *RegisteredPayment) IsPaid() bool { return p.message == "Transaction paid" }

// IsInlineProcessing reports whether the payment is being processed without a redirect.
func (p *RegisteredPayment) IsInlineProcessing() bool { return p.message == "Internal processing" }

// IPKSeF returns the KSeF invoice identifier, empty when the response carries none.
func (p *RegisteredPayment) IPKSeF() string { return p.ipksef }

// CardRecurringAlias returns the stored-card alias registered with this payment, if any.
func (p *RegisteredPayment) CardRecurringAlias() string {
	return stringField(objectField(p.raw, "additionalInfo"), "card_recurring_alias")
}

// RecurringAlias returns the alias of the recurring payment registered with this
// payment (RegisterPaymentRequest.RecurringRegistration), empty when the response
// carries none.
func (p *RegisteredPayment) RecurringAlias() string {
	return strictString(p.recurringRegistration(), "alias")
}

// RecurringMethods returns the payment methods of the recurring payment
// registered with this payment, e.g. [blik].
func (p *RegisteredPayment) RecurringMethods() []string {
	methods := []string{}
	for _, method := range arrayField(p.recurringRegistration(), "methods") {
		if text, ok := method.(string); ok {
			methods = append(methods, text)
		}
	}
	return methods
}

// Multibanco returns the Multibanco reference to show the payer (registration
// with the Multibanco channel), nil when the response carries none.
func (p *RegisteredPayment) Multibanco() *MultibancoReference {
	if multibanco := objectOrNil(p.raw, "multibanco"); multibanco != nil {
		return &MultibancoReference{raw: multibanco}
	}
	return nil
}

// PaidTransaction returns the already paid transaction of a "Transaction paid"
// answer (additionalInfo.transaction), nil when the response carries none.
func (p *RegisteredPayment) PaidTransaction() *RegisteredTransaction {
	if transaction := objectOrNil(phpObject(p.raw["additionalInfo"]), "transaction"); transaction != nil {
		return &RegisteredTransaction{raw: transaction, currency: p.currency}
	}
	return nil
}

func (p *RegisteredPayment) recurringRegistration() map[string]any {
	return phpObject(phpObject(p.raw["additionalInfo"])["recurring_registration"])
}

// Raw returns the decoded response body.
func (p *RegisteredPayment) Raw() map[string]any { return p.raw }

// MultibancoReference is the Multibanco payment reference returned by the
// registration (multibanco): the payer pays it at an ATM or in online banking
// with the entity and reference numbers, so you can show them without
// redirecting to the gateway.
type MultibancoReference struct {
	raw map[string]any
}

// Entity returns the entity number (5 digits), empty when missing.
func (m *MultibancoReference) Entity() string { return stringOrInt(m.raw, "entity") }

// Reference returns the reference number (9 digits, without spaces), empty when missing.
func (m *MultibancoReference) Reference() string { return stringOrInt(m.raw, "reference") }

// Amount returns the amount to pay in Currency() and whether the API sent a parsable one.
func (m *MultibancoReference) Amount() (Money, bool) {
	return moneyFromAPI(m.raw["amount"], m.Currency())
}

// Currency returns the currency of the amount, EUR when missing or invalid.
func (m *MultibancoReference) Currency() Currency {
	if code := stringOrInt(m.raw, "currency"); Currency(code).Valid() {
		return Currency(code)
	}
	return CurrencyEUR
}

// ExpiresAt returns the expiry in UTC, ISO 8601 (e.g. 2026-11-21T10:00:00+00:00).
func (m *MultibancoReference) ExpiresAt() string { return stringOrInt(m.raw, "expires_at") }

// Raw returns the multibanco object exactly as the API sent it.
func (m *MultibancoReference) Raw() map[string]any { return m.raw }

// RegisteredTransaction is the transaction returned with a registration
// answered "Transaction paid" (additionalInfo.transaction) - the payment was
// already paid (e.g. a repeated registration). Its statuses differ from
// Transaction.Status(): pending, paid, captured, canceled. Every string is
// empty when the API sent none.
type RegisteredTransaction struct {
	raw      map[string]any
	currency Currency
}

// PaymentID returns the transaction identifier.
func (t *RegisteredTransaction) PaymentID() string { return stringOrInt(t.raw, "payment_id") }

// Status returns the transaction status, preserved verbatim even when unknown.
func (t *RegisteredTransaction) Status() RegisteredTransactionStatus {
	return RegisteredTransactionStatus(stringOrInt(t.raw, "status"))
}

// IsPaid reports whether the status is paid or captured.
func (t *RegisteredTransaction) IsPaid() bool {
	return t.Status() == RegisteredTransactionStatusPaid || t.Status() == RegisteredTransactionStatusCaptured
}

// Value returns the amount in the currency of the registered amount and
// whether the API sent a parsable one.
func (t *RegisteredTransaction) Value() (Money, bool) {
	return moneyFromAPI(t.raw["value"], t.currency)
}

// Email returns the payer's email.
func (t *RegisteredTransaction) Email() string { return stringOrInt(t.raw, "email") }

// Description returns the payment description.
func (t *RegisteredTransaction) Description() string { return stringOrInt(t.raw, "description") }

// Name returns the payer's first name.
func (t *RegisteredTransaction) Name() string { return stringOrInt(t.raw, "name") }

// Surname returns the payer's last name.
func (t *RegisteredTransaction) Surname() string { return stringOrInt(t.raw, "surname") }

// PID returns the provider transaction identifier, when known.
func (t *RegisteredTransaction) PID() string { return stringOrInt(t.raw, "pid") }

// MTID returns the mtid of the transaction.
func (t *RegisteredTransaction) MTID() string { return stringOrInt(t.raw, "mtid") }

// CreationDate returns the creation timestamp as reported by the API.
func (t *RegisteredTransaction) CreationDate() string { return stringOrInt(t.raw, "creation_date") }

// PaymentDate returns the payment timestamp as reported by the API.
func (t *RegisteredTransaction) PaymentDate() string { return stringOrInt(t.raw, "payment_date") }

// Raw returns the transaction object exactly as the API sent it.
func (t *RegisteredTransaction) Raw() map[string]any { return t.raw }

// Transaction is the state of a registered payment.
type Transaction struct {
	raw                   map[string]any
	id                    string
	value                 Money
	status                TransactionStatus
	paymentMethod         string
	creationDate          string
	paymentDate           string
	settled               bool
	refunded              bool
	refundedAmount        Money
	availableRefundAmount Money
	fullyRefunded         bool
	direct                bool
	gatewayID             string
	payer                 map[string]any
	refunds               []*TransactionRefund
}

func transactionFromAPI(data map[string]any) *Transaction {
	inner := objectField(data, "transaction")
	transaction := &Transaction{
		raw:                   data,
		id:                    stringField(inner, "id"),
		value:                 moneyField(inner, "value", CurrencyPLN),
		status:                TransactionStatus(stringField(inner, "status")),
		paymentMethod:         stringField(inner, "payment_method"),
		creationDate:          stringField(inner, "creation_date"),
		paymentDate:           stringField(inner, "payment_date"),
		settled:               boolField(inner, "settled"),
		refunded:              boolField(inner, "refunded"),
		refundedAmount:        moneyField(inner, "refunded_amount", CurrencyPLN),
		availableRefundAmount: moneyField(inner, "available_refund_amount", CurrencyPLN),
		fullyRefunded:         boolField(inner, "fully_refunded"),
		direct:                boolField(inner, "direct"),
		gatewayID:             stringField(inner, "gateway_id"),
		payer:                 objectField(data, "payer"),
	}
	for _, entry := range arrayField(data, "refunds") {
		if refund, ok := entry.(map[string]any); ok {
			transaction.refunds = append(transaction.refunds, transactionRefundFromAPI(refund))
		}
	}
	return transaction
}

// ID returns the transaction identifier.
func (t *Transaction) ID() string { return t.id }

// Value returns the transaction amount.
func (t *Transaction) Value() Money { return t.value }

// Status returns the transaction status, preserved verbatim even when unknown.
func (t *Transaction) Status() TransactionStatus { return t.status }

// IsPaid reports whether the transaction is paid or captured.
func (t *Transaction) IsPaid() bool {
	return t.status == TransactionStatusPaid || t.status == TransactionStatusCaptured
}

// Rate returns the dpay commission rate in percent as sent (e.g. 1.50), empty
// when missing.
func (t *Transaction) Rate() string {
	switch rate := t.inner()["rate"].(type) {
	case string, float64, int, int64:
		return php.Strval(rate)
	}
	return ""
}

// MinimalFee returns the minimal commission as sent by the API (minimal_fee,
// e.g. 20 = 0.20 PLN), nil when missing or not an integer.
func (t *Transaction) MinimalFee() *int64 { return integerOrDigits(t.inner(), "minimal_fee") }

// PermanentFee returns the added fixed commission as sent by the API
// (permanent_fee, cards only), nil when missing or not an integer.
func (t *Transaction) PermanentFee() *int64 { return integerOrDigits(t.inner(), "permanent_fee") }

// URLs returns the return and IPN addresses of the registration, nil when the
// API sent none.
func (t *Transaction) URLs() *TransactionURLs {
	urls := objectOrNil(t.inner(), "urls")
	if urls == nil {
		return nil
	}
	return &TransactionURLs{
		success: strictString(urls, "success"),
		fail:    strictString(urls, "fail"),
		ipn:     strictString(urls, "ipn"),
	}
}

func (t *Transaction) inner() map[string]any { return phpObject(t.raw["transaction"]) }

var integerPattern = regexp.MustCompile(`^-?\d+$`)

// integerOrDigits returns data[key] when it is a JSON integer or a string of
// digits with an optional minus sign, else nil.
func integerOrDigits(data map[string]any, key string) *int64 {
	if value, ok := jsonInt(data[key]); ok {
		return &value
	}
	if text, ok := data[key].(string); ok && integerPattern.MatchString(text) {
		parsed, err := strconv.ParseInt(text, 10, 64)
		if err != nil {
			// PHP saturates an integer cast that overflows
			parsed = math.MaxInt64
			if strings.HasPrefix(text, "-") {
				parsed = math.MinInt64
			}
		}
		return &parsed
	}
	return nil
}

// TransactionURLs are the return and IPN addresses stored with a transaction
// (transaction.urls of Payments.Details). An empty address means none.
type TransactionURLs struct {
	success string
	fail    string
	ipn     string
}

// Success returns the success URL.
func (u *TransactionURLs) Success() string { return u.success }

// Fail returns the failure URL.
func (u *TransactionURLs) Fail() string { return u.fail }

// IPN returns the IPN URL, empty when the payment was registered without one.
func (u *TransactionURLs) IPN() string { return u.ipn }

// PaymentMethod returns the method the payer used.
func (t *Transaction) PaymentMethod() string { return t.paymentMethod }

// CreationDate returns the creation timestamp as reported by the API.
func (t *Transaction) CreationDate() string { return t.creationDate }

// PaymentDate returns the payment timestamp as reported by the API.
func (t *Transaction) PaymentDate() string { return t.paymentDate }

// IsSettled reports whether the funds were settled to the merchant.
func (t *Transaction) IsSettled() bool { return t.settled }

// IsRefunded reports whether any refund was issued.
func (t *Transaction) IsRefunded() bool { return t.refunded }

// RefundedAmount returns the total amount already refunded.
func (t *Transaction) RefundedAmount() Money { return t.refundedAmount }

// AvailableRefundAmount returns how much can still be refunded.
func (t *Transaction) AvailableRefundAmount() Money { return t.availableRefundAmount }

// IsFullyRefunded reports whether the whole amount was refunded.
func (t *Transaction) IsFullyRefunded() bool { return t.fullyRefunded }

// IsDirect reports whether the payment was a direct transaction.
func (t *Transaction) IsDirect() bool { return t.direct }

// GatewayID returns the gateway identifier.
func (t *Transaction) GatewayID() string { return t.gatewayID }

// Payer returns the payer object exactly as the API sent it.
func (t *Transaction) Payer() map[string]any { return t.payer }

// Refunds returns the refunds issued against this transaction.
func (t *Transaction) Refunds() []*TransactionRefund { return t.refunds }

// Raw returns the decoded response body.
func (t *Transaction) Raw() map[string]any { return t.raw }

// TransactionRefund is a single refund listed in transaction details.
type TransactionRefund struct {
	raw          map[string]any
	paymentID    string
	value        Money
	status       TransactionStatus
	creationDate string
	paymentDate  string
}

func transactionRefundFromAPI(data map[string]any) *TransactionRefund {
	status := stringField(data, "status")
	if status == "" {
		status = string(TransactionStatusPaid)
	}
	return &TransactionRefund{
		raw:          data,
		paymentID:    stringField(data, "payment_id"),
		value:        moneyField(data, "value", CurrencyPLN),
		status:       TransactionStatus(status),
		creationDate: stringField(data, "creation_date"),
		paymentDate:  stringField(data, "payment_date"),
	}
}

// PaymentID returns the refund payment identifier.
func (r *TransactionRefund) PaymentID() string { return r.paymentID }

// Value returns the refunded amount.
func (r *TransactionRefund) Value() Money { return r.value }

// Status returns the refund status.
func (r *TransactionRefund) Status() TransactionStatus { return r.status }

// CreationDate returns the creation timestamp as reported by the API.
func (r *TransactionRefund) CreationDate() string { return r.creationDate }

// PaymentDate returns the payment timestamp as reported by the API.
func (r *TransactionRefund) PaymentDate() string { return r.paymentDate }

// Raw returns the decoded refund object.
func (r *TransactionRefund) Raw() map[string]any { return r.raw }
