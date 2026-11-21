package dpay

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"strings"

	"github.com/dpayglobal/dpay-go-sdk/internal/php"
)

// IPNAck is the exact body dpay expects in an IPN response. The HTTP status is
// ignored: anything other than this body is treated as a failed delivery.
const IPNAck = "OK"

// VerifyIPN parses and authenticates an IPN notification. It is the single
// entry point for every IPN type: a merchant without alias_ipn_url receives
// transfer and alias_update notifications on url_ipn, so branch on Type().
// The signatures are:
//   - transfer, dcb: sha256(id . secret . amount . email . type . attempt . version . custom), dcb without email;
//   - alias_update: sha256(id . secret . type . version).
//
// Always compare the amount with your own order before marking it as paid.
func VerifyIPN(rawBody []byte, secretHash string) (*IPNEvent, error) {
	var payload map[string]any
	if err := json.Unmarshal(rawBody, &payload); err != nil {
		return nil, &SignatureError{message: "Invalid IPN payload"}
	}
	eventType := scalarString(payload["type"])
	required := []string{"id", "amount", "type", "attempt", "version", "signature"}
	if eventType == string(IPNTypeAliasUpdate) {
		required = []string{"id", "type", "version", "signature"}
	}
	for _, field := range required {
		if value, present := payload[field]; !present || value == nil {
			return nil, &SignatureError{message: "Invalid IPN payload"}
		}
	}
	signature, ok := payload["signature"].(string)
	if !ok {
		return nil, &SignatureError{message: "Invalid IPN payload"}
	}

	var parts []string
	if eventType == string(IPNTypeAliasUpdate) {
		parts = []string{scalarString(payload["id"]), secretHash, eventType, numericString(payload["version"])}
	} else {
		parts = []string{scalarString(payload["id"]), secretHash, scalarString(payload["amount"])}
		if eventType != string(IPNTypeDCB) {
			parts = append(parts, scalarString(payload["email"]))
		}
		parts = append(parts,
			eventType,
			numericString(payload["attempt"]),
			numericString(payload["version"]),
			scalarString(payload["custom"]),
		)
	}

	digest := sha256.Sum256([]byte(strings.Join(parts, "")))
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

// ID returns the transaction identifier - for alias_update the payment that
// registered the alias.
func (e *IPNEvent) ID() string { return stringField(e.raw, "id") }

// Amount returns the amount as the raw decimal string from the payload. The
// payload carries no currency, so compare this with your own order. It is
// empty for alias_update: its signature does not cover an amount, so a value
// in the payload is ignored.
func (e *IPNEvent) Amount() string { return e.signedTransferField("amount") }

// Email returns the payer address, empty when absent and for alias_update
// (not signed there).
func (e *IPNEvent) Email() string { return e.signedTransferField("email") }

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

// IsAliasUpdate reports whether this is a BLIK OneClick alias or recurring
// payment status change - details in AliasUpdate().
func (e *IPNEvent) IsAliasUpdate() bool { return e.Type() == IPNTypeAliasUpdate }

// AliasUpdate returns the alias change of an alias_update IPN, nil for other types.
func (e *IPNEvent) AliasUpdate() *AliasUpdate {
	if !e.IsAliasUpdate() {
		return nil
	}
	return &AliasUpdate{raw: e.raw}
}

// Attempt returns which delivery attempt this is; 0 for alias_update (not
// signed there).
func (e *IPNEvent) Attempt() int64 {
	if e.IsAliasUpdate() {
		return 0
	}
	return intField(e.raw, "attempt")
}

// Version returns the notification format version.
func (e *IPNEvent) Version() int64 { return intField(e.raw, "version") }

// Custom returns the merchant reference passed at registration, empty for
// alias_update (not signed there).
func (e *IPNEvent) Custom() string { return e.signedTransferField("custom") }

// signedTransferField returns a field the transfer signature covers. The
// alias_update signature does not cover it, so for alias_update it is empty
// even when the payload carries it - otherwise a captured alias_update could
// be replayed with an injected amount.
func (e *IPNEvent) signedTransferField(key string) string {
	if e.IsAliasUpdate() {
		return ""
	}
	return stringField(e.raw, key)
}

// CapturePaymentID returns the capture identifier, empty for other event types.
//
// Deprecated: dpay no longer sends capture IPNs - use the payment.captured
// webhook event (WebhookEventTypePaymentCaptured).
func (e *IPNEvent) CapturePaymentID() string { return stringField(e.raw, "capture_payment_id") }

// Signature returns the signature the notification carried.
func (e *IPNEvent) Signature() string { return stringField(e.raw, "signature") }

// Raw returns the decoded payload.
func (e *IPNEvent) Raw() map[string]any { return e.raw }

// AliasUpdate is the payload of an alias_update IPN: a BLIK OneClick alias
// (AliasTypeUID) or a recurring payment (AliasTypePayID) changed its status. It
// is sent to RegisterPaymentRequest.AliasIPNURL, or to the IPN URL without it.
//
// The signature covers only id, type and version - none of the fields below.
// Anyone who has seen one alias_update of a payment can replay it with a
// different status, so before acting on a change (e.g. stopping a
// subscription) confirm the current state with client.Recurring.Status() or
// client.Blik.Alias(). Every accessor returns an empty value when the field is
// null or missing.
type AliasUpdate struct {
	raw map[string]any
}

// ChangeType returns ALIAS_REGISTER, ALIAS_UPDATE, ALIAS_UNREGISTER, ALIAS_EXPIRED or ALIAS_DECLINED.
func (a *AliasUpdate) ChangeType() AliasChangeType {
	return AliasChangeType(stringOrInt(a.raw, "change_type"))
}

// AliasType returns UID (BLIK OneClick) or PAYID (recurring payment).
func (a *AliasUpdate) AliasType() AliasType { return AliasType(stringOrInt(a.raw, "alias_type")) }

// AliasStatus returns ACTIVE, INACTIVE, UNREGISTERED, EXPIRED or DECLINED.
func (a *AliasUpdate) AliasStatus() AliasStatus {
	return AliasStatus(stringOrInt(a.raw, "alias_status"))
}

// IsActive reports whether the alias status is ACTIVE.
func (a *AliasUpdate) IsActive() bool { return a.AliasStatus() == AliasStatusActive }

// AliasKey returns the alias identifier at BLIK - not the alias value you send with payments.
func (a *AliasUpdate) AliasKey() string { return stringOrInt(a.raw, "alias_key") }

// AliasLabel returns the alias label shown to the customer.
func (a *AliasUpdate) AliasLabel() string { return stringOrInt(a.raw, "alias_label") }

// AliasExpireAt returns the alias expiry as sent by the API.
func (a *AliasUpdate) AliasExpireAt() string { return stringOrInt(a.raw, "alias_expire_at") }

// ChangeReasonCategory returns the category of the change reason.
func (a *AliasUpdate) ChangeReasonCategory() string {
	return stringOrInt(a.raw, "change_reason_category")
}

// ChangeReasonMessage returns the description of the change reason.
func (a *AliasUpdate) ChangeReasonMessage() string {
	return stringOrInt(a.raw, "change_reason_message")
}

// Raw returns the decoded payload.
func (a *AliasUpdate) Raw() map[string]any { return a.raw }
