package dpay

import (
	"context"
	"strings"

	"github.com/dpayglobal/dpay-go-sdk/internal/wire"
)

// PaymentService registers payments and reads transaction details.
type PaymentService struct {
	client *Client
}

// Register creates a payment and returns the redirect target or the inline result.
func (s *PaymentService) Register(ctx context.Context, request *RegisterPaymentRequest) (*RegisteredPayment, error) {
	body, err := request.toBody(s.client.service)
	if err != nil {
		return nil, err
	}
	body.Set("checksum", s.client.checksum.SecretSecond(s.client.service, []any{
		bodyString(body, "value"),
		bodyString(body, "url_success"),
		bodyString(body, "url_fail"),
		bodyString(body, "url_ipn"),
	}))

	data, err := s.client.postJSONObject(ctx, hostAPIPayments, "/api/v1_0/payments/register", body)
	if err != nil {
		return nil, err
	}
	if isRejection(data) {
		return nil, paymentRejectedFromAPI(data)
	}
	return registeredPaymentFromAPI(data), nil
}

// Details reads the current state of a transaction.
func (s *PaymentService) Details(ctx context.Context, transactionID string) (*Transaction, error) {
	body := wire.NewBody()
	body.Set("service", s.client.service)
	body.Set("transaction_id", transactionID)
	body.Set("checksum", s.client.checksum.OrderedBody(body.Values()))

	data, err := s.client.postJSONObject(ctx, hostPanel, "/api/v1/pbl/details", body)
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
	return &PaymentRejectedError{
		APIError: &APIError{
			kind:        ErrPaymentRejected,
			message:     message,
			HTTPStatus:  200,
			ErrorCode:   stringField(objectField(data, "additionalInfo"), "error"),
			FieldErrors: map[string][]string{},
			RawBody:     marshalRaw(data),
		},
		TransactionID: stringField(data, "transactionId"),
	}
}

// RegisteredPayment is the result of registering a payment.
type RegisteredPayment struct {
	raw           map[string]any
	transactionID string
	message       string
	ipksef        string
}

func registeredPaymentFromAPI(data map[string]any) *RegisteredPayment {
	return &RegisteredPayment{
		raw:           data,
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

// Raw returns the decoded response body.
func (p *RegisteredPayment) Raw() map[string]any { return p.raw }

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
