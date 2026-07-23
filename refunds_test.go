package dpay

import (
	"context"
	"errors"
	"testing"
)

func TestRefundCreateMinimal(t *testing.T) {
	server, sent, path := recordingServer(t, 200, `{"status":"success","refund":true,"message":"accepted"}`)
	client, _ := New("test_service", "secret_hash", WithBaseURLs(BaseURLs{Panel: server.URL}))

	refund, err := client.Refunds.Create(context.Background(), "tx-1")
	if err != nil {
		t.Fatal(err)
	}
	if *path != "/api/v1/pbl/refund" {
		t.Fatalf("path = %q", *path)
	}
	if !refund.IsAccepted() || refund.Message() != "accepted" {
		t.Fatalf("refund = %+v", refund)
	}

	decoded := decodeBody(t, *sent)
	if len(decoded) != 3 {
		t.Fatalf("minimal body must carry service, transaction_id and checksum: %v", decoded)
	}
	if decoded["checksum"] != expectedOrderedChecksum(t, "secret_hash", "test_service", "tx-1") {
		t.Fatalf("checksum = %v", decoded["checksum"])
	}
}

func TestRefundCreateWithAmountAndReason(t *testing.T) {
	server, sent, _ := recordingServer(t, 200, `{"status":"success","refund":true}`)
	client, _ := New("test_service", "secret_hash", WithBaseURLs(BaseURLs{Panel: server.URL}))

	if _, err := client.Refunds.Create(context.Background(), "tx-1",
		WithRefundAmount(PLN(500)), WithRefundReason("reklamacja")); err != nil {
		t.Fatal(err)
	}

	decoded := decodeBody(t, *sent)
	if decoded["value"] != "5.00" || decoded["reason"] != "reklamacja" {
		t.Fatalf("body = %v", decoded)
	}
	want := expectedOrderedChecksum(t, "secret_hash", "test_service", "tx-1", "5.00", "reklamacja")
	if decoded["checksum"] != want {
		t.Fatalf("checksum = %v, want %v - optional fields must enter the checksum", decoded["checksum"], want)
	}
}

func TestRefundSendsExplicitEmptyReason(t *testing.T) {
	server, sent, _ := recordingServer(t, 200, `{"status":"success","refund":true}`)
	client, _ := New("svc", "hash", WithBaseURLs(BaseURLs{Panel: server.URL}))

	if _, err := client.Refunds.Create(context.Background(), "tx-1", WithRefundReason("")); err != nil {
		t.Fatal(err)
	}
	decoded := decodeBody(t, *sent)
	if _, present := decoded["reason"]; !present {
		t.Fatal("an explicitly set empty reason must be sent, unlike an absent one")
	}
}

func TestRefundNotAccepted(t *testing.T) {
	server, _, _ := recordingServer(t, 200, `{"status":"error","refund":false,"message":"too late"}`)
	client, _ := New("svc", "hash", WithBaseURLs(BaseURLs{Panel: server.URL}))
	refund, err := client.Refunds.Create(context.Background(), "tx-1")
	if err != nil {
		t.Fatal(err)
	}
	if refund.IsAccepted() {
		t.Fatal("status error must not be accepted")
	}
}

func TestCheckAvailabilityBusinessOutcomes(t *testing.T) {
	for _, status := range []int{200, 400, 402, 406, 409, 410, 411} {
		server, _, path := recordingServer(t, status, `{"refund":false,"message":"not available"}`)
		client, _ := New("svc", "hash", WithBaseURLs(BaseURLs{Panel: server.URL}))

		availability, err := client.Refunds.CheckAvailability(context.Background(), "tx-1")
		if err != nil {
			t.Fatalf("status %d must be a business outcome, got %v", status, err)
		}
		if availability.IsAvailable() || availability.HTTPStatus() != status {
			t.Fatalf("status %d: %+v", status, availability)
		}
		if *path != "/api/v1/pbl/check-refund-availability" {
			t.Fatalf("path = %q", *path)
		}
	}
}

func TestCheckAvailabilityAvailable(t *testing.T) {
	server, _, _ := recordingServer(t, 200, `{"refund":true,"message":"ok"}`)
	client, _ := New("svc", "hash", WithBaseURLs(BaseURLs{Panel: server.URL}))
	availability, err := client.Refunds.CheckAvailability(context.Background(), "tx-1")
	if err != nil {
		t.Fatal(err)
	}
	if !availability.IsAvailable() || availability.Message() != "ok" {
		t.Fatalf("availability = %+v", availability)
	}
}

func TestCheckAvailability401Split(t *testing.T) {
	server, _, _ := recordingServer(t, 401, `{"refund":false,"message":"Unauthorized request"}`)
	client, _ := New("svc", "hash", WithBaseURLs(BaseURLs{Panel: server.URL}))
	if _, err := client.Refunds.CheckAvailability(context.Background(), "tx-1"); !errors.Is(err, ErrAuthentication) {
		t.Fatalf("Unauthorized request must be an error, got %v", err)
	}

	server, _, _ = recordingServer(t, 401, `{"refund":false,"message":"refund window closed"}`)
	client, _ = New("svc", "hash", WithBaseURLs(BaseURLs{Panel: server.URL}))
	availability, err := client.Refunds.CheckAvailability(context.Background(), "tx-1")
	if err != nil {
		t.Fatalf("401 with a different message is a business outcome, got %v", err)
	}
	if availability.HTTPStatus() != 401 {
		t.Fatalf("HTTPStatus = %d", availability.HTTPStatus())
	}
}

func TestCheckAvailabilityWithoutRefundKeyIsError(t *testing.T) {
	server, _, _ := recordingServer(t, 200, `{"message":"nothing here"}`)
	client, _ := New("svc", "hash", WithBaseURLs(BaseURLs{Panel: server.URL}))
	if _, err := client.Refunds.CheckAvailability(context.Background(), "tx-1"); err == nil {
		t.Fatal("a response without the refund key must raise")
	}
}

func TestCheckAvailabilityUnhandledStatusIsError(t *testing.T) {
	server, _, _ := recordingServer(t, 500, `{"refund":false}`)
	client, _ := New("svc", "hash", WithBaseURLs(BaseURLs{Panel: server.URL}))
	if _, err := client.Refunds.CheckAvailability(context.Background(), "tx-1"); !errors.Is(err, ErrServer) {
		t.Fatalf("err = %v", err)
	}
}
