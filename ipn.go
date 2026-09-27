package dpay

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"

	"github.com/dpayglobal/dpay-go-sdk/internal/php"
)

// IPNAck is the exact body dpay expects in an IPN response. The HTTP status is
// ignored: anything other than this body is treated as a failed delivery.
const IPNAck = "OK"

// VerifyIPN parses and authenticates an IPN notification. Always compare the
// amount with your own order before marking it as paid.
func VerifyIPN(rawBody []byte, secretHash string) (*IPNEvent, error) {
	var payload map[string]any
	if err := json.Unmarshal(rawBody, &payload); err != nil {
		return nil, &SignatureError{message: "Invalid IPN payload"}
	}
	for _, field := range []string{"id", "amount", "type", "attempt", "version", "signature"} {
		if value, present := payload[field]; !present || value == nil {
			return nil, &SignatureError{message: "Invalid IPN payload"}
		}
	}
	signature, ok := payload["signature"].(string)
	if !ok {
		return nil, &SignatureError{message: "Invalid IPN payload"}
	}

	eventType := scalarString(payload["type"])
	parts := []string{scalarString(payload["id"]), secretHash, scalarString(payload["amount"])}
	if eventType != string(IPNTypeDCB) {
		parts = append(parts, scalarString(payload["email"]))
	}
	parts = append(parts,
		eventType,
		numericString(payload["attempt"]),
		numericString(payload["version"]),
		scalarString(payload["custom"]),
	)

	joined := ""
	for _, part := range parts {
		joined += part
	}
	digest := sha256.Sum256([]byte(joined))
	expected := hex.EncodeToString(digest[:])
	if subtle.ConstantTimeCompare([]byte(expected), []byte(signature)) != 1 {
		return nil, &SignatureError{message: "Invalid IPN signature"}
	}
	return &IPNEvent{raw: payload}, nil
}

func scalarString(value any) string {
	if value == nil || !php.IsScalar(value) {
		return ""
	}
	return php.Strval(value)
}

func numericString(value any) string {
	if !php.IsNumeric(value) {
		return "0"
	}
	return php.Strval(value)
}

// IPNEvent is a verified IPN notification.
type IPNEvent struct {
	raw map[string]any
}

// ID returns the transaction identifier.
func (e *IPNEvent) ID() string { return stringField(e.raw, "id") }

// Amount returns the amount as the raw decimal string from the payload. The
// payload carries no currency, so compare this with your own order.
func (e *IPNEvent) Amount() string { return stringField(e.raw, "amount") }

// Email returns the payer address, empty when absent.
func (e *IPNEvent) Email() string { return stringField(e.raw, "email") }

// Type returns the event type.
func (e *IPNEvent) Type() IPNType { return IPNType(stringField(e.raw, "type")) }

// IsTransfer reports whether this is a transfer notification.
func (e *IPNEvent) IsTransfer() bool { return e.Type() == IPNTypeTransfer }

// IsCapture reports whether this is a capture notification.
//
// Deprecated: dpay no longer sends capture IPNs - use the payment.captured
// webhook event (WebhookEventTypePaymentCaptured).
func (e *IPNEvent) IsCapture() bool { return e.Type() == IPNTypeCapture }

// IsDCB reports whether this is a direct carrier billing notification.
func (e *IPNEvent) IsDCB() bool { return e.Type() == IPNTypeDCB }

// Attempt returns which delivery attempt this is.
func (e *IPNEvent) Attempt() int64 { return intField(e.raw, "attempt") }

// Version returns the notification format version.
func (e *IPNEvent) Version() int64 { return intField(e.raw, "version") }

// Custom returns the merchant reference passed at registration.
func (e *IPNEvent) Custom() string { return stringField(e.raw, "custom") }

// CapturePaymentID returns the capture identifier, empty for other event types.
//
// Deprecated: dpay no longer sends capture IPNs - use the payment.captured
// webhook event (WebhookEventTypePaymentCaptured).
func (e *IPNEvent) CapturePaymentID() string { return stringField(e.raw, "capture_payment_id") }

// Signature returns the signature the notification carried.
func (e *IPNEvent) Signature() string { return stringField(e.raw, "signature") }

// Raw returns the decoded payload.
func (e *IPNEvent) Raw() map[string]any { return e.raw }
