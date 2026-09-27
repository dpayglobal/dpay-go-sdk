package dpay

import (
	"net/http"
	"time"

	"github.com/dpayglobal/dpay-go-sdk/internal/wire"
)

const (
	hostAPIPayments = wire.HostAPIPayments
	hostPanel       = wire.HostPanel
	hostGateway     = wire.HostGateway
)

const defaultTimeout = 30 * time.Second

// BaseURLs overrides the API hosts. An empty field keeps the production default.
type BaseURLs struct {
	// APIPayments defaults to https://api-payments.dpay.pl.
	APIPayments string
	// Panel defaults to https://panel.dpay.pl.
	Panel string
	// Gateway defaults to https://secure.dpay.pl.
	Gateway string
}

// Option configures a Client.
type Option func(*clientConfig)

type clientConfig struct {
	timeout    time.Duration
	httpClient HTTPDoer
	baseURLs   BaseURLs
	err        error
}

// WithTimeout sets the timeout of the default HTTP client. It is ignored when
// WithHTTPClient is used.
func WithTimeout(timeout time.Duration) Option {
	return func(config *clientConfig) {
		if timeout <= 0 {
			config.err = newValidationError(`Option "timeout" must be a positive integer`)
			return
		}
		config.timeout = timeout
	}
}

// WithHTTPClient sets the transport used for every request.
func WithHTTPClient(doer HTTPDoer) Option {
	return func(config *clientConfig) {
		if doer == nil {
			config.err = newValidationError(`Option "http_client" must implement HTTPDoer`)
			return
		}
		config.httpClient = doer
	}
}

// WithBaseURLs overrides the API hosts, trimming trailing slashes.
func WithBaseURLs(urls BaseURLs) Option {
	return func(config *clientConfig) {
		config.baseURLs = urls
	}
}

// Client is the entry point to the dpay API. Its service fields are safe for
// concurrent use.
type Client struct {
	// Payments registers payments and reads transaction details.
	Payments *PaymentService
	// Refunds creates refunds and checks their availability.
	Refunds *RefundService
	// Banks lists the available banks.
	Banks *BankService
	// Blik manages BLIK OneClick aliases.
	Blik *BlikService
	// Cards runs server-to-server card operations.
	Cards *CardService
	// Payouts reads payout details.
	Payouts *PayoutService
	// Recurring reads, retries and cancels recurring payments.
	Recurring *RecurringService
	// Events reads the event history the webhooks deliver.
	Events *EventService

	service    string
	checksum   wire.Checksum
	baseURLs   wire.BaseURLs
	httpClient HTTPDoer
}

// New returns a Client for a payment point identified by service and its secret hash.
func New(service, secretHash string, opts ...Option) (*Client, error) {
	if service == "" {
		return nil, newValidationError(`Option "service" is required and must be a non-empty string`)
	}
	if secretHash == "" {
		return nil, newValidationError(`Option "secret_hash" is required and must be a non-empty string`)
	}

	config := &clientConfig{timeout: defaultTimeout}
	for _, apply := range opts {
		apply(config)
	}
	if config.err != nil {
		return nil, config.err
	}

	httpClient := config.httpClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: config.timeout, CheckRedirect: doNotFollowRedirects}
	}

	client := &Client{
		service:    service,
		checksum:   wire.NewChecksum(secretHash),
		baseURLs:   wire.NewBaseURLs(config.baseURLs.APIPayments, config.baseURLs.Panel, config.baseURLs.Gateway),
		httpClient: httpClient,
	}
	client.Payments = &PaymentService{client: client}
	client.Refunds = &RefundService{client: client}
	client.Banks = &BankService{client: client}
	client.Blik = &BlikService{client: client}
	client.Cards = &CardService{client: client}
	client.Payouts = &PayoutService{client: client}
	client.Recurring = &RecurringService{client: client}
	client.Events = &EventService{client: client}
	return client, nil
}

// Service returns the payment point name the client authenticates as.
func (c *Client) Service() string {
	return c.service
}

// doNotFollowRedirects makes the default transport surface a redirect as the
// response itself. Two reasons: the PHP SDK does not set CURLOPT_FOLLOWLOCATION,
// so following one would report a different status than the reference SDK; and a
// payments client must never re-send a request to a host chosen by the response.
func doNotFollowRedirects(*http.Request, []*http.Request) error {
	return http.ErrUseLastResponse
}

// String returns a pointer to s, for optional string fields.
func String(s string) *string { return &s }

// Bool returns a pointer to b, for optional bool fields. Use it to send false explicitly.
func Bool(b bool) *bool { return &b }

// Int returns a pointer to i, for optional int fields.
func Int(i int) *int { return &i }

// Int64 returns a pointer to i, for optional int64 fields.
func Int64(i int64) *int64 { return &i }

// TimestampOption overrides the timestamp an operation would otherwise read from the clock.
type TimestampOption func(*int64)

// WithTimestamp pins the timestamp sent with the request.
func WithTimestamp(timestamp int64) TimestampOption {
	return func(target *int64) { *target = timestamp }
}

func resolveTimestamp(opts []TimestampOption) int64 {
	timestamp := time.Now().Unix()
	for _, apply := range opts {
		apply(&timestamp)
	}
	return timestamp
}
