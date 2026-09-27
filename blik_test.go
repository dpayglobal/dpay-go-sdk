package dpay

import (
	"context"
	"testing"
)

func TestBlikAliasCall(t *testing.T) {
	server, sent, path := recordingServer(t, 200, `{"data":{
		"alias_value":"alias-1","alias_type":"UID","status":"ACTIVE","expiration_date":"2027-01-01",
		"apps":[{"key":"mbank","label":"mBank"},{"key":"ing","label":"ING"}]}}`)
	client, _ := New("test_service", "secret_hash", WithBaseURLs(BaseURLs{APIPayments: server.URL}))

	alias, err := client.Blik.Alias(context.Background(), "alias-1", BlikAliasTypeUID)
	if err != nil {
		t.Fatal(err)
	}
	if *path != "/api/v1_0/payments/blik/aliases" {
		t.Fatalf("path = %q", *path)
	}
	if alias.Value() != "alias-1" || alias.Type() != BlikAliasTypeUID || !alias.IsActive() {
		t.Fatalf("alias = %+v", alias)
	}
	if len(alias.Apps()) != 2 || alias.Apps()[1].Label() != "ING" {
		t.Fatal("apps lost")
	}

	decoded := decodeBody(t, *sent)
	if decoded["service"] != "test_service" || decoded["alias_value"] != "alias-1" || decoded["alias_type"] != "UID" {
		t.Fatalf("body = %v", decoded)
	}
	if decoded["checksum"] != expectedRegisterChecksum(t, "test_service", "secret_hash", "alias-1") {
		t.Fatalf("checksum = %v", decoded["checksum"])
	}
}

func TestBlikAliasRejectsUnknownType(t *testing.T) {
	client, _ := New("svc", "hash", WithBaseURLs(BaseURLs{APIPayments: "http://127.0.0.1:1"}))
	if _, err := client.Blik.Alias(context.Background(), "a", "NOPE"); err == nil ||
		err.Error() != `dpay: Invalid BLIK alias type "NOPE"` {
		t.Fatalf("err = %v", err)
	}
}

func TestBlikUnregisterAliasReasonStaysOutOfChecksum(t *testing.T) {
	server, sent, path := recordingServer(t, 200, `{"data":{}}`)
	client, _ := New("test_service", "secret_hash", WithBaseURLs(BaseURLs{APIPayments: server.URL}))

	if err := client.Blik.UnregisterAlias(context.Background(), "alias-1", BlikAliasTypeUID,
		WithUnregisterReason("na życzenie klienta")); err != nil {
		t.Fatal(err)
	}
	if *path != "/api/v1_0/payments/blik/aliases/unregister" {
		t.Fatalf("path = %q", *path)
	}
	decoded := decodeBody(t, *sent)
	if decoded["reason"] != "na życzenie klienta" {
		t.Fatalf("reason lost: %v", decoded)
	}
	want := expectedRegisterChecksum(t, "test_service", "secret_hash", "alias-1")
	if decoded["checksum"] != want {
		t.Fatalf("checksum = %v, want %v - reason must not enter the checksum", decoded["checksum"], want)
	}
}

func TestBlikUnregisterAliasWithoutReason(t *testing.T) {
	server, sent, _ := recordingServer(t, 200, `{"data":{}}`)
	client, _ := New("svc", "hash", WithBaseURLs(BaseURLs{APIPayments: server.URL}))
	if err := client.Blik.UnregisterAlias(context.Background(), "a", BlikAliasTypeUID); err != nil {
		t.Fatal(err)
	}
	decoded := decodeBody(t, *sent)
	if _, present := decoded["reason"]; present {
		t.Fatal("absent reason must not be sent")
	}
}

func TestBlikAliasesAcceptOnlyUID(t *testing.T) {
	client, _ := New("svc", "hash", WithBaseURLs(BaseURLs{APIPayments: "http://127.0.0.1:1"}))
	// PAYID aliases belong to recurring payments (client.Recurring), the API answers 422 here
	if _, err := client.Blik.Alias(context.Background(), "a", "PAYID"); err == nil ||
		err.Error() != `dpay: Invalid BLIK alias type "PAYID"` {
		t.Fatalf("Alias err = %v", err)
	}
	if err := client.Blik.UnregisterAlias(context.Background(), "a", "PAYID"); err == nil ||
		err.Error() != `dpay: Invalid BLIK alias type "PAYID"` {
		t.Fatalf("UnregisterAlias err = %v", err)
	}
}

func TestBlikAliasDefaultsTypeToUID(t *testing.T) {
	server, _, _ := recordingServer(t, 200, `{"data":{"alias_value":"a"}}`)
	client, _ := New("svc", "hash", WithBaseURLs(BaseURLs{APIPayments: server.URL}))
	alias, _ := client.Blik.Alias(context.Background(), "a", BlikAliasTypeUID)
	if alias.Type() != BlikAliasTypeUID {
		t.Fatal("missing alias_type must default to UID")
	}
}
