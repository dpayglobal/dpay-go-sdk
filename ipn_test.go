package dpay

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"testing"
)

func ipnSignature(parts ...string) string {
	joined := ""
	for _, part := range parts {
		joined += part
	}
	digest := sha256.Sum256([]byte(joined))
	return hex.EncodeToString(digest[:])
}

func TestVerifyIPNTransfer(t *testing.T) {
	signature := ipnSignature("tx-1", "secret", "29.99", "jan@example.com", "transfer", "1", "2", "order-1")
	body := `{"id":"tx-1","amount":"29.99","email":"jan@example.com","type":"transfer",` +
		`"attempt":1,"version":2,"custom":"order-1","signature":"` + signature + `"}`

	event, err := VerifyIPN([]byte(body), "secret")
	if err != nil {
		t.Fatal(err)
	}
	if event.ID() != "tx-1" || event.Amount() != "29.99" || event.Email() != "jan@example.com" {
		t.Fatalf("event = %+v", event)
	}
	if !event.IsTransfer() || event.IsCapture() || event.IsDCB() {
		t.Fatal("type helpers broken")
	}
	if event.Attempt() != 1 || event.Version() != 2 || event.Custom() != "order-1" {
		t.Fatal("metadata broken")
	}
	if event.Signature() != signature {
		t.Fatal("signature lost")
	}
	if event.Type() != IPNTypeTransfer {
		t.Fatal("Type broken")
	}
}

func TestVerifyIPNDCBOmitsEmail(t *testing.T) {
	signature := ipnSignature("tx-2", "secret", "10.50", "dcb", "3", "1", "")
	body := `{"id":"tx-2","amount":"10.50","type":"dcb","attempt":3,"version":1,"signature":"` + signature + `"}`

	event, err := VerifyIPN([]byte(body), "secret")
	if err != nil {
		t.Fatal(err)
	}
	if !event.IsDCB() || event.Email() != "" {
		t.Fatalf("event = %+v", event)
	}
}

func TestVerifyIPNCaptureWithNumericAmountAndStringCounters(t *testing.T) {
	signature := ipnSignature("tx-3", "secret", "10.5", "", "capture", "1", "1", "")
	body := `{"id":"tx-3","amount":10.5,"email":"","type":"capture","attempt":"1","version":"1",` +
		`"custom":"","capture_payment_id":"cap-9","signature":"` + signature + `"}`

	event, err := VerifyIPN([]byte(body), "secret")
	if err != nil {
		t.Fatalf("numeric amount and string counters must verify: %v", err)
	}
	if event.Amount() != "10.5" {
		t.Fatalf("Amount = %q, want the raw string form", event.Amount())
	}
	if !event.IsCapture() || event.CapturePaymentID() != "cap-9" {
		t.Fatalf("event = %+v", event)
	}
}

func TestVerifyIPNRejectsBadSignature(t *testing.T) {
	body := `{"id":"tx-1","amount":"29.99","type":"transfer","attempt":1,"version":1,"signature":"deadbeef"}`
	_, err := VerifyIPN([]byte(body), "secret")
	if !errors.Is(err, ErrSignature) || err.Error() != "dpay: Invalid IPN signature" {
		t.Fatalf("err = %v", err)
	}
}

func TestVerifyIPNRejectsMalformedPayloads(t *testing.T) {
	cases := map[string]string{
		"not json":        `nope`,
		"array":           `[1,2]`,
		"missing id":      `{"amount":"1","type":"transfer","attempt":1,"version":1,"signature":"x"}`,
		"missing amount":  `{"id":"t","type":"transfer","attempt":1,"version":1,"signature":"x"}`,
		"missing type":    `{"id":"t","amount":"1","attempt":1,"version":1,"signature":"x"}`,
		"missing attempt": `{"id":"t","amount":"1","type":"transfer","version":1,"signature":"x"}`,
		"missing version": `{"id":"t","amount":"1","type":"transfer","attempt":1,"signature":"x"}`,
		"missing sig":     `{"id":"t","amount":"1","type":"transfer","attempt":1,"version":1}`,
		"non-string sig":  `{"id":"t","amount":"1","type":"transfer","attempt":1,"version":1,"signature":5}`,
	}
	for name, body := range cases {
		_, err := VerifyIPN([]byte(body), "secret")
		if !errors.Is(err, ErrSignature) || err.Error() != "dpay: Invalid IPN payload" {
			t.Errorf("%s: err = %v", name, err)
		}
	}
}

func TestVerifyIPNAckConstant(t *testing.T) {
	if IPNAck != "OK" {
		t.Fatalf("IPNAck = %q, want OK - dpay accepts nothing else", IPNAck)
	}
}
