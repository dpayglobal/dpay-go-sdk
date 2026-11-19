package dpay

import (
	"context"
	"fmt"

	"github.com/dpayglobal/dpay-go-sdk/internal/php"
	"github.com/dpayglobal/dpay-go-sdk/internal/wire"
)

// PayoutService lists payouts and reads their details.
type PayoutService struct {
	client *Client
}

// Details reads a payout by its withdraw identifier. Without WithTimestamp no
// timestamp field is sent at all, matching the PHP SDK.
func (s *PayoutService) Details(ctx context.Context, withdrawID int64, opts ...TimestampOption) (*PayoutDetails, error) {
	body := wire.NewBody()
	body.Set("service", s.client.service)
	if len(opts) > 0 {
		body.Set("timestamp", resolveTimestamp(opts))
	}
	body.Set("withdraw_id", withdrawID)
	body.Set("checksum", s.client.checksum.OrderedBody(body))

	data, err := s.client.postJSONObject(ctx, hostPanel, "/api/v1/pbl/withdraws/details", body)
	if err != nil {
		return nil, err
	}
	return payoutDetailsFromAPI(data), nil
}

// PayoutListParams filters the payout list. Every nil (or empty) field is omitted.
type PayoutListParams struct {
	// DateFrom keeps payouts created at or after this date or date-time (any
	// format the API parses, e.g. 2026-08-01). It is sent and signed trimmed:
	// the API trims every string before it verifies the checksum.
	DateFrom *string
	// DateTo keeps payouts created at or before this date or date-time, e.g.
	// 2026-08-31 23:59:59. Sent trimmed like DateFrom.
	DateTo *string
	// State keeps payouts in this state.
	State *PayoutState
	// Type keeps payouts of this type; empty means any.
	Type PayoutType
	// Page is the page number, at least 1 (the API defaults to 1). Iterate ignores it.
	Page *int
	// PerPage is the page size, 1-200.
	PerPage *int
}

// PayoutPageParams selects a page of the positions of a payout.
type PayoutPageParams struct {
	// Page is the page number, at least 1.
	Page *int
	// PerPage is the page size, 1-500.
	PerPage *int
}

// List returns one page of payouts, newest first. The body is signed with the
// ordered-body checksum in the order service, date_from, date_to, state, type,
// page, per_page. A nil params lists every payout.
func (s *PayoutService) List(ctx context.Context, params *PayoutListParams) (*PayoutPage, error) {
	filters := PayoutListParams{}
	if params != nil {
		filters = *params
	}
	body := wire.NewBody()
	body.Set("service", s.client.service)
	for _, date := range []struct {
		name  string
		value *string
	}{{"date_from", filters.DateFrom}, {"date_to", filters.DateTo}} {
		if date.value == nil {
			continue
		}
		// The API trims strings before it verifies the checksum - an untrimmed value would fail with 401
		trimmed := php.Trim(*date.value)
		if trimmed == "" {
			return nil, newValidationError(date.name + " must not be empty")
		}
		body.Set(date.name, trimmed)
	}
	if filters.State != nil {
		if !filters.State.Valid() {
			return nil, newValidationError(fmt.Sprintf(`Invalid payout state "%d"`, int(*filters.State)))
		}
		body.Set("state", int(*filters.State))
	}
	if filters.Type != "" {
		if !filters.Type.Valid() {
			return nil, newValidationError(fmt.Sprintf("Invalid payout type %q", string(filters.Type)))
		}
		body.Set("type", string(filters.Type))
	}
	if err := setPage(body, filters.Page, filters.PerPage, 200); err != nil {
		return nil, err
	}
	body.Set("checksum", s.client.checksum.OrderedBody(body))

	data, err := s.client.postJSONObject(ctx, hostPanel, "/api/v1/pbl/withdraws", body)
	if err != nil {
		return nil, err
	}
	return payoutPageFromAPI(data), nil
}

