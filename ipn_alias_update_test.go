package dpay

import (
	"encoding/json"
	"errors"
	"reflect"
	"testing"
)

const aliasUpdateID = "30D9493D-1D73-3FBD-A5D4-633723CC7A68"

// aliasUpdatePayload is a valid alias_update IPN for secret123 with the given
// fields replaced; a nil value sends JSON null.
func aliasUpdatePayload(t *testing.T, overrides map[string]any) []byte {
	t.Helper()
	payload := map[string]any{
		"id":                     aliasUpdateID,
		"type":                   "alias_update",
		"change_type":            "ALIAS_UNREGISTER",
		"alias_type":             "UID",
		"alias_status":           "UNREGISTERED",
		"alias_label":            "Moj sklep",
		"alias_key":              "KEY-1",
		"alias_expire_at":        nil,
		"change_reason_category": "USER",
		"change_reason_message":  "Unregistered in the banking app",
		"version":                1,
		"signature":              ipnSignature(aliasUpdateID, "secret123", "alias_update", "1"),
	}
	for key, value := range overrides {
		payload[key] = value
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	return encoded
}

func transferPayload(t *testing.T, overrides map[string]any) []byte {
	t.Helper()
	payload := map[string]any{
		"id": aliasUpdateID, "amount": "10.00", "email": "client@example.com", "type": "transfer",
		"attempt": 1, "version": 1, "custom": "order-1234",
		"signature": ipnSignature(aliasUpdateID, "secret123", "10.00", "client@example.com", "transfer", "1", "1", "order-1234"),
	}
	for key, value := range overrides {
		payload[key] = value
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	return encoded
}

func TestVerifyIPNAliasUpdate(t *testing.T) {
	event, err := VerifyIPN(aliasUpdatePayload(t, nil), "secret123")
	if err != nil {
		t.Fatal(err)
	}
	if !event.IsAliasUpdate() || event.IsTransfer() || event.Type() != IPNTypeAliasUpdate || !IPNTypeAliasUpdate.Valid() {
		t.Fatal("type helpers broken")
	}
	if event.ID() != aliasUpdateID || event.Amount() != "" || event.Version() != 1 {
		t.Fatalf("event = %+v", event.Raw())
	}
	alias := event.AliasUpdate()
	if alias == nil {
		t.Fatal("AliasUpdate() = nil")
	}
	if alias.ChangeType() != AliasChangeUnregister || alias.AliasType() != AliasTypeUID ||
		alias.AliasStatus() != AliasStatusUnregistered || alias.IsActive() {
		t.Fatalf("alias = %+v", alias.Raw())
	}
	if alias.AliasKey() != "KEY-1" || alias.AliasLabel() != "Moj sklep" || alias.AliasExpireAt() != "" ||
		alias.ChangeReasonCategory() != "USER" || alias.ChangeReasonMessage() != "Unregistered in the banking app" {
		t.Fatalf("alias = %+v", alias.Raw())
	}
}

// The signature covers only id, type and version - a changed status still verifies, a changed id does not.
func TestVerifyIPNAliasUpdateSignatureIgnoresTheAliasFields(t *testing.T) {
	event, err := VerifyIPN(aliasUpdatePayload(t, map[string]any{"alias_status": "ACTIVE"}), "secret123")
	if err != nil || !event.AliasUpdate().IsActive() {
		t.Fatalf("err = %v", err)
	}
	_, err = VerifyIPN(aliasUpdatePayload(t, map[string]any{"id": "OTHER"}), "secret123")
	if !errors.Is(err, ErrSignature) || err.Error() != "dpay: Invalid IPN signature" {
		t.Fatalf("err = %v", err)
	}
}

func TestVerifyIPNAliasUpdateWithoutVersionIsInvalid(t *testing.T) {
	for _, field := range []string{"version", "id", "signature"} {
		_, err := VerifyIPN(aliasUpdatePayload(t, map[string]any{field: nil}), "secret123")
		if !errors.Is(err, ErrSignature) || err.Error() != "dpay: Invalid IPN payload" {
			t.Errorf("%s null: err = %v", field, err)
		}
	}
}

// amount, email, attempt and custom are not part of the alias_update signature - an injected value
// must not reach handlers that read them without checking the type.
func TestVerifyIPNAliasUpdateIgnoresUnsignedTransferFields(t *testing.T) {
	event, err := VerifyIPN(aliasUpdatePayload(t, map[string]any{
		"amount": "999.00", "email": "attacker@example.com", "attempt": 7, "custom": "order-1234",
	}), "secret123")
	if err != nil {
		t.Fatal(err)
	}
	if !event.IsAliasUpdate() || event.Amount() != "" || event.Email() != "" || event.Attempt() != 0 || event.Custom() != "" {
		t.Fatalf("unsigned fields leaked: amount %q email %q attempt %d custom %q",
			event.Amount(), event.Email(), event.Attempt(), event.Custom())
	}
	if event.ID() != aliasUpdateID || event.Version() != 1 {
		t.Fatal("signed fields lost")
	}
}

func TestVerifyIPNTypeCannotDowngradeTheSignature(t *testing.T) {
	_, err := VerifyIPN(transferPayload(t, map[string]any{"type": "alias_update"}), "secret123")
	if err == nil || err.Error() != "dpay: Invalid IPN signature" {
		t.Fatalf("transfer as alias_update: err = %v", err)
	}
	_, err = VerifyIPN(aliasUpdatePayload(t, map[string]any{"type": "transfer", "amount": "", "attempt": 1}), "secret123")
	if err == nil || err.Error() != "dpay: Invalid IPN signature" {
		t.Fatalf("alias_update as transfer: err = %v", err)
	}
}

func TestVerifyIPNTransferHasNoAliasUpdate(t *testing.T) {
	event, err := VerifyIPN(transferPayload(t, nil), "secret123")
	if err != nil {
		t.Fatal(err)
	}
	if event.IsAliasUpdate() || event.AliasUpdate() != nil || event.Amount() != "10.00" || event.Attempt() != 1 {
		t.Fatal("a transfer must keep its fields and have no alias update")
	}
}

func TestAPIVectorsIPN(t *testing.T) {
	vectors := loadAPIVectors(t)
	if len(vectors.IPN) != 3 {
		t.Fatalf("ipn vectors = %d, want 3", len(vectors.IPN))
	}
	for _, vector := range vectors.IPN {
		event, err := VerifyIPN(vector.Payload, vectors.SecretHash)
		if err != nil {
			t.Errorf("%s: %v", vector.Name, err)
			continue
		}
		if event.ID() != vectors.TransactionID {
			t.Errorf("%s: id = %q", vector.Name, event.ID())
		}
		var payload map[string]any
		if err := json.Unmarshal(vector.Payload, &payload); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(event.Raw(), payload) {
			t.Errorf("%s: raw = %v", vector.Name, event.Raw())
		}
		if _, err := VerifyIPN(vector.Payload, vectors.SecretHash+"x"); err == nil || err.Error() != "dpay: Invalid IPN signature" {
			t.Errorf("%s: a wrong secret must not verify: %v", vector.Name, err)
		}
	}
}
