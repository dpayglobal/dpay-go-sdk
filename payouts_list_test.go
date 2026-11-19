package dpay

import (
	"context"
	"errors"
	"reflect"
	"testing"
)

const emptyPayoutPage = `{"withdraws":[],"meta":{"page":1,"per_page":50,"total":0,"total_pages":0}}`

func TestPayoutDetailsOfARegularPayoutIsPaysafecardAware(t *testing.T) {
	client, _ := scriptedClient(t, okResponse(`{"id":12346,"state":0,"net":5.0,"fee":0.5,"gross":5.5,
		"creation_date":"2026-07-01 10:00:00","direct_settlement":0,"paysafecard":1,
		"nrb":"PL61109010140000071219812874","declined":0}`))
	details, err := client.Payouts.Details(context.Background(), 12346)
	if err != nil {
		t.Fatal(err)
	}
	if !details.IsPaysafecard() || !details.IsWaiting() || details.Currency() != CurrencyPLN || details.ExportDate() != "" {
		t.Fatalf("details = %+v", details)
	}
	if details.Gross().Minor() != 550 || details.Gross().Currency() != CurrencyPLN {
		t.Fatalf("gross = %v", details.Gross())
	}
}

func TestPayoutListSendsFiltersInChecksumOrder(t *testing.T) {
	client, doer := scriptedClient(t, okResponse(`{"withdraws":[{"id":12345,"type":"standard","state":1,
		"net":"27.15","fee":"3.06","gross":"30.21","currency_code":"EUR","creation_date":"2026-08-01 10:00:00",
		"export_date":"2026-08-01 14:20:00","declined":0}],
		"meta":{"page":2,"per_page":10,"total":11,"total_pages":2}}`))
	processed := PayoutStateProcessed
	page, err := client.Payouts.List(context.Background(), &PayoutListParams{
		Type: PayoutTypeStandard, State: &processed, DateTo: String("2026-08-31 23:59:59"), DateFrom: String("2026-08-01"),
		Page: Int(2), PerPage: Int(10),
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := doer.requests[0].URL.String(); got != "https://panel.dpay.pl/api/v1/pbl/withdraws" {
		t.Fatalf("URL = %s", got)
	}
	keys := keysInOrder(t, doer.bodies[0])
	if want := []string{"service", "date_from", "date_to", "state", "type", "page", "per_page", "checksum"}; !reflect.DeepEqual(keys, want) {
		t.Fatalf("keys = %v, want %v", keys, want)
	}
	body := doer.lastBody(t)
	if body["state"] != float64(1) {
		t.Fatalf("state = %#v, want the JSON integer 1", body["state"])
	}
	if body["checksum"] != sha256Hex(t, "MyShop|2026-08-01|2026-08-31 23:59:59|1|standard|2|10|secret123") ||
		body["checksum"] != "5be864bc1b81a002fe134dbaed5db1d6517db76fdc952842b756da253ceb504b" {
		t.Fatalf("checksum = %v", body["checksum"])
	}

	items := page.Items()
	if len(items) != 1 {
		t.Fatalf("items = %d", len(items))
	}
	item := items[0]
	eur := func(minor int64) Money { money, _ := NewMoney(minor, CurrencyEUR); return money }
	if item.ID() != 12345 || item.Type() != PayoutTypeStandard || !item.IsProcessed() || item.IsDeclined() {
		t.Fatalf("item = %+v", item)
	}
	if !item.Net().Equal(eur(2715)) || !item.Fee().Equal(eur(306)) || !item.Gross().Equal(eur(3021)) || item.Currency() != CurrencyEUR {
		t.Fatalf("amounts = %v %v %v", item.Net(), item.Fee(), item.Gross())
	}
	if item.ExportDate() != "2026-08-01 14:20:00" || item.CreationDate() != "2026-08-01 10:00:00" {
		t.Fatalf("dates = %q %q", item.CreationDate(), item.ExportDate())
	}
	if page.Meta().Page() != 2 || page.Meta().PerPage() != 10 || page.Meta().Total() != 11 || page.Meta().TotalPages() != 2 || page.HasMore() {
		t.Fatalf("meta = %+v", page.Meta())
	}
}

func TestPayoutListMinimalSignsServiceOnly(t *testing.T) {
	client, doer := scriptedClient(t, okResponse(emptyPayoutPage))
	page, err := client.Payouts.List(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if keys := keysInOrder(t, doer.bodies[0]); !reflect.DeepEqual(keys, []string{"service", "checksum"}) {
		t.Fatalf("keys = %v", keys)
	}
	if doer.lastBody(t)["checksum"] != sha256Hex(t, "MyShop|secret123") {
		t.Fatal("checksum broken")
	}
	if len(page.Items()) != 0 || page.HasMore() {
		t.Fatalf("page = %+v", page)
	}
}

func TestPayoutListStateFiltersSignTheirCode(t *testing.T) {
	for _, testCase := range []struct {
		state PayoutState
		want  string
	}{{PayoutStateFailed, "MyShop|-1|secret123"}, {PayoutStateWaiting, "MyShop|0|secret123"}} {
		client, doer := scriptedClient(t, okResponse(emptyPayoutPage))
		state := testCase.state
		if _, err := client.Payouts.List(context.Background(), &PayoutListParams{State: &state}); err != nil {
			t.Fatal(err)
		}
		if doer.lastBody(t)["checksum"] != sha256Hex(t, testCase.want) {
			t.Errorf("state %d: checksum is not sha256(%s)", state, testCase.want)
		}
	}
}

func TestPayoutListDateFiltersAreSentTrimmedBecauseTheAPITrimsBeforeTheChecksum(t *testing.T) {
	client, doer := scriptedClient(t, okResponse(emptyPayoutPage))
	if _, err := client.Payouts.List(context.Background(), &PayoutListParams{
		DateFrom: String(" 2026-08-01\n"), DateTo: String("2026-08-31 23:59:59 "),
	}); err != nil {
		t.Fatal(err)
	}
	body := doer.lastBody(t)
	if body["date_from"] != "2026-08-01" || body["date_to"] != "2026-08-31 23:59:59" {
		t.Fatalf("dates = %q %q", body["date_from"], body["date_to"])
	}
	if body["checksum"] != sha256Hex(t, "MyShop|2026-08-01|2026-08-31 23:59:59|secret123") {
		t.Fatal("checksum must cover the trimmed dates")
	}
}

func TestPayoutIterateWalksAllPages(t *testing.T) {
	row := func(id string) string {
		return `{"id":` + id + `,"type":"direct","state":0,"net":"1.00","fee":"0.00","gross":"1.00","currency_code":"PLN","declined":0}`
	}
	client, doer := scriptedClient(t,
		okResponse(`{"withdraws":[`+row("3")+`,`+row("2")+`],"meta":{"page":1,"per_page":2,"total":3,"total_pages":2}}`),
		okResponse(`{"withdraws":[`+row("1")+`],"meta":{"page":2,"per_page":2,"total":3,"total_pages":2}}`),
	)
	payouts := client.Payouts.Iterate(context.Background(), &PayoutListParams{Type: PayoutTypeDirect, PerPage: Int(2), Page: Int(7)})
	var ids []int64
	for payouts.Next() {
		ids = append(ids, payouts.Payout().ID())
	}
	if payouts.Err() != nil {
		t.Fatal(payouts.Err())
	}
	if !reflect.DeepEqual(ids, []int64{3, 2, 1}) || len(doer.requests) != 2 {
		t.Fatalf("ids = %v after %d requests", ids, len(doer.requests))
	}
	first := decodeBody(t, doer.bodies[0])
	if first["page"] != float64(1) || doer.lastBody(t)["page"] != float64(2) {
		t.Fatalf("pages = %v, %v - Iterate starts at 1 and always sends the page", first["page"], doer.lastBody(t)["page"])
	}
}

func TestPayoutIterateStopsOnAnEmptyPageAndOnErrors(t *testing.T) {
	client, doer := scriptedClient(t, okResponse(`{"withdraws":[],"meta":{"page":1,"total_pages":5}}`))
	payouts := client.Payouts.Iterate(context.Background(), nil)
	if payouts.Next() || payouts.Err() != nil || len(doer.requests) != 1 {
		t.Fatal("an empty page must end the iteration")
	}

	client, _ = scriptedClient(t, scriptedResponse{status: 500, body: `{"message":"boom"}`})
	payouts = client.Payouts.Iterate(context.Background(), nil)
	if payouts.Next() || !errors.Is(payouts.Err(), ErrServer) {
		t.Fatalf("err = %v", payouts.Err())
	}
}

func TestPayoutListValidation(t *testing.T) {
	invalidState, failed := PayoutState(2), PayoutStateFailed
	cases := []struct {
		name   string
		params PayoutListParams
		want   string
	}{
		{"state", PayoutListParams{State: &invalidState}, `Invalid payout state "2"`},
		{"type", PayoutListParams{Type: "instant"}, `Invalid payout type "instant"`},
		{"empty date_from", PayoutListParams{DateFrom: String(" ")}, "date_from must not be empty"},
		{"empty date_to", PayoutListParams{State: &failed, DateTo: String("\t\n")}, "date_to must not be empty"},
		{"page 0", PayoutListParams{Page: Int(0)}, "page must be at least 1"},
		{"per_page 201", PayoutListParams{PerPage: Int(201)}, "per_page must be between 1 and 200"},
		{"per_page 0", PayoutListParams{PerPage: Int(0)}, "per_page must be between 1 and 200"},
	}
	for _, testCase := range cases {
		client, doer := scriptedClient(t)
		_, err := client.Payouts.List(context.Background(), &testCase.params)
		if !errors.Is(err, ErrInvalidArgument) || err.Error() != "dpay: "+testCase.want {
			t.Errorf("%s: err = %v, want %q", testCase.name, err, testCase.want)
		}
		if len(doer.requests) != 0 {
			t.Errorf("%s: a request was sent", testCase.name)
		}
	}
}

func TestPayoutDetailsFull(t *testing.T) {
	client, doer := scriptedClient(t, okResponse(`{
		"settlement":{"id":12345,"state":1,"net":"27.15","fee":"3.06","gross":"30.21","currency_code":"PLN",
			"creation_date":"2026-08-01 10:00:00","export_date":"2026-08-01 14:20:00","direct_settlement":0,
			"declined":0,"paysafecard":0,"nrb":"PL61109010140000071219812874"},
		"summary":{"net":"27.15","fee":"3.06","gross":"30.21","count":3},
		"transactions":[{"payment_id":"8F2C1A90-5B7E-4C31-9A02-7D6E4B1F3C88","date":"2026-07-31 12:04:11",
			"net":"9.05","fee":"1.02","gross":"10.07","service_id":42,"service_name":"Sklep testowy",
			"custom":"order-1234","ipksef":null}],
		"meta":{"page":1,"per_page":1,"total":3,"total_pages":3}}`))
	settlement, err := client.Payouts.DetailsFull(context.Background(), 12345, &PayoutPageParams{Page: Int(1), PerPage: Int(1)})
	if err != nil {
		t.Fatal(err)
	}
	if got := doer.requests[0].URL.String(); got != "https://panel.dpay.pl/api/v1/pbl/withdraws/details/full" {
		t.Fatalf("URL = %s", got)
	}
	if keys := keysInOrder(t, doer.bodies[0]); !reflect.DeepEqual(keys, []string{"service", "withdraw_id", "page", "per_page", "checksum"}) {
		t.Fatalf("keys = %v", keys)
	}
	if doer.lastBody(t)["checksum"] != sha256Hex(t, "MyShop|12345|1|1|secret123") {
		t.Fatal("checksum broken")
	}

	header := settlement.Settlement()
	if header.ID() != 12345 || !header.IsProcessed() || header.IsPaysafecard() ||
		header.ExportDate() != "2026-08-01 14:20:00" || header.NRB() != "PL61109010140000071219812874" {
		t.Fatalf("settlement = %+v", header)
	}
	if settlement.SummaryNet().Minor() != 2715 || settlement.SummaryFee().Minor() != 306 ||
		settlement.SummaryGross().Minor() != 3021 || settlement.SummaryCount() != 3 {
		t.Fatal("summary broken")
	}
	positions := settlement.Positions()
	if len(positions) != 1 {
		t.Fatalf("positions = %d", len(positions))
	}
	position := positions[0]
	if position.PaymentID() != "8F2C1A90-5B7E-4C31-9A02-7D6E4B1F3C88" || position.Date() != "2026-07-31 12:04:11" ||
		position.Net().Minor() != 905 || position.Fee().Minor() != 102 || position.Gross().Minor() != 1007 ||
		position.ServiceID() == nil || *position.ServiceID() != 42 || position.ServiceName() != "Sklep testowy" ||
		position.Custom() != "order-1234" || position.IPKSeF() != "" {
		t.Fatalf("position = %+v", position.Raw())
	}
	if !settlement.Meta().HasMore() {
		t.Fatal("page 1 of 3 has more")
	}
}

func TestPayoutDetailsFullInForeignCurrencyAndMinimalBody(t *testing.T) {
	client, doer := scriptedClient(t, okResponse(`{
		"settlement":{"id":7,"state":0,"net":"10.00","fee":"0.00","gross":"10.00","currency_code":"EUR",
			"direct_settlement":1,"declined":0,
			"receiver":{"nrb":"PL61109010140000071219812874","title":"FV 1","amount":"10.00","receiverName":"Firma","receiverAddress":"Lisboa"}},
		"summary":{"net":"10.00","fee":"0.00","gross":"10.00","count":1},
		"transactions":[{"payment_id":"P-1","date":null,"net":"10.00","fee":"0.00","gross":"10.00","service_id":"42"}],
		"meta":{"page":1,"per_page":100,"total":1,"total_pages":1}}`))
	settlement, err := client.Payouts.DetailsFull(context.Background(), 7, nil)
	if err != nil {
		t.Fatal(err)
	}
	if keys := keysInOrder(t, doer.bodies[0]); !reflect.DeepEqual(keys, []string{"service", "withdraw_id", "checksum"}) {
		t.Fatalf("keys = %v", keys)
	}
	if doer.lastBody(t)["checksum"] != sha256Hex(t, "MyShop|7|secret123") {
		t.Fatal("checksum broken")
	}
	if settlement.Settlement().Currency() != CurrencyEUR || settlement.SummaryNet().Currency() != CurrencyEUR ||
		settlement.Positions()[0].Net().Currency() != CurrencyEUR || settlement.Settlement().Net().Currency() != CurrencyEUR {
		t.Fatal("the settlement currency must apply to every amount")
	}
	if settlement.Positions()[0].ServiceID() != nil || settlement.Positions()[0].Date() != "" {
		t.Fatal("service_id must be a JSON integer and a null date empty")
	}
	receiver := settlement.Settlement().Receiver()
	if receiver == nil || receiver.ReceiverName() != "Firma" {
		t.Fatal("receiver lost")
	}
	if amount, present := receiver.Amount(); !present || amount.Currency() != CurrencyEUR || amount.Minor() != 1000 {
		t.Fatalf("receiver amount = %v", amount)
	}
	if settlement.Meta().HasMore() {
		t.Fatal("a single page has no more")
	}
}

func TestPayoutDetailsFullValidatesPerPageAndMapsNotFound(t *testing.T) {
	client, doer := scriptedClient(t, scriptedResponse{status: 404, body: `{"status":"error","message":"Withdraw does not exist!"}`})
	_, err := client.Payouts.DetailsFull(context.Background(), 1, &PayoutPageParams{PerPage: Int(501)})
	if !errors.Is(err, ErrInvalidArgument) || err.Error() != "dpay: per_page must be between 1 and 500" || len(doer.requests) != 0 {
		t.Fatalf("err = %v", err)
	}
	if _, err := client.Payouts.DetailsFull(context.Background(), 1, &PayoutPageParams{Page: Int(0)}); err == nil ||
		err.Error() != "dpay: page must be at least 1" {
		t.Fatalf("err = %v", err)
	}
	_, err = client.Payouts.DetailsFull(context.Background(), 999, nil)
	var apiErr *APIError
	if !errors.Is(err, ErrNotFound) || !errors.As(err, &apiErr) || apiErr.HTTPStatus != 404 {
		t.Fatalf("err = %v", err)
	}
}

func TestPayoutListItemFallbacks(t *testing.T) {
	item := payoutListItemFromAPI(map[string]any{"id": "17", "state": "-1", "net": "x", "currency_code": "eur", "declined": "1"})
	if item.ID() != 17 || !item.IsFailed() || !item.IsDeclined() || item.Currency() != CurrencyPLN || item.Type() != "" {
		t.Fatalf("item = %+v", item.Raw())
	}
	if !item.Net().Equal(PLN(0)) || !item.Gross().Equal(PLN(0)) {
		t.Fatal("missing or unparsable amounts are 0 in the payout currency")
	}
	meta := pageMetaFromAPI(map[string]any{})
	if meta.Page() != 1 || meta.TotalPages() != 0 || meta.HasMore() {
		t.Fatalf("meta defaults = %+v", meta)
	}
}

// The SDK itself produces the payout vectors of testdata/api_vectors.json.
func TestPayoutBodiesMatchTheSharedVectors(t *testing.T) {
	vectors := loadAPIVectors(t)
	byName := map[string]string{}
	bodies := map[string]string{}
	for _, vector := range vectors.OrderedBody {
		byName[vector.Name] = vector.Checksum
		bodies[vector.Name] = string(vector.Body)
	}
	failed, waiting, processed := PayoutStateFailed, PayoutStateWaiting, PayoutStateProcessed
	calls := []struct {
		name string
		call func(*Client) error
	}{
		{"payouts_list_minimal", func(c *Client) error { _, err := c.Payouts.List(context.Background(), nil); return err }},
		{"payouts_list_all_filters", func(c *Client) error {
			_, err := c.Payouts.List(context.Background(), &PayoutListParams{
				DateFrom: String("2026-08-01"), DateTo: String("2026-08-31 23:59:59"), State: &processed,
				Type: PayoutTypeStandard, Page: Int(2), PerPage: Int(10),
			})
			return err
		}},
		{"payouts_list_state_failed", func(c *Client) error {
			_, err := c.Payouts.List(context.Background(), &PayoutListParams{State: &failed})
			return err
		}},
		{"payouts_list_state_waiting_page_1", func(c *Client) error {
			_, err := c.Payouts.List(context.Background(), &PayoutListParams{State: &waiting, Page: Int(1)})
			return err
		}},
		{"payouts_details_full_minimal", func(c *Client) error {
			_, err := c.Payouts.DetailsFull(context.Background(), 12345, nil)
			return err
		}},
		{"payouts_details_full_page", func(c *Client) error {
			_, err := c.Payouts.DetailsFull(context.Background(), 12345, &PayoutPageParams{Page: Int(2), PerPage: Int(500)})
			return err
		}},
	}
	for _, testCase := range calls {
		if byName[testCase.name] == "" {
			t.Fatalf("vector %s is missing", testCase.name)
		}
		doer := &scriptedDoer{responses: []scriptedResponse{okResponse(emptyPayoutPage)}}
		client, err := New(vectors.Service, vectors.SecretHash, WithHTTPClient(doer))
		if err != nil {
			t.Fatal(err)
		}
		if err := testCase.call(client); err != nil {
			t.Fatal(err)
		}
		body := doer.lastBody(t)
		if body["checksum"] != byName[testCase.name] {
			t.Errorf("%s: checksum = %v, want %s", testCase.name, body["checksum"], byName[testCase.name])
		}
		delete(body, "checksum")
		if want := decodeBody(t, bodies[testCase.name]); !reflect.DeepEqual(body, want) {
			t.Errorf("%s: body = %v, want %v", testCase.name, body, want)
		}
		keys := keysInOrder(t, doer.bodies[0])
		if wantKeys := keysInOrder(t, bodies[testCase.name]); !reflect.DeepEqual(keys[:len(keys)-1], wantKeys) {
			t.Errorf("%s: keys = %v, want %v", testCase.name, keys, wantKeys)
		}
	}
}