// Iterate walks every matching payout page by page (page 1, 2, ...; the page
// is always sent, so it is part of the checksum), fetching the next page only
// when the current one is used up:
//
//	payouts := client.Payouts.Iterate(ctx, &dpay.PayoutListParams{State: &state})
//	for payouts.Next() {
//		handle(payouts.Payout())
//	}
//	if err := payouts.Err(); err != nil {
//		// ...
//	}
func (s *PayoutService) Iterate(ctx context.Context, params *PayoutListParams) *PayoutIterator {
	iterator := &PayoutIterator{service: s, ctx: ctx}
	if params != nil {
		iterator.params = *params
	}
	return iterator
}

// DetailsFull reads a payout with its settlement summary and the paginated
// list of settled transactions. The body is signed with the ordered-body
// checksum in the order service, withdraw_id, page, per_page. A nil page asks
// for the first page with the default size. A missing payout is ErrNotFound.
func (s *PayoutService) DetailsFull(ctx context.Context, withdrawID int64, page *PayoutPageParams) (*PayoutSettlement, error) {
	body := wire.NewBody()
	body.Set("service", s.client.service)
	body.Set("withdraw_id", withdrawID)
	if page != nil {
		if err := setPage(body, page.Page, page.PerPage, 500); err != nil {
			return nil, err
		}
	}
	body.Set("checksum", s.client.checksum.OrderedBody(body))

	data, err := s.client.postJSONObject(ctx, hostPanel, "/api/v1/pbl/withdraws/details/full", body)
	if err != nil {
		return nil, err
	}
	return payoutSettlementFromAPI(data), nil
}

func setPage(body *wire.Body, page, perPage *int, maxPerPage int) error {
	if page != nil {
		if *page < 1 {
			return newValidationError("page must be at least 1")
		}
		body.Set("page", *page)
	}
	if perPage != nil {
		if *perPage < 1 || *perPage > maxPerPage {
			return newValidationError(fmt.Sprintf("per_page must be between 1 and %d", maxPerPage))
		}
		body.Set("per_page", *perPage)
	}
	return nil
}

// PayoutIterator walks the payouts returned by PayoutService.Iterate. It is not
// safe for concurrent use.
type PayoutIterator struct {
	service *PayoutService
	ctx     context.Context
	params  PayoutListParams
	page    []*PayoutListItem
	index   int
	current *PayoutListItem
	next    int
	more    bool
	err     error
}

// Next advances to the next payout, fetching the next page when needed. It
// returns false at the end of the list or after an error (see Err).
func (it *PayoutIterator) Next() bool {
	for {
		if it.index < len(it.page) {
			it.current = it.page[it.index]
			it.index++
			return true
		}
		it.current = nil
		if it.err != nil || (it.next > 0 && !it.more) {
			return false
		}
		it.next++
		params := it.params
		params.Page = Int(it.next)
		page, err := it.service.List(it.ctx, &params)
		if err != nil {
			it.err = err
			return false
		}
		it.page, it.index = page.Items(), 0
		it.more = page.HasMore()
	}
}

// Payout returns the payout Next advanced to.
func (it *PayoutIterator) Payout() *PayoutListItem { return it.current }

// Err returns the error that stopped the iteration, nil at a regular end.
func (it *PayoutIterator) Err() error { return it.err }

// PayoutPage is one page of the payout list.
type PayoutPage struct {
	raw   map[string]any
	items []*PayoutListItem
	meta  *PageMeta
}

func payoutPageFromAPI(data map[string]any) *PayoutPage {
	page := &PayoutPage{raw: data, items: []*PayoutListItem{}, meta: pageMetaFromAPI(phpObject(data["meta"]))}
	for _, item := range arrayField(data, "withdraws") {
		if row, ok := phpArray(item); ok {
			page.items = append(page.items, payoutListItemFromAPI(row))
		}
	}
	return page
}

// Items returns the payouts of this page.
func (p *PayoutPage) Items() []*PayoutListItem { return p.items }

// Meta returns the pagination of the list.
func (p *PayoutPage) Meta() *PageMeta { return p.meta }

// HasMore reports whether the page is not empty and further pages follow.
func (p *PayoutPage) HasMore() bool { return len(p.items) > 0 && p.meta.HasMore() }

// Raw returns the decoded response body.
func (p *PayoutPage) Raw() map[string]any { return p.raw }

