package dpay

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"
)

func webhookHeaders(vector webhookVector, signature string) http.Header {
	if signature == "" {
		signature = vector.Signature
	}
	// deliberately not canonical: the verifier matches header names in any letter case
	return http.Header{
		"Webhook-Id":        {vector.ID},
		"WEBHOOK-TIMESTAMP": {strconv.FormatInt(vector.Timestamp, 10)},
		"webhook-signature": {signature},
	}
}

func verifierAt(vector webhookVector, offset int64, secrets ...string) WebhookVerifier {
	return WebhookVerifier{
		Secrets: secrets,
		Now:     func() time.Time { return time.Unix(vector.Timestamp+offset, 0) },
	}
}

func TestWebhookVerifierReturnsTheEvent(t *testing.T) {
	vector := loadAPIVectors(t).Webhook

	event, err := verifierAt(vector, 10, vector.Secret).ConstructEvent([]byte(vector.Body), webhookHeaders(vector, ""))
	if err != nil {
		t.Fatal(err)
	}
	if event.ID() != vector.ID || event.Type() != WebhookEventTypePaymentSucceeded {
		t.Fatalf("event = %v", event.Raw())
	}
	if event.ObjectType() != "payment" || event.Object()["amount"] != float64(1000) {
		t.Fatalf("object = %v", event.Object())
	}
	if !event.IsLivemode() {
		t.Fatal("livemode defaults to true")
	}
}

func TestWebhookVerifierAcceptsCanonicalHeadersAndTheSecretWithoutPrefix(t *testing.T) {
	vector := loadAPIVectors(t).Webhook
	headers := http.Header{}
	headers.Set("webhook-id", vector.ID)
	headers.Set("webhook-timestamp", strconv.FormatInt(vector.Timestamp, 10))
	headers.Add("webhook-signature", vector.Signature)
	headers.Add("webhook-signature", "v1,ignored-second-value")

	if err := verifierAt(vector, 0, strings.TrimPrefix(vector.Secret, "whsec_")).Verify([]byte(vector.Body), headers); err != nil {
		t.Fatal(err)
	}
}

func TestWebhookVerifierAcceptsEitherSignatureDuringARotation(t *testing.T) {
	vector := loadAPIVectors(t).Webhook

	// only the old secret on the receiving side, the header carries two signatures
	if err := verifierAt(vector, 0, vector.OldSecret).Verify([]byte(vector.Body), webhookHeaders(vector, vector.RotationSignature)); err != nil {
		t.Fatalf("old secret: %v", err)
	}
	// both secrets on the receiving side
	if err := verifierAt(vector, 0, vector.OldSecret, vector.Secret).Verify([]byte(vector.Body), webhookHeaders(vector, "")); err != nil {
		t.Fatalf("both secrets: %v", err)
	}
}

func TestWebhookVerifierRejections(t *testing.T) {
	vector := loadAPIVectors(t).Webhook
	missing := webhookHeaders(vector, "")
	delete(missing, "webhook-signature")
	emptyID := webhookHeaders(vector, "")
	emptyID["Webhook-Id"] = []string{""}
	badTimestamp := webhookHeaders(vector, "")
	badTimestamp["WEBHOOK-TIMESTAMP"] = []string{"-1790503500"}

	cases := []struct {
		name     string
		body     string
		headers  http.Header
		offset   int64
		secrets  []string
		sentinel error
		message  string
	}{
		{"tampered body", strings.Replace(vector.Body, "1000", "100000", 1), webhookHeaders(vector, ""), 0,
			[]string{vector.Secret}, ErrSignature, "dpay: No valid webhook signature found"},
		{"old timestamp", vector.Body, webhookHeaders(vector, ""), 301,
			[]string{vector.Secret}, ErrSignature, "dpay: Webhook timestamp is outside the tolerance zone"},
		{"future timestamp", vector.Body, webhookHeaders(vector, ""), -301,
			[]string{vector.Secret}, ErrSignature, "dpay: Webhook timestamp is outside the tolerance zone"},
		{"missing header", vector.Body, missing, 0,
			[]string{vector.Secret}, ErrSignature, "dpay: Missing webhook-id, webhook-timestamp or webhook-signature header"},
		{"empty header", vector.Body, emptyID, 0,
			[]string{vector.Secret}, ErrSignature, "dpay: Missing webhook-id, webhook-timestamp or webhook-signature header"},
		{"timestamp not digits", vector.Body, badTimestamp, 0,
			[]string{vector.Secret}, ErrSignature, "dpay: Invalid webhook-timestamp header"},
		{"other signature version", vector.Body, webhookHeaders(vector, "v2,"+strings.TrimPrefix(vector.Signature, "v1,")), 0,
			[]string{vector.Secret}, ErrSignature, "dpay: No valid webhook signature found"},
		{"wrong secret", vector.Body, webhookHeaders(vector, ""), 0,
			[]string{vector.OldSecret}, ErrSignature, "dpay: No valid webhook signature found"},
		{"no secrets", vector.Body, webhookHeaders(vector, ""), 0,
			nil, ErrSignature, "dpay: No valid webhook signature found"},
		{"secret is not base64", vector.Body, webhookHeaders(vector, ""), 0,
			[]string{"whsec_***"}, ErrInvalidArgument, "dpay: Webhook secret must be the whsec_ value from the dpay panel"},
		{"empty secret", vector.Body, webhookHeaders(vector, ""), 0,
			[]string{vector.Secret, "whsec_"}, ErrInvalidArgument, "dpay: Webhook secret must be the whsec_ value from the dpay panel"},
	}
	for _, testCase := range cases {
		err := verifierAt(vector, testCase.offset, testCase.secrets...).Verify([]byte(testCase.body), testCase.headers)
		if !errors.Is(err, testCase.sentinel) || err.Error() != testCase.message {
			t.Errorf("%s: err = %v, want %q", testCase.name, err, testCase.message)
		}
	}
}

