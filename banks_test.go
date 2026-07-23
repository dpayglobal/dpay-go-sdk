package dpay

import (
	"context"
	"testing"
)

func TestBanksAll(t *testing.T) {
	server, sent, path := recordingServer(t, 200,
		`[{"id":"1","name":"Bank A","image":"https://a/logo.png","on_from":0,"on_to":24,"iterator":3,"test":false,"type":"pbl"},
		  {"id":"2","name":"Bank B"}]`)
	client, _ := New("svc", "hash", WithBaseURLs(BaseURLs{Panel: server.URL}))

	banks, err := client.Banks.All(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if *path != "/api/v1/pbl/banks" || *sent != "" {
		t.Fatalf("All must be a GET without a body: path=%q body=%q", *path, *sent)
	}
	if len(banks) != 2 {
		t.Fatalf("got %d banks", len(banks))
	}
	first := banks[0]
	if first.ID() != "1" || first.Name() != "Bank A" || first.Image() != "https://a/logo.png" {
		t.Fatalf("bank = %+v", first)
	}
	if first.OnTo() != 24 || first.Iterator() == nil || *first.Iterator() != 3 || first.IsTest() || first.Type() != "pbl" {
		t.Fatalf("bank fields = %+v", first)
	}
	if banks[1].Iterator() != nil {
		t.Fatal("absent iterator must stay nil")
	}
}

func TestBanksForServiceSignsWithTimestamp(t *testing.T) {
	server, sent, _ := recordingServer(t, 200, `[]`)
	client, _ := New("test_service", "secret_hash", WithBaseURLs(BaseURLs{Panel: server.URL}))

	if _, err := client.Banks.ForService(context.Background(), WithTimestamp(1784700000)); err != nil {
		t.Fatal(err)
	}
	decoded := decodeBody(t, *sent)
	if decoded["service"] != "test_service" || decoded["timestamp"] != float64(1784700000) {
		t.Fatalf("body = %v", decoded)
	}
	want := expectedOrderedChecksum(t, "secret_hash", "test_service", "1784700000")
	if decoded["checksum"] != want {
		t.Fatalf("checksum = %v, want %v", decoded["checksum"], want)
	}
}

func TestBanksForServiceUsesClockByDefault(t *testing.T) {
	server, sent, _ := recordingServer(t, 200, `[]`)
	client, _ := New("svc", "hash", WithBaseURLs(BaseURLs{Panel: server.URL}))
	if _, err := client.Banks.ForService(context.Background()); err != nil {
		t.Fatal(err)
	}
	decoded := decodeBody(t, *sent)
	if decoded["timestamp"] == nil || decoded["timestamp"].(float64) < 1700000000 {
		t.Fatalf("timestamp = %v, want a current unix time", decoded["timestamp"])
	}
}