// PageMeta is the pagination of a payout list or of payout positions.
type PageMeta struct {
	page       int64
	perPage    int64
	total      int64
	totalPages int64
}

func pageMetaFromAPI(data map[string]any) *PageMeta {
	return &PageMeta{
		page:       numericInt(data, "page", 1),
		perPage:    numericInt(data, "per_page", 0),
		total:      numericInt(data, "total", 0),
		totalPages: numericInt(data, "total_pages", 0),
	}
}

// Page returns the current page number, 1 when the API sent none.
func (m *PageMeta) Page() int64 { return m.page }

// PerPage returns the page size.
func (m *PageMeta) PerPage() int64 { return m.perPage }

// Total returns the number of entries on all pages.
func (m *PageMeta) Total() int64 { return m.total }

// TotalPages returns the number of pages, 0 when the API sent none.
func (m *PageMeta) TotalPages() int64 { return m.totalPages }

// HasMore reports whether pages follow the current one.
func (m *PageMeta) HasMore() bool { return m.page < m.totalPages }

// PayoutListItem is one payout of the payout list.
type PayoutListItem struct {
	raw      map[string]any
	currency Currency
}

func payoutListItemFromAPI(data map[string]any) *PayoutListItem {
	return &PayoutListItem{raw: data, currency: currencyCode(data, "currency_code", CurrencyPLN)}
}

// ID returns the payout (withdraw) identifier, the one Details and DetailsFull take.
func (p *PayoutListItem) ID() int64 { return numericInt(p.raw, "id", 0) }

// Type returns standard, direct or paysafecard, preserved verbatim even when unknown.
func (p *PayoutListItem) Type() PayoutType { return PayoutType(strictString(p.raw, "type")) }

// State returns the raw state code: 0 waiting, 1 processed, -1 failed.
func (p *PayoutListItem) State() int64 { return numericInt(p.raw, "state", 0) }

// IsWaiting reports whether the payout is still queued.
func (p *PayoutListItem) IsWaiting() bool { return p.State() == int64(PayoutStateWaiting) }

// IsProcessed reports whether the payout was sent.
func (p *PayoutListItem) IsProcessed() bool { return p.State() == int64(PayoutStateProcessed) }

// IsFailed reports whether the payout failed.
func (p *PayoutListItem) IsFailed() bool { return p.State() == int64(PayoutStateFailed) }

// Net returns the amount paid out, 0 when missing or unparsable.
func (p *PayoutListItem) Net() Money { return moneyField(p.raw, "net", p.currency) }

// Fee returns the fee charged.
func (p *PayoutListItem) Fee() Money { return moneyField(p.raw, "fee", p.currency) }

// Gross returns the amount including fees.
func (p *PayoutListItem) Gross() Money { return moneyField(p.raw, "gross", p.currency) }

// Currency returns currency_code, PLN when missing or invalid.
func (p *PayoutListItem) Currency() Currency { return p.currency }

// CreationDate returns the creation timestamp as reported by the API.
func (p *PayoutListItem) CreationDate() string { return strictString(p.raw, "creation_date") }

// ExportDate returns when the payout was sent to the bank, empty before.
func (p *PayoutListItem) ExportDate() string { return strictString(p.raw, "export_date") }

// IsDeclined reports whether the payout was declined.
func (p *PayoutListItem) IsDeclined() bool { return numericInt(p.raw, "declined", 0) == 1 }

// Raw returns the decoded payout row.
func (p *PayoutListItem) Raw() map[string]any { return p.raw }

// PayoutSettlement is a payout with its settlement summary and a page of the
// settled transactions (DetailsFull).
type PayoutSettlement struct {
	raw        map[string]any
	settlement *PayoutDetails
	positions  []*SettlementPosition
	meta       *PageMeta
}

func payoutSettlementFromAPI(data map[string]any) *PayoutSettlement {
	settlement := &PayoutSettlement{
		raw:        data,
		settlement: payoutDetailsFromAPI(phpObject(data["settlement"])),
		positions:  []*SettlementPosition{},
		meta:       pageMetaFromAPI(phpObject(data["meta"])),
	}
	currency := settlement.settlement.Currency()
	for _, item := range arrayField(data, "transactions") {
		if row, ok := phpArray(item); ok {
			settlement.positions = append(settlement.positions, &SettlementPosition{raw: row, currency: currency})
		}
	}
	return settlement
}

