package dpay

import (
	"context"
	"testing"
)

func TestPayoutDetails(t *testing.T) {
	server, sent, path := recordingServer(t, 200, `{
		"id":42,"state":1,"net":"100.00","fee":"2.50","gross":"102.50",
		"creation_date":"2026-07-01 09:00:00","direct_settlement":1,"nrb":"PL61",
		"declined":0,"decline_reason":null,"decline_status":null,
		"receiver":{"nrb":"PL62","title":"Wypłata","amount":"100.00","service":"svc",
			"receiverName":"Jan Kowalski","receiverAddress":"Warszawa"}
	}`)
	client, _ := New("test_service", "secret_hash", WithBaseURLs(BaseURLs{Panel: server.URL}))

	payout, err := client.Payouts.Details(context.Background(), 42, WithTimestamp(1784700000))
	if err != nil {
		t.Fatal(err)
	}
	if *path != "/api/v1/pbl/withdraws/details" {
		t.Fatalf("path = %q", *path)
	}
	if payout.ID() != 42 || !payout.IsProcessed() || payout.IsWaiting() || payout.IsFailed() {
		t.Fatalf("state handling: %+v", payout)
	}
	if payout.Net().Minor() != 10000 || payout.Fee().Minor() != 250 || payout.Gross().Minor() != 10250 {
		t.Fatal("amounts broken")
	}
	if !payout.IsDirectSettlement() || payout.NRB() != "PL61" || payout.IsDeclined() {
		t.Fatalf("flags: %+v", payout)
	}
	receiver := payout.Receiver()
	if receiver == nil || receiver.ReceiverName() != "Jan Kowalski" {
		t.Fatal("receiver lost")
	}
	amount, ok := receiver.Amount()
	if !ok || amount.Minor() != 10000 {
		t.Fatal("receiver amount broken")
	}

	decoded := decodeBody(t, *sent)
	if decoded["withdraw_id"] != float64(42) {
		t.Fatalf("body = %v", decoded)
	}
	want := expectedOrderedChecksum(t, "secret_hash", "test_service", "1784700000", "42")
	if decoded["checksum"] != want {
		t.Fatalf("checksum = %v, want %v - order is service, timestamp, withdraw_id", decoded["checksum"], want)
	}
}

func TestPayoutDetailsWithoutTimestampOmitsField(t *testing.T) {
	server, sent, _ := recordingServer(t, 200, `{"id":1,"state":0}`)
	client, _ := New("test_service", "secret_hash", WithBaseURLs(BaseURLs{Panel: server.URL}))
	if _, err := client.Payouts.Details(context.Background(), 7); err != nil {
		t.Fatal(err)
	}
	decoded := decodeBody(t, *sent)
	if _, present := decoded["timestamp"]; present {
		t.Fatal("without WithTimestamp the field must be absent entirely")
	}
	want := expectedOrderedChecksum(t, "secret_hash", "test_service", "7")
	if decoded["checksum"] != want {
		t.Fatalf("checksum = %v, want %v", decoded["checksum"], want)
	}
}

func TestPayoutFailedState(t *testing.T) {
	server, _, _ := recordingServer(t, 200, `{"id":1,"state":-1,"declined":1,"decline_reason":"no funds"}`)
	client, _ := New("svc", "hash", WithBaseURLs(BaseURLs{Panel: server.URL}))
	payout, _ := client.Payouts.Details(context.Background(), 1)
	if !payout.IsFailed() || !payout.IsDeclined() || payout.DeclineReason() != "no funds" {
		t.Fatalf("payout = %+v", payout)
	}
}

func TestPayoutWithoutReceiver(t *testing.T) {
	server, _, _ := recordingServer(t, 200, `{"id":1,"state":0}`)
	client, _ := New("svc", "hash", WithBaseURLs(BaseURLs{Panel: server.URL}))
	payout, _ := client.Payouts.Details(context.Background(), 1)
	if payout.Receiver() != nil || !payout.IsWaiting() {
		t.Fatalf("payout = %+v", payout)
	}
}