func TestWebhookVerifierTolerance(t *testing.T) {
	vector := loadAPIVectors(t).Webhook
	verifier := verifierAt(vector, 600, vector.Secret)
	if err := verifier.Verify([]byte(vector.Body), webhookHeaders(vector, "")); !errors.Is(err, ErrSignature) {
		t.Fatalf("default tolerance: err = %v", err)
	}
	verifier.Tolerance = 10 * time.Minute
	if err := verifier.Verify([]byte(vector.Body), webhookHeaders(vector, "")); err != nil {
		t.Fatalf("custom tolerance: %v", err)
	}
	verifier = verifierAt(vector, 300, vector.Secret)
	if err := verifier.Verify([]byte(vector.Body), webhookHeaders(vector, "")); err != nil {
		t.Fatalf("300 seconds are within the default tolerance: %v", err)
	}
}

// The official test vector of the Standard Webhooks specification.
func TestWebhookVerifierStandardWebhooksVector(t *testing.T) {
	headers := http.Header{}
	headers.Set("webhook-id", "msg_p5jXN8AQM9LWM0D4loKWxJek")
	headers.Set("webhook-timestamp", "1614265330")
	headers.Set("webhook-signature", "v1,g0hM9SsE+OTPJTGt/tmIKtSyZlE3uFJELVlNIOLJ1OE=")
	verifier := WebhookVerifier{
		Secrets: []string{"whsec_MfKQ9r8GKYqrTwjUPD8ILPZIo2LaLaSw"},
		Now:     func() time.Time { return time.Unix(1614265330, 0) },
	}
	event, err := verifier.ConstructEvent([]byte(`{"test": 2432232314}`), headers)
	if err != nil {
		t.Fatal(err)
	}
	if event.Raw()["test"] != float64(2432232314) || event.ID() != "" {
		t.Fatalf("event = %v", event.Raw())
	}
}

func TestVerifyWebhookUsesTheCurrentTime(t *testing.T) {
	vector := loadAPIVectors(t).Webhook
	if _, err := VerifyWebhook([]byte(vector.Body), webhookHeaders(vector, ""), vector.Secret); err == nil ||
		err.Error() != "dpay: Webhook timestamp is outside the tolerance zone" {
		t.Fatalf("a vector from 2026-09 must be too old for time.Now: %v", err)
	}
}

func TestWebhookVerifierRejectsAPayloadThatIsNotAnObject(t *testing.T) {
	vector := loadAPIVectors(t).Webhook
	body := "not json"
	headers := webhookHeaders(vector, signWebhook(t, vector.Secret, vector.ID, vector.Timestamp, body))
	_, err := verifierAt(vector, 0, vector.Secret).ConstructEvent([]byte(body), headers)
	if !errors.Is(err, ErrSignature) || err.Error() != "dpay: Invalid webhook payload" {
		t.Fatalf("err = %v", err)
	}
}

// signWebhook signs independently of the verifier: v1,base64(HMAC-SHA256(key, id.timestamp.body)).
func signWebhook(t *testing.T, secret, id string, timestamp int64, body string) string {
	t.Helper()
	key, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(secret, "whsec_"))
	if err != nil {
		t.Fatal(err)
	}
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(id + "." + strconv.FormatInt(timestamp, 10) + "." + body))
	return "v1," + base64.StdEncoding.EncodeToString(mac.Sum(nil))
}

