package dpay

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/dpayglobal/dpay-go-sdk/internal/php"
	"github.com/dpayglobal/dpay-go-sdk/internal/wire"
)

// MerchantEventTypes returns the events a merchant endpoint can subscribe to
// and the Events API can filter on.
func MerchantEventTypes() []WebhookEventType {
	return []WebhookEventType{
		WebhookEventTypePaymentSucceeded,
		WebhookEventTypePaymentFailed,
		WebhookEventTypePaymentCaptured,
		WebhookEventTypeRefundSucceeded,
		WebhookEventTypeRefundFailed,
		WebhookEventTypeRecurringPaymentActivated,
		WebhookEventTypeRecurringPaymentCanceled,
		WebhookEventTypeRecurringPaymentExpired,
		WebhookEventTypeRecurringPaymentDeclined,
		WebhookEventTypePayoutPaid,
		WebhookEventTypePayoutFailed,
	}
}

// PaymentRegistrationEventTypes returns the events allowed in the webhook
// object of a payment registration.
func PaymentRegistrationEventTypes() []WebhookEventType {
	return []WebhookEventType{
		WebhookEventTypePaymentSucceeded,
		WebhookEventTypePaymentFailed,
		WebhookEventTypePaymentCaptured,
		WebhookEventTypeRefundSucceeded,
		WebhookEventTypeRefundFailed,
		WebhookEventTypeRecurringPaymentActivated,
		WebhookEventTypeRecurringPaymentCanceled,
		WebhookEventTypeRecurringPaymentExpired,
		WebhookEventTypeRecurringPaymentDeclined,
	}
}

// RefundEventTypes returns the events allowed in the webhook object of a refund.
func RefundEventTypes() []WebhookEventType {
	return []WebhookEventType{WebhookEventTypeRefundSucceeded, WebhookEventTypeRefundFailed}
}

// CaptureEventTypes returns the events allowed in the webhook object of a card capture.
func CaptureEventTypes() []WebhookEventType {
	return []WebhookEventType{WebhookEventTypePaymentCaptured}
}

func containsEventType(types []WebhookEventType, eventType WebhookEventType) bool {
	for _, candidate := range types {
		if candidate == eventType {
			return true
		}
	}
	return false
}

func assertEventsAllowed(events, allowed []WebhookEventType, context string) error {
	for _, event := range events {
		if !containsEventType(allowed, event) {
			return newValidationError(fmt.Sprintf("Event %q is not allowed in the webhook object of %s", string(event), context))
		}
	}
	return nil
}

// WebhookTarget is a per-request webhook address: the webhook object of a
// payment registration, a refund or a card capture. The events of that payment
// go to this URL, signed with the webhook secret of the service, on top of the
// endpoints set in the panel.
type WebhookTarget struct {
	// URL is an https:// address of at most 500 characters.
	URL string
	// Events narrows the events sent to URL; empty means every event the request allows.
	Events []WebhookEventType
}

// Validate reports whether the URL is an https URL of at most 500 characters
// and the events are distinct merchant event types.
func (t WebhookTarget) Validate() error {
	if len(t.URL) > 500 {
		return newValidationError("Webhook URL must be at most 500 characters")
	}
	if !validURL(t.URL) || len(t.URL) < 8 || !strings.EqualFold(t.URL[:8], "https://") {
		return newValidationError(fmt.Sprintf("Webhook URL %q must be a valid https:// URL", t.URL))
	}
	if hasDuplicates(t.Events) {
		return newValidationError("Webhook events must be distinct")
	}
	return assertEventsAllowed(t.Events, MerchantEventTypes(), "a request")
}

func (t WebhookTarget) validateFor(allowed []WebhookEventType, context string) error {
	if err := t.Validate(); err != nil {
		return err
	}
	return assertEventsAllowed(t.Events, allowed, context)
}

// toBody puts url first, then events: the refund checksum hashes the values in this order.
func (t WebhookTarget) toBody() *wire.Body {
	body := wire.NewBody()
	body.Set("url", t.URL)
	if len(t.Events) > 0 {
		events := make([]string, 0, len(t.Events))
		for _, event := range t.Events {
			events = append(events, string(event))
		}
		body.Set("events", events)
	}
	return body
}

// DefaultWebhookTolerance is how far webhook-timestamp may be from the current time.
const DefaultWebhookTolerance = 300 * time.Second

// WebhookVerifier verifies dpay webhooks (Standard Webhooks): webhook-signature
// is "v1," + base64(HMAC-SHA256(key, id.timestamp.body)), where the key is the
// base64-decoded secret without the whsec_ prefix. During a secret rotation dpay
// sends two signatures separated by a space - one match is enough. The zero
// value of Tolerance and Now means DefaultWebhookTolerance and time.Now.
type WebhookVerifier struct {
	// Secrets are the whsec_ secrets of the endpoint (or of the service for a
	// per-request webhook); list both during a rotation.
	Secrets []string
	// Tolerance is how far webhook-timestamp may be from Now.
	Tolerance time.Duration
	// Now returns the current time.
	Now func() time.Time
}

// VerifyWebhook verifies a webhook with DefaultWebhookTolerance and returns its
// event. Pass the raw request body exactly as received, before parsing it.
func VerifyWebhook(rawBody []byte, headers http.Header, secrets ...string) (*WebhookEvent, error) {
	return WebhookVerifier{Secrets: secrets}.ConstructEvent(rawBody, headers)
}