// Settlement returns the payout itself.
func (p *PayoutSettlement) Settlement() *PayoutDetails { return p.settlement }

// SummaryNet returns the net amount of the whole settlement (not only the page).
func (p *PayoutSettlement) SummaryNet() Money { return p.summaryMoney("net") }

// SummaryFee returns the fees of the whole settlement.
func (p *PayoutSettlement) SummaryFee() Money { return p.summaryMoney("fee") }

// SummaryGross returns the gross amount of the whole settlement.
func (p *PayoutSettlement) SummaryGross() Money { return p.summaryMoney("gross") }

// SummaryCount returns the number of transactions in the whole settlement.
func (p *PayoutSettlement) SummaryCount() int64 { return numericInt(p.summary(), "count", 0) }

// Positions returns the settled transactions of this page.
func (p *PayoutSettlement) Positions() []*SettlementPosition { return p.positions }

// Meta returns the pagination of the positions.
func (p *PayoutSettlement) Meta() *PageMeta { return p.meta }

// Raw returns the decoded response body.
func (p *PayoutSettlement) Raw() map[string]any { return p.raw }

func (p *PayoutSettlement) summary() map[string]any { return phpObject(p.raw["summary"]) }

func (p *PayoutSettlement) summaryMoney(key string) Money {
	return moneyField(p.summary(), key, p.settlement.Currency())
}

// SettlementPosition is one settled transaction of a payout, its amounts in
// the currency of the payout.
type SettlementPosition struct {
	raw      map[string]any
	currency Currency
}

// PaymentID returns the transaction identifier.
func (p *SettlementPosition) PaymentID() string { return strictString(p.raw, "payment_id") }

// Date returns the transaction date as reported by the API.
func (p *SettlementPosition) Date() string { return strictString(p.raw, "date") }

// Net returns the amount after fees.
func (p *SettlementPosition) Net() Money { return moneyField(p.raw, "net", p.currency) }

// Fee returns the fee charged.
func (p *SettlementPosition) Fee() Money { return moneyField(p.raw, "fee", p.currency) }

// Gross returns the transaction amount.
func (p *SettlementPosition) Gross() Money { return moneyField(p.raw, "gross", p.currency) }

// ServiceID returns the id of the payment point, nil unless it is a JSON integer.
func (p *SettlementPosition) ServiceID() *int64 { return optionalJSONInt(p.raw, "service_id") }

// ServiceName returns the name of the payment point.
func (p *SettlementPosition) ServiceName() string { return strictString(p.raw, "service_name") }

// Custom returns the custom value of the registration.
func (p *SettlementPosition) Custom() string { return strictString(p.raw, "custom") }

// IPKSeF returns the KSeF invoice identifier.
func (p *SettlementPosition) IPKSeF() string { return strictString(p.raw, "ipksef") }

// Raw returns the decoded transaction row.
func (p *SettlementPosition) Raw() map[string]any { return p.raw }

// PayoutDetails is the state of a single payout.
type PayoutDetails struct {
	raw              map[string]any
	currency         Currency
	id               int64
	state            int64
	net              Money
	fee              Money
	gross            Money
	creationDate     string
	directSettlement bool
	nrb              string
	declined         bool
	declineReason    string
	declineStatus    string
	receiver         *PayoutReceiver
}

func payoutDetailsFromAPI(data map[string]any) *PayoutDetails {
	currency := currencyCode(data, "currency_code", CurrencyPLN)
	details := &PayoutDetails{
		raw:              data,
		currency:         currency,
		id:               intField(data, "id"),
		state:            intField(data, "state"),
		net:              moneyField(data, "net", currency),
		fee:              moneyField(data, "fee", currency),
		gross:            moneyField(data, "gross", currency),
		creationDate:     stringField(data, "creation_date"),
		directSettlement: intField(data, "direct_settlement") == 1,
		nrb:              stringField(data, "nrb"),
		declined:         intField(data, "declined") == 1,
		declineReason:    stringField(data, "decline_reason"),
		declineStatus:    stringField(data, "decline_status"),
	}
	if receiver, ok := data["receiver"].(map[string]any); ok {
		details.receiver = &PayoutReceiver{raw: receiver, currency: currency}
	}
	return details
}