func TestWebhookEventAccessors(t *testing.T) {
	event, _ := webhookEventFromJSON(map[string]any{
		"id": "evt_01k6a8q2m4pz7h8c3v5n9t2x6y", "type": "recurring_payment.canceled", "api_version": "2026-10-01",
		"created": "2026-09-27T10:05:00Z", "livemode": false, "service": "shop", "merchant_ref": "m-1",
		"data": map[string]any{"object": map[string]any{"object": "recurring_payment", "alias": "SUB-1"}},
	})
	if event.Type() != WebhookEventTypeRecurringPaymentCanceled || event.APIVersion() != "2026-10-01" ||
		event.Created() != "2026-09-27T10:05:00Z" || event.IsLivemode() || event.Service() != "shop" ||
		event.MerchantRef() != "m-1" || event.ObjectType() != "recurring_payment" || event.Object()["alias"] != "SUB-1" {
		t.Fatalf("event = %v", event.Raw())
	}

	sparse, _ := webhookEventFromJSON(map[string]any{"id": 5, "livemode": nil, "data": []any{}})
	if sparse.ID() != "" || !sparse.IsLivemode() || len(sparse.Object()) != 0 || sparse.ObjectType() != "" {
		t.Fatalf("sparse = %v", sparse.Raw())
	}
	if _, ok := webhookEventFromJSON("text"); ok {
		t.Fatal("a string is not an event")
	}
}

func TestWebhookTargetValidate(t *testing.T) {
	valid := WebhookTarget{URL: "HTTPS://shop.test/webhooks", Events: []WebhookEventType{WebhookEventTypePayoutPaid}}
	if err := valid.Validate(); err != nil {
		t.Fatalf("valid target: %v", err)
	}
	cases := map[string]WebhookTarget{
		`dpay: Webhook URL "http://shop.test/webhooks" must be a valid https:// URL`: {URL: "http://shop.test/webhooks"},
		`dpay: Webhook URL "https://" must be a valid https:// URL`:                  {URL: "https://"},
		"dpay: Webhook URL must be at most 500 characters":                           {URL: "https://shop.test/" + strings.Repeat("a", 483)},
		"dpay: Webhook events must be distinct": {URL: "https://shop.test/webhooks", Events: []WebhookEventType{
			WebhookEventTypePaymentFailed, WebhookEventTypePaymentFailed,
		}},
		`dpay: Event "merchant.updated" is not allowed in the webhook object of a request`: {
			URL: "https://shop.test/webhooks", Events: []WebhookEventType{"merchant.updated"},
		},
	}
	for want, target := range cases {
		if err := target.Validate(); !errors.Is(err, ErrInvalidArgument) || err.Error() != want {
			t.Errorf("err = %v, want %q", err, want)
		}
	}
	if err := (WebhookTarget{URL: "https://shop.test/" + strings.Repeat("a", 482)}).Validate(); err != nil {
		t.Fatalf("500 characters are allowed: %v", err)
	}
}

func TestWebhookTargetBodyPutsTheURLFirst(t *testing.T) {
	target := WebhookTarget{URL: "https://shop.test/webhooks", Events: []WebhookEventType{WebhookEventTypeRefundFailed}}
	if got := bodyJSON(t, target.toBody()); got != `{"url":"https://shop.test/webhooks","events":["refund.failed"]}` {
		t.Fatalf("toBody() = %s", got)
	}
	if got := bodyJSON(t, (WebhookTarget{URL: "https://shop.test/webhooks"}).toBody()); got != `{"url":"https://shop.test/webhooks"}` {
		t.Fatalf("without events = %s", got)
	}
}

func TestEventTypeLists(t *testing.T) {
	if len(MerchantEventTypes()) != 11 || len(PaymentRegistrationEventTypes()) != 9 ||
		len(RefundEventTypes()) != 2 || len(CaptureEventTypes()) != 1 {
		t.Fatal("event type lists changed")
	}
	for _, list := range [][]WebhookEventType{PaymentRegistrationEventTypes(), RefundEventTypes(), CaptureEventTypes()} {
		for _, eventType := range list {
			if !containsEventType(MerchantEventTypes(), eventType) {
				t.Errorf("%s is not a merchant event type", eventType)
			}
		}
	}
	MerchantEventTypes()[0] = "changed"
	if MerchantEventTypes()[0] != WebhookEventTypePaymentSucceeded {
		t.Fatal("the lists must not be shared")
	}
}
