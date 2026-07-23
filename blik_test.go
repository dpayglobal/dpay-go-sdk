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

	if err := client.Blik.UnregisterAlias(context.Background(), "alias-1", BlikAliasTypePayID,
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

func TestBlikRecurringStatusCall(t *testing.T) {
	server, sent, path := recordingServer(t, 200, `{"data":{
		"alias_value":"payid-1","alias_type":"PAYID","status":"ACTIVE","expiration_date":"2027-01-01",
		"registration":{"model":"A","frequency":"1M","limit_amt":5000,"tot_limit_amt":60000,
			"is_limit_amt_fixed":true,"init_date":"2026-08-01","label":"Subskrypcja",
			"registered_at":"2026-07-01 12:00:00"}}}`)
	client, _ := New("test_service", "secret_hash", WithBaseURLs(BaseURLs{APIPayments: server.URL}))

	status, err := client.Blik.RecurringStatus(context.Background(), "payid-1")
	if err != nil {
		t.Fatal(err)
	}
	if *path != "/api/v1_0/payments/blik/recurring/status" {
		t.Fatalf("path = %q", *path)
	}
	if status.Value() != "payid-1" || status.Type() != BlikAliasTypePayID || !status.IsActive() {
		t.Fatalf("status = %+v", status)
	}
	registration := status.Registration()
	if registration == nil || registration.Model() != "A" || registration.Frequency() != "1M" {
		t.Fatal("registration lost")
	}
	if registration.LimitAmt() == nil || *registration.LimitAmt() != 5000 {
		t.Fatal("limit_amt lost")
	}
	if registration.TotLimitAmt() == nil || *registration.TotLimitAmt() != 60000 {
		t.Fatal("tot_limit_amt lost")
	}
	if registration.IsLimitAmtFixed() == nil || !*registration.IsLimitAmtFixed() {
		t.Fatal("is_limit_amt_fixed lost")
	}
	if registration.Label() != "Subskrypcja" || registration.RegisteredAt() != "2026-07-01 12:00:00" {
		t.Fatal("registration metadata lost")
	}
	if registration.InitDate() != "2026-08-01" {
		t.Fatal("init_date lost")
	}

	decoded := decodeBody(t, *sent)
	if _, present := decoded["alias_type"]; present {
		t.Fatal("recurring/status must not send alias_type")
	}
	if decoded["checksum"] != expectedRegisterChecksum(t, "test_service", "secret_hash", "payid-1") {
		t.Fatalf("checksum = %v", decoded["checksum"])
	}
}

func TestBlikRecurringStatusWithoutRegistration(t *testing.T) {
	server, _, _ := recordingServer(t, 200, `{"data":{"alias_value":"p","status":"INACTIVE"}}`)
	client, _ := New("svc", "hash", WithBaseURLs(BaseURLs{APIPayments: server.URL}))
	status, _ := client.Blik.RecurringStatus(context.Background(), "p")
	if status.Registration() != nil || status.IsActive() {
		t.Fatalf("status = %+v", status)
	}
	if status.Type() != BlikAliasTypePayID {
		t.Fatal("missing alias_type must default to PAYID")
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