// ID returns the payout identifier.
func (p *PayoutDetails) ID() int64 { return p.id }

// State returns the raw state code: 0 waiting, 1 processed, -1 failed.
func (p *PayoutDetails) State() int64 { return p.state }

// IsWaiting reports whether the payout is still queued.
func (p *PayoutDetails) IsWaiting() bool { return p.state == int64(PayoutStateWaiting) }

// IsProcessed reports whether the payout was sent.
func (p *PayoutDetails) IsProcessed() bool { return p.state == int64(PayoutStateProcessed) }

// IsFailed reports whether the payout failed.
func (p *PayoutDetails) IsFailed() bool { return p.state == int64(PayoutStateFailed) }

// Net returns the amount before fees.
func (p *PayoutDetails) Net() Money { return p.net }

// Fee returns the fee charged.
func (p *PayoutDetails) Fee() Money { return p.fee }

// Gross returns the amount including fees.
func (p *PayoutDetails) Gross() Money { return p.gross }

// Currency returns currency_code, PLN when missing or invalid (Details sends
// no currency). Net, Fee, Gross and the receiver amount use it.
func (p *PayoutDetails) Currency() Currency { return p.currency }

// CreationDate returns the creation timestamp as reported by the API.
func (p *PayoutDetails) CreationDate() string { return p.creationDate }

// ExportDate returns when the payout was sent to the bank (DetailsFull only),
// empty when the API sent none.
func (p *PayoutDetails) ExportDate() string { return strictString(p.raw, "export_date") }

// IsPaysafecard reports whether this is a paysafecard payout.
func (p *PayoutDetails) IsPaysafecard() bool {
	value, exists := p.raw["paysafecard"]
	return exists && php.IsScalar(value) && php.Int(value) == 1
}

// IsDirectSettlement reports whether this was a direct settlement.
func (p *PayoutDetails) IsDirectSettlement() bool { return p.directSettlement }

// NRB returns the account number the payout was sent to.
func (p *PayoutDetails) NRB() string { return p.nrb }

// IsDeclined reports whether the payout was declined.
func (p *PayoutDetails) IsDeclined() bool { return p.declined }

// DeclineReason returns why the payout was declined.
func (p *PayoutDetails) DeclineReason() string { return p.declineReason }

// DeclineStatus returns the decline status code.
func (p *PayoutDetails) DeclineStatus() string { return p.declineStatus }

// Receiver returns the payout recipient, nil when the API sent none.
func (p *PayoutDetails) Receiver() *PayoutReceiver { return p.receiver }

// Raw returns the decoded response body.
func (p *PayoutDetails) Raw() map[string]any { return p.raw }

// PayoutReceiver is the recipient of a payout.
type PayoutReceiver struct {
	raw      map[string]any
	currency Currency
}

// NRB returns the recipient account number.
func (r *PayoutReceiver) NRB() string { return stringField(r.raw, "nrb") }

// Title returns the transfer title.
func (r *PayoutReceiver) Title() string { return stringField(r.raw, "title") }

// Amount returns the payout amount and whether the API sent a parsable one.
func (r *PayoutReceiver) Amount() (Money, bool) {
	currency := r.currency
	if currency == "" {
		currency = CurrencyPLN
	}
	return moneyFromAPI(r.raw["amount"], currency)
}

// Service returns the payment point the payout came from.
func (r *PayoutReceiver) Service() string { return stringField(r.raw, "service") }

// ReceiverName returns the recipient name.
func (r *PayoutReceiver) ReceiverName() string { return stringField(r.raw, "receiverName") }

// ReceiverAddress returns the recipient address.
func (r *PayoutReceiver) ReceiverAddress() string { return stringField(r.raw, "receiverAddress") }

// Raw returns the decoded receiver object.
func (r *PayoutReceiver) Raw() map[string]any { return r.raw }
