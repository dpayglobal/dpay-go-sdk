package dpay

import (
	"context"
	"regexp"

	"github.com/dpayglobal/dpay-go-sdk/internal/php"

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
	amount    *Money
	reason    *string
	webhook   *WebhookTarget
	messageID *string
}

// RefundMessageIDHeader is the request header with your own refund identifier
// (WithRefundMessageID); it sets the refund transfer title.
const RefundMessageIDHeader = "DigitalPayments-PSP-Message-ID"

var refundMessageIDPattern = regexp.MustCompile(`^[\x20-\x7E]+$`)

// WithRefundMessageID sends your own refund identifier in the
// DigitalPayments-PSP-Message-ID header (RefundMessageIDHeader). It is not part
// of the body or the checksum: the refund transfer title becomes
// "dpay.pl zwrot <messageID>" and Refund.Message() returns it. It must be
// printable ASCII and not only spaces; the API keeps the first 24 characters.
// Only Create sends it - CheckAvailability ignores it.
func WithRefundMessageID(messageID string) RefundOption {
	return func(options *refundOptions) { options.messageID = &messageID }
}

// WithRefundAmount refunds only part of the transaction.
func WithRefundAmount(amount Money) RefundOption {
	return func(options *refundOptions) { options.amount = &amount }
}

// WithRefundReason attaches a reason to the refund.
func WithRefundReason(reason string) RefundOption {
	return func(options *refundOptions) { options.reason = &reason }
}

// WithRefundWebhook sends the refund.succeeded / refund.failed events of this
// refund also to target, signed with the webhook secret of the service. Unlike
// in a payment registration the webhook object enters the checksum: its url and
// events are hashed in the order they are sent. Only Create sends it -
// CheckAvailability ignores it, like the PHP SDK.
func WithRefundWebhook(target WebhookTarget) RefundOption {
	return func(options *refundOptions) { options.webhook = &target }
}

func resolveRefundOptions(opts []RefundOption) *refundOptions {
	options := &refundOptions{}
	for _, apply := range opts {
		apply(options)
	}
	return options
}

func (s *RefundService) signedBody(transactionID string, options *refundOptions, withWebhook bool) *wire.Body {
	body := wire.NewBody()
	body.Set("service", s.client.service)
	body.Set("transaction_id", transactionID)
	if options.amount != nil {
		body.Set("value", options.amount.String())
	}
	if options.reason != nil {
		body.Set("reason", *options.reason)
	}
	if withWebhook && options.webhook != nil {
		body.Set("webhook", options.webhook.toBody())
	}
	body.Set("checksum", s.client.checksum.OrderedBody(body))
	return body
}

// Create orders a refund, in full unless WithRefundAmount narrows it. A
// successful response means dpay accepted the refund; its outcome comes as a
// refund.succeeded or refund.failed webhook event.
func (s *RefundService) Create(ctx context.Context, transactionID string, opts ...RefundOption) (*Refund, error) {
	options := resolveRefundOptions(opts)
	if options.webhook != nil {
		if err := options.webhook.validateFor(RefundEventTypes(), "a refund"); err != nil {
			return nil, err
		}
	}
	var headers []requestHeader
	if options.messageID != nil {
		messageID := *options.messageID
		if !refundMessageIDPattern.MatchString(messageID) || php.Trim(messageID) == "" {
			return nil, newValidationError("Refund message ID must be non-empty printable ASCII")
		}
		headers = append(headers, requestHeader{name: RefundMessageIDHeader, value: messageID})
	}
	data, err := s.client.postJSONObject(ctx, hostPanel, "/api/v1/pbl/refund", s.signedBody(transactionID, options, true), headers...)
	if err != nil {
		return nil, err
	}
	return refundFromAPI(data), nil
}

// CheckAvailability reports whether a refund can be issued. Every answer that
// carries the refund key - HTTP 200 and the business refusals 400, 402, 404
// (transaction does not exist), 406, 409, 410, 411, 423 and a business 401 - is
// returned as RefundAvailability (HTTPStatus tells the reason apart). An
// invalid checksum (401 "Unauthorized request"), validation errors (422), rate
// limits and server errors are returned as errors.
func (s *RefundService) CheckAvailability(ctx context.Context, transactionID string, opts ...RefundOption) (*RefundAvailability, error) {
	response, err := s.client.sendBody(ctx, "POST", hostPanel,
		"/api/v1/pbl/check-refund-availability", s.signedBody(transactionID, resolveRefundOptions(opts), false))
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
	if status == 401 {
		message, ok := data["message"].(string)
		return !ok || message != "Unauthorized request"
	}
	return status == 200 || (status >= 400 && status < 500)
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
