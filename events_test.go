package dpay

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func eventJSON(id, eventType string) string {
	return `{"id":"` + id + `","type":"` + eventType + `","api_version":"2026-10-01","created":"2026-09-27T10:05:00Z",` +
		`"livemode":true,"service":"sdk-test-service","data":{"object":{"object":"payment","id":"TX-1"}}}`
}

func TestEventsListSignsTheTimestampAndSendsFilters(t *testing.T) {
	client, sent, path := newVectorClient(t, 200, `{"status":"success","data":[`+
		eventJSON("evt_01k6a8q2m4pz7h8c3v5n9t2x6y", "payment.succeeded")+`],"has_more":false,"next_starting_after":null}`)

	page, err := client.Events.List(context.Background(), &EventListParams{
		Types: []WebhookEventType{WebhookEventTypePaymentSucceeded, WebhookEventTypeRefundFailed},
		Limit: Int(50),
	}, WithTimestamp(1790503500))
	if err != nil {
		t.Fatal(err)
	}
	if *path != "/api/v1_0/events" {
		t.Fatalf("path = %q", *path)
	}
	// sha256(service|hash|timestamp) - the filters stay out of the checksum
	want := `{"service":"sdk-test-service","timestamp":1790503500,"types":["payment.succeeded","refund.failed"],"limit":50,` +
		`"checksum":"390cb30baacbc92bf2244d049f4b500937c6209446dd2fd79829d7975321abf0"}`
	if *sent != want {
		t.Fatalf("body = %s\nwant   %s", *sent, want)
	}
	if len(page.Data()) != 1 || page.Data()[0].Type() != WebhookEventTypePaymentSucceeded || page.HasMore() || page.NextStartingAfter() != "" {
		t.Fatalf("page = %v", page.Raw())
	}
}

func TestEventsListKeepsTheFieldOrderOfThePHPSDK(t *testing.T) {
	client, sent, _ := newVectorClient(t, 200, `{"status":"success","data":[],"has_more":false}`)
	if _, err := client.Events.List(context.Background(), &EventListParams{
		Limit: Int(1), StartingAfter: String("evt_01k6a8q2m4pz7h8c3v5n9t2x6y"),
		CreatedTo: String("2026-09-30T00:00:00Z"), CreatedFrom: String("2026-09-01T00:00:00Z"),
		Types: []WebhookEventType{WebhookEventTypePayoutPaid},
	}, WithTimestamp(1790503500)); err != nil {
		t.Fatal(err)
	}
	want := []string{"service", "timestamp", "types", "created_from", "created_to", "starting_after", "limit", "checksum"}
	if got := keysInOrder(t, *sent); strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("keys = %v, want %v", got, want)
	}
}

func TestEventsListWithoutParamsUsesTheClock(t *testing.T) {
	client, sent, _ := newVectorClient(t, 200, `{"status":"success","data":[],"has_more":false}`)
	if _, err := client.Events.List(context.Background(), nil); err != nil {
		t.Fatal(err)
	}
	body := decodeBody(t, *sent)
	timestamp, ok := body["timestamp"].(float64)
	if !ok || timestamp < 1790000000 || len(body) != 3 {
		t.Fatalf("body = %v", body)
	}
}

func TestEventsListRejectsInvalidParamsBeforeCallingTheAPI(t *testing.T) {
	client, _ := New("svc", "hash", WithBaseURLs(BaseURLs{APIPayments: "http://127.0.0.1:1"}))
	cases := map[string]EventListParams{
		`Event "merchant.updated" is not allowed in the webhook object of the Events API`: {Types: []WebhookEventType{"merchant.updated"}},
		`Event "webhook.test" is not allowed in the webhook object of the Events API`:     {Types: []WebhookEventType{WebhookEventTypeWebhookTest}},
		"Event types must be a non-empty list of distinct types":                          {Types: []WebhookEventType{}},
		"starting_after must be an event id (evt_...)":                                    {StartingAfter: String("evt_1")},
		"limit must be between 1 and 100":                                                 {Limit: Int(101)},
	}
	for want, params := range cases {
		if _, err := client.Events.List(context.Background(), &params); !errors.Is(err, ErrInvalidArgument) || err.Error() != "dpay: "+want {
			t.Errorf("err = %v, want %q", err, want)
		}
	}
	duplicated := EventListParams{Types: []WebhookEventType{WebhookEventTypePaymentFailed, WebhookEventTypePaymentFailed}}
	if _, err := client.Events.List(context.Background(), &duplicated); err == nil ||
		err.Error() != "dpay: Event types must be a non-empty list of distinct types" {
		t.Fatalf("err = %v", err)
	}
}

func TestEventsIteratePagesUntilTheEnd(t *testing.T) {
	var bodies []string
	pages := []string{
		`{"status":"success","data":[` + eventJSON("evt_01k6a8q2m4pz7h8c3v5n9t2x6y", "payment.succeeded") +
			`],"has_more":true,"next_starting_after":"evt_01k6a8q2m4pz7h8c3v5n9t2x6y"}`,
		`{"status":"success","data":[` + eventJSON("evt_01k6a8q2m4pz7h8c3v5n9t2x6a", "refund.failed") +
			`],"has_more":false,"next_starting_after":"evt_01k6a8q2m4pz7h8c3v5n9t2x6a"}`,
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		buffer, _ := io.ReadAll(r.Body)
		bodies = append(bodies, string(buffer))
		w.Write([]byte(pages[len(bodies)-1]))
	}))
	defer server.Close()
	client, _ := New("sdk-test-service", "sdk-test-hash-0001", WithBaseURLs(BaseURLs{APIPayments: server.URL}))

	params := &EventListParams{Limit: Int(1)}
	events := client.Events.Iterate(context.Background(), params)
	var ids []string
	for events.Next() {
		ids = append(ids, events.Event().ID())
	}
	if err := events.Err(); err != nil {
		t.Fatal(err)
	}
	if strings.Join(ids, ",") != "evt_01k6a8q2m4pz7h8c3v5n9t2x6y,evt_01k6a8q2m4pz7h8c3v5n9t2x6a" || len(bodies) != 2 {
		t.Fatalf("ids = %v, requests = %d", ids, len(bodies))
	}
	if _, present := decodeBody(t, bodies[0])["starting_after"]; present {
		t.Fatal("the first page starts from the newest event")
	}
	if got := decodeBody(t, bodies[1])["starting_after"]; got != "evt_01k6a8q2m4pz7h8c3v5n9t2x6y" {
		t.Fatalf("starting_after = %v", got)
	}
	if params.StartingAfter != nil {
		t.Fatal("Iterate must not modify the caller's params")
	}
	if events.Next() || events.Event() != nil {
		t.Fatal("an exhausted iterator stays exhausted")
	}
}

func TestEventsIterateStopsOnAnError(t *testing.T) {
	client, _, _ := newVectorClient(t, 429, `{"message":"Too Many Attempts."}`)
	events := client.Events.Iterate(context.Background(), nil)
	if events.Next() {
		t.Fatal("no event after an error")
	}
	if !errors.Is(events.Err(), ErrRateLimit) {
		t.Fatalf("err = %v", events.Err())
	}
}

func TestEventPageFromAPI(t *testing.T) {
	page := eventPageFromAPI(map[string]any{
		"data":                []any{map[string]any{"id": "evt_1"}, "skipped", []any{}},
		"has_more":            "yes",
		"next_starting_after": 5,
	})
	if len(page.Data()) != 2 || page.Data()[0].ID() != "evt_1" || page.HasMore() || page.NextStartingAfter() != "" {
		t.Fatalf("page = %v", page.Raw())
	}
}
