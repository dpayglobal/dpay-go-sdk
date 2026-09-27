package dpay

import (
	"context"

	"github.com/dpayglobal/dpay-go-sdk/internal/wire"
)

// PayoutService reads the state of payouts.
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

// PayoutDetails is the state of a single payout.
type PayoutDetails struct {
	raw              map[string]any
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
	details := &PayoutDetails{
		raw:              data,
		id:               intField(data, "id"),
		state:            intField(data, "state"),
		net:              moneyField(data, "net", CurrencyPLN),
		fee:              moneyField(data, "fee", CurrencyPLN),
		gross:            moneyField(data, "gross", CurrencyPLN),
		creationDate:     stringField(data, "creation_date"),
		directSettlement: intField(data, "direct_settlement") == 1,
		nrb:              stringField(data, "nrb"),
		declined:         intField(data, "declined") == 1,
		declineReason:    stringField(data, "decline_reason"),
		declineStatus:    stringField(data, "decline_status"),
	}
	if receiver, ok := data["receiver"].(map[string]any); ok {
		details.receiver = &PayoutReceiver{raw: receiver}
	}
	return details
}

// ID returns the payout identifier.
func (p *PayoutDetails) ID() int64 { return p.id }

// State returns the raw state code: 0 waiting, 1 processed, -1 failed.
func (p *PayoutDetails) State() int64 { return p.state }

// IsWaiting reports whether the payout is still queued.
func (p *PayoutDetails) IsWaiting() bool { return p.state == 0 }

// IsProcessed reports whether the payout was sent.
func (p *PayoutDetails) IsProcessed() bool { return p.state == 1 }

// IsFailed reports whether the payout failed.
func (p *PayoutDetails) IsFailed() bool { return p.state == -1 }

// Net returns the amount before fees.
func (p *PayoutDetails) Net() Money { return p.net }

// Fee returns the fee charged.
func (p *PayoutDetails) Fee() Money { return p.fee }

// Gross returns the amount including fees.
func (p *PayoutDetails) Gross() Money { return p.gross }

// CreationDate returns the creation timestamp as reported by the API.
func (p *PayoutDetails) CreationDate() string { return p.creationDate }

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
	raw map[string]any
}

// NRB returns the recipient account number.
func (r *PayoutReceiver) NRB() string { return stringField(r.raw, "nrb") }

// Title returns the transfer title.
func (r *PayoutReceiver) Title() string { return stringField(r.raw, "title") }

// Amount returns the payout amount and whether the API sent a parsable one.
func (r *PayoutReceiver) Amount() (Money, bool) {
	return moneyFromAPI(r.raw["amount"], CurrencyPLN)
}

// Service returns the payment point the payout came from.
func (r *PayoutReceiver) Service() string { return stringField(r.raw, "service") }

// ReceiverName returns the recipient name.
func (r *PayoutReceiver) ReceiverName() string { return stringField(r.raw, "receiverName") }

// ReceiverAddress returns the recipient address.
func (r *PayoutReceiver) ReceiverAddress() string { return stringField(r.raw, "receiverAddress") }

// Raw returns the decoded receiver object.
func (r *PayoutReceiver) Raw() map[string]any { return r.raw }