// ConstructEvent verifies the signature and returns the event. A failed check
// is a *SignatureError (ErrSignature); a secret that is not base64 is a
// *ValidationError (ErrInvalidArgument), a configuration problem.
func (v WebhookVerifier) ConstructEvent(rawBody []byte, headers http.Header) (*WebhookEvent, error) {
	if err := v.Verify(rawBody, headers); err != nil {
		return nil, err
	}
	var payload any
	if err := json.Unmarshal(rawBody, &payload); err != nil {
		return nil, &SignatureError{message: "Invalid webhook payload"}
	}
	event, ok := webhookEventFromJSON(payload)
	if !ok {
		return nil, &SignatureError{message: "Invalid webhook payload"}
	}
	return event, nil
}

// Verify checks the headers webhook-id, webhook-timestamp and webhook-signature
// (any letter case) against rawBody.
func (v WebhookVerifier) Verify(rawBody []byte, headers http.Header) error {
	id, hasID := webhookHeader(headers, "webhook-id")
	timestamp, hasTimestamp := webhookHeader(headers, "webhook-timestamp")
	signatures, hasSignatures := webhookHeader(headers, "webhook-signature")
	if !hasID || !hasTimestamp || !hasSignatures {
		return &SignatureError{message: "Missing webhook-id, webhook-timestamp or webhook-signature header"}
	}
	// digits only; a value too large for int64 saturates and fails the tolerance check, like in PHP
	sent, ok := digitsInt(timestamp)
	if !ok {
		return &SignatureError{message: "Invalid webhook-timestamp header"}
	}
	now := time.Now
	if v.Now != nil {
		now = v.Now
	}
	tolerance := v.Tolerance
	if tolerance == 0 {
		tolerance = DefaultWebhookTolerance
	}
	seconds := int64(tolerance / time.Second)
	if distance := now().Unix() - sent; distance > seconds || distance < -seconds {
		return &SignatureError{message: "Webhook timestamp is outside the tolerance zone"}
	}

	signed := []byte(id + "." + timestamp + "." + string(rawBody))
	expected := make([][]byte, 0, len(v.Secrets))
	for _, secret := range v.Secrets {
		key, err := webhookKey(secret)
		if err != nil {
			return err
		}
		mac := hmac.New(sha256.New, key)
		mac.Write(signed)
		expected = append(expected, []byte(base64.StdEncoding.EncodeToString(mac.Sum(nil))))
	}

	for _, entry := range strings.FieldsFunc(signatures, isASCIISpace) {
		version, signature, found := strings.Cut(entry, ",")
		if !found || version != "v1" {
			continue
		}
		for _, candidate := range expected {
			if hmac.Equal(candidate, []byte(signature)) {
				return nil
			}
		}
	}
	return &SignatureError{message: "No valid webhook signature found"}
}

func webhookKey(secret string) ([]byte, error) {
	key, ok := php.Base64DecodeStrict(strings.TrimPrefix(secret, "whsec_"))
	if !ok || len(key) == 0 {
		return nil, newValidationError("Webhook secret must be the whsec_ value from the dpay panel")
	}
	return key, nil
}

// webhookHeader reads the first value of a header in any letter case. The
// canonical key wins; a hand-built http.Header may hold other spellings.
func webhookHeader(headers http.Header, name string) (string, bool) {
	values, found := headers[http.CanonicalHeaderKey(name)]
	if !found {
		keys := make([]string, 0, len(headers))
		for key := range headers {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			if strings.EqualFold(key, name) {
				values, found = headers[key], true
				break
			}
		}
	}
	if !found || len(values) == 0 || values[0] == "" {
		return "", false
	}
	return values[0], true
}

func isASCIISpace(character rune) bool {
	switch character {
	case ' ', '\t', '\n', '\r', '\f', '\v':
		return true
	}
	return false
}

// WebhookEvent is the envelope of a webhook event or of an Events API entry:
// {id, type, api_version, created, livemode, service, [merchant_ref], data: {object}}.
// The object (payment, refund, recurring_payment, payout) stays a map, with
// amounts in minor units.
type WebhookEvent struct {
	raw map[string]any
}

func webhookEventFromJSON(value any) (*WebhookEvent, bool) {
	raw, ok := phpArray(value)
	if !ok {
		return nil, false
	}
	return &WebhookEvent{raw: raw}, true
}

// ID returns the event id (evt_...), the same on every delivery attempt - deduplicate on it.
func (e *WebhookEvent) ID() string { return strictString(e.raw, "id") }

// Type returns the event type, preserved verbatim even when unknown.
func (e *WebhookEvent) Type() WebhookEventType { return WebhookEventType(strictString(e.raw, "type")) }

// APIVersion returns the version of the event format, e.g. 2026-10-01.
func (e *WebhookEvent) APIVersion() string { return strictString(e.raw, "api_version") }

// Created returns the event time in UTC (YYYY-MM-DDTHH:MM:SSZ). It is not the
// delivery time, so do not use it against replays.
func (e *WebhookEvent) Created() string { return strictString(e.raw, "created") }

// IsLivemode reports whether the event comes from production; only an explicit
// false means test mode.
func (e *WebhookEvent) IsLivemode() bool {
	livemode, ok := e.raw["livemode"].(bool)
	return !ok || livemode
}

// Service returns the service name, empty for account events (payouts) and test events.
func (e *WebhookEvent) Service() string { return strictString(e.raw, "service") }

// MerchantRef returns the merchant reference, present only in events sent to a dpay Connect partner.
func (e *WebhookEvent) MerchantRef() string { return strictString(e.raw, "merchant_ref") }

// Object returns data.object, an empty map when the event carries none.
func (e *WebhookEvent) Object() map[string]any {
	return phpObject(phpObject(e.raw["data"])["object"])
}

// ObjectType returns data.object.object: payment, refund, recurring_payment,
// payout or webhook_endpoint.
func (e *WebhookEvent) ObjectType() string { return strictString(e.Object(), "object") }

// Raw returns the decoded envelope.
func (e *WebhookEvent) Raw() map[string]any { return e.raw }
