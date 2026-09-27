package dpay

import (
	"context"
	"strconv"

	"github.com/dpayglobal/dpay-go-sdk/internal/wire"
)

// EventService reads the event history of the service - the same envelopes as
// webhooks, newest first. Use it to catch up after an outage of your webhook
// endpoint. Entries are re-encoded by the API, so do not verify webhook
// signatures on them.
type EventService struct {
	client *Client
}

// EventListParams filters the event history. Every nil field is omitted.
type EventListParams struct {
	// Types keeps events of these merchant event types; a non-nil list must
	// not be empty or repeat a type.
	Types []WebhookEventType
	// CreatedFrom keeps events created at or after this date (ISO 8601).
	CreatedFrom *string
	// CreatedTo keeps events created at or before this date (ISO 8601).
	CreatedTo *string
	// StartingAfter is the event id (evt_...) the page starts after, usually
	// EventPage.NextStartingAfter of the previous page.
	StartingAfter *string
	// Limit is the page size, 1-100 (the API defaults to 20).
	Limit *int
}

// validate reports the first problem, using the same messages as the PHP SDK.
func (p EventListParams) validate() error {
	if p.Types != nil {
		if len(p.Types) == 0 || hasDuplicates(p.Types) {
			return newValidationError("Event types must be a non-empty list of distinct types")
		}
		if err := assertEventsAllowed(p.Types, MerchantEventTypes(), "the Events API"); err != nil {
			return err
		}
	}
	if p.StartingAfter != nil && !eventIDPattern.MatchString(*p.StartingAfter) {
		return newValidationError("starting_after must be an event id (evt_...)")
	}
	if p.Limit != nil && (*p.Limit < 1 || *p.Limit > 100) {
		return newValidationError("limit must be between 1 and 100")
	}
	return nil
}

// List returns one page of events. The request is signed with the timestamp,
// which the API accepts within 300 seconds of its clock; WithTimestamp pins it.
// A nil params lists every event.
func (s *EventService) List(ctx context.Context, params *EventListParams, opts ...TimestampOption) (*EventPage, error) {
	filters := EventListParams{}
	if params != nil {
		filters = *params
	}
	if err := filters.validate(); err != nil {
		return nil, err
	}
	timestamp := resolveTimestamp(opts)

	body := wire.NewBody()
	body.Set("service", s.client.service)
	body.Set("timestamp", timestamp)
	if filters.Types != nil {
		types := make([]string, 0, len(filters.Types))
		for _, eventType := range filters.Types {
			types = append(types, string(eventType))
		}
		body.Set("types", types)
	}
	body.SetIfNotNil("created_from", filters.CreatedFrom)
	body.SetIfNotNil("created_to", filters.CreatedTo)
	body.SetIfNotNil("starting_after", filters.StartingAfter)
	body.SetIfNotNil("limit", filters.Limit)
	body.Set("checksum", s.client.checksum.SecretSecond(s.client.service, []any{strconv.FormatInt(timestamp, 10)}))

	data, err := s.client.postJSONObject(ctx, hostAPIPayments, "/api/v1_0/events", body)
	if err != nil {
		return nil, err
	}
	return eventPageFromAPI(data), nil
}

// Iterate walks every matching event page by page, newest first, fetching the
// next page only when the current one is used up:
//
//	events := client.Events.Iterate(ctx, &dpay.EventListParams{Types: types})
//	for events.Next() {
//		handle(events.Event())
//	}
//	if err := events.Err(); err != nil {
//		// ...
//	}
func (s *EventService) Iterate(ctx context.Context, params *EventListParams) *EventIterator {
	iterator := &EventIterator{service: s, ctx: ctx}
	if params != nil {
		iterator.params = *params
	}
	return iterator
}

// EventIterator walks the event history returned by EventService.Iterate. It
// is not safe for concurrent use.
type EventIterator struct {
	service *EventService
	ctx     context.Context
	params  EventListParams
	page    []*WebhookEvent
	index   int
	current *WebhookEvent
	fetched bool
	more    bool
	err     error
}

// Next advances to the next event, fetching the next page when needed. It
// returns false at the end of the history or after an error (see Err).
func (it *EventIterator) Next() bool {
	for {
		if it.index < len(it.page) {
			it.current = it.page[it.index]
			it.index++
			return true
		}
		it.current = nil
		if it.err != nil || (it.fetched && !it.more) {
			return false
		}
		page, err := it.service.List(it.ctx, &it.params)
		it.fetched = true
		if err != nil {
			it.err = err
			return false
		}
		it.page, it.index = page.Data(), 0
		it.params.StartingAfter = page.nextStartingAfter
		it.more = page.HasMore() && page.nextStartingAfter != nil
	}
}

// Event returns the event Next advanced to.
func (it *EventIterator) Event() *WebhookEvent { return it.current }

// Err returns the error that stopped the iteration, nil at a regular end.
func (it *EventIterator) Err() error { return it.err }

// EventPage is one page of the event history.
type EventPage struct {
	raw               map[string]any
	data              []*WebhookEvent
	hasMore           bool
	nextStartingAfter *string
}

func eventPageFromAPI(data map[string]any) *EventPage {
	page := &EventPage{raw: data, data: []*WebhookEvent{}}
	for _, item := range arrayField(data, "data") {
		if event, ok := webhookEventFromJSON(item); ok {
			page.data = append(page.data, event)
		}
	}
	page.hasMore, _ = data["has_more"].(bool)
	if next, ok := data["next_starting_after"].(string); ok {
		page.nextStartingAfter = &next
	}
	return page
}

// Data returns the events, newest first.
func (p *EventPage) Data() []*WebhookEvent { return p.data }

// HasMore reports whether older events follow.
func (p *EventPage) HasMore() bool { return p.hasMore }

// NextStartingAfter returns the StartingAfter of the next page, empty when there is none.
func (p *EventPage) NextStartingAfter() string {
	if p.nextStartingAfter == nil {
		return ""
	}
	return *p.nextStartingAfter
}

// Raw returns the decoded response body.
func (p *EventPage) Raw() map[string]any { return p.raw }
