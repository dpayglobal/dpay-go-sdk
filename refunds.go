package dpay

import (
	"context"

	"github.com/dpayglobal/dpay-go-sdk/internal/wire"
)

// RefundService creates refunds and checks their availability.
type RefundService struct {
	client *Client
}

// RefundOption sets an optional field of a refund request. Unlike an absent
// option, an explicitly set value is always sent and always enters the checksum.
type RefundOption func(*refundOptions)

type refundOptions struct {
	amount *Money
	reason *string
}

// WithRefundAmount refunds only part of the transaction.
func WithRefundAmount(amount Money) RefundOption {
	return func(options *refundOptions) { options.amount = &amount }
}

// WithRefundReason attaches a reason to the refund.
func WithRefundReason(reason string) RefundOption {
	return func(options *refundOptions) { options.reason = &reason }
}

func (s *RefundService) signedBody(transactionID string, opts []RefundOption) *wire.Body {
	options := &refundOptions{}
	for _, apply := range opts {
		apply(options)
	}
	body := wire.NewBody()
	body.Set("service", s.client.service)
	body.Set("transaction_id", transactionID)
	if options.amount != nil {
		body.Set("value", options.amount.String())
	}
	if options.reason != nil {
		body.Set("reason", *options.reason)
	}
	body.Set("checksum", s.client.checksum.OrderedBody(body.Values()))
	return body
}

// Create refunds a transaction, in full unless WithRefundAmount narrows it.
func (s *RefundService) Create(ctx context.Context, transactionID string, opts ...RefundOption) (*Refund, error) {
	data, err := s.client.postJSONObject(ctx, hostPanel, "/api/v1/pbl/refund", s.signedBody(transactionID, opts))
	if err != nil {
		return nil, err
	}
	return refundFromAPI(data), nil
}

var availabilityOutcomeStatuses = map[int]bool{200: true, 400: true, 402: true, 406: true, 409: true, 410: true, 411: true}

// CheckAvailability reports whether a refund can be issued. Statuses the API
// uses to express a business outcome are returned as a result, not as an error.
func (s *RefundService) CheckAvailability(ctx context.Context, transactionID string, opts ...RefundOption) (*RefundAvailability, error) {
	response, err := s.client.sendBody(ctx, "POST", hostPanel,
		"/api/v1/pbl/check-refund-availability", s.signedBody(transactionID, opts))
	if err != nil {
		return nil, err
	}
	data, decoded := response.decodeJSONObject()
	if decoded {
		if _, present := data["refund"]; present && isAvailabilityOutcome(response.status, data) {
			return refundAvailabilityFromAPI(data, response.status), nil
		}
	}
	return nil, mapAPIError(response)
}

func isAvailabilityOutcome(status int, data map[string]any) bool {
	if availabilityOutcomeStatuses[status] {
		return true
	}
	if status != 401 {
		return false
	}
	message, ok := data["message"].(string)
	return !ok || message != "Unauthorized request"
}

// Refund is the outcome of a refund request.
type Refund struct {
	raw      map[string]any
	accepted bool
	message  string
}

func refundFromAPI(data map[string]any) *Refund {
	status, _ := data["status"].(string)
	accepted, _ := data["refund"].(bool)
	return &Refund{
		raw:      data,
		accepted: status == "success" && accepted,
		message:  stringField(data, "message"),
	}
}

// IsAccepted reports whether dpay accepted the refund.
func (r *Refund) IsAccepted() bool { return r.accepted }

// Message returns the message the API returned.
func (r *Refund) Message() string { return r.message }

// Raw returns the decoded response body.
func (r *Refund) Raw() map[string]any { return r.raw }

// RefundAvailability reports whether a transaction can still be refunded.
type RefundAvailability struct {
	raw        map[string]any
	available  bool
	message    string
	httpStatus int
}

func refundAvailabilityFromAPI(data map[string]any, status int) *RefundAvailability {
	available, _ := data["refund"].(bool)
	return &RefundAvailability{
		raw:        data,
		available:  available,
		message:    stringField(data, "message"),
		httpStatus: status,
	}
}

// IsAvailable reports whether a refund can be issued.
func (a *RefundAvailability) IsAvailable() bool { return a.available }

// Message returns the explanation the API returned.
func (a *RefundAvailability) Message() string { return a.message }

// HTTPStatus returns the status the API answered with, which encodes the reason.
func (a *RefundAvailability) HTTPStatus() int { return a.httpStatus }

// Raw returns the decoded response body.
func (a *RefundAvailability) Raw() map[string]any { return a.raw }
