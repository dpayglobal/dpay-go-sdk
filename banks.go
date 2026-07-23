package dpay

import (
	"context"

	"github.com/dpayglobal/dpay-go-sdk/internal/wire"
)

// BankService lists the banks available for online transfers.
type BankService struct {
	client *Client
}

// All returns every bank dpay supports.
func (s *BankService) All(ctx context.Context) ([]*Bank, error) {
	data, err := s.client.getJSONArray(ctx, hostPanel, "/api/v1/pbl/banks")
	if err != nil {
		return nil, err
	}
	return mapBanks(data), nil
}

// ForService returns the banks enabled for this payment point. Without
// WithTimestamp the current time is sent, exactly as the PHP SDK does.
func (s *BankService) ForService(ctx context.Context, opts ...TimestampOption) ([]*Bank, error) {
	body := wire.NewBody()
	body.Set("service", s.client.service)
	body.Set("timestamp", resolveTimestamp(opts))
	body.Set("checksum", s.client.checksum.OrderedBody(body.Values()))

	data, err := s.client.postJSONArray(ctx, hostPanel, "/api/v1/pbl/banks", body)
	if err != nil {
		return nil, err
	}
	return mapBanks(data), nil
}

func mapBanks(data []any) []*Bank {
	banks := make([]*Bank, 0, len(data))
	for _, entry := range data {
		if object, ok := entry.(map[string]any); ok {
			banks = append(banks, bankFromAPI(object))
		}
	}
	return banks
}

// Bank is a single bank offered on the payment page.
type Bank struct {
	raw      map[string]any
	id       string
	name     string
	image    string
	onFrom   int64
	onTo     int64
	iterator *int64
	test     bool
	bankType string
}

func bankFromAPI(data map[string]any) *Bank {
	bank := &Bank{
		raw:      data,
		id:       stringField(data, "id"),
		name:     stringField(data, "name"),
		image:    stringField(data, "image"),
		onFrom:   intField(data, "on_from"),
		onTo:     intField(data, "on_to"),
		test:     boolField(data, "test"),
		bankType: stringField(data, "type"),
	}
	bank.iterator = optionalInt(data, "iterator")
	return bank
}

// ID returns the bank identifier used when preselecting a channel.
func (b *Bank) ID() string { return b.id }

// Name returns the display name.
func (b *Bank) Name() string { return b.name }

// Image returns the logo URL, empty when the API sent none.
func (b *Bank) Image() string { return b.image }

// OnFrom returns the hour from which the bank accepts payments.
func (b *Bank) OnFrom() int64 { return b.onFrom }

// OnTo returns the hour until which the bank accepts payments.
func (b *Bank) OnTo() int64 { return b.onTo }

// Iterator returns the display order, nil when the API sent none.
func (b *Bank) Iterator() *int64 { return b.iterator }

// IsTest reports whether this is a sandbox bank.
func (b *Bank) IsTest() bool { return b.test }

// Type returns the bank type.
func (b *Bank) Type() string { return b.bankType }

// Raw returns the decoded bank object.
func (b *Bank) Raw() map[string]any { return b.raw }
