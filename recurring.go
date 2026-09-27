package dpay

import (
	"context"

	"github.com/dpayglobal/dpay-go-sdk/internal/wire"
)

// RecurringService reads, retries and cancels recurring payments shared by
// payment methods (today BLIK). Registration and charges go through
// Payments.Register with RegisterPaymentRequest.RecurringRegistration and
// RegisterPaymentRequest.RecurringAlias.
//
// The API allows status about 60 and retry and cancel about 30 calls a minute,
// counted together with the rest of the payments API from your IP address. Do
// not poll the status in a loop - the outcome comes as a webhook.
type RecurringService struct {
	client *Client
}

// RecurringCancelOption sets an optional field of a recurring payment cancellation.
type RecurringCancelOption func(*recurringCancelOptions)

type recurringCancelOptions struct {
	reason *string
}

// WithCancelReason attaches a reason (up to 255 characters, BLIK receives the
// first 20). It is sent in the body but does not enter the checksum.
func WithCancelReason(reason string) RecurringCancelOption {
	return func(options *recurringCancelOptions) { options.reason = &reason }
}

// Status returns the current state and terms of the recurring payment
// registered for this service under alias.
func (s *RecurringService) Status(ctx context.Context, alias string) (*RecurringStatus, error) {
	body := wire.NewBody()
	body.Set("service", s.client.service)
	body.Set("alias", alias)
	body.Set("checksum", s.client.checksum.SecretSecond(s.client.service, []any{alias}))

	data, err := s.client.postJSONObject(ctx, hostAPIPayments, "/api/v1_0/payments/recurring/status", body)
	if err != nil {
		return nil, err
	}
	return recurringStatusFromAPI(phpObject(data["data"])), nil
}

// Retry retries a declined recurring charge with the same BLIK transaction (up
// to 3 times within 5 minutes, only after declines the customer can fix, e.g.
// INSUFFICIENT_FUNDS). transactionID is the transactionId of the declined
// charge. A retry the API does not allow returns an ErrInvalidRequest *APIError
// whose FieldErrors["retry"] holds the reason, e.g. DECLINE_NOT_RETRYABLE or
// RETRY_LIMIT_REACHED.
func (s *RecurringService) Retry(ctx context.Context, transactionID string) (*RecurringRetryResult, error) {
	body := wire.NewBody()
	body.Set("service", s.client.service)
	body.Set("transaction_id", transactionID)
	body.Set("checksum", s.client.checksum.SecretSecond(s.client.service, []any{transactionID}))

	data, err := s.client.postJSONObject(ctx, hostAPIPayments, "/api/v1_0/payments/recurring/retry", body)
	if err != nil {
		return nil, err
	}
	return recurringRetryResultFromAPI(phpObject(data["data"])), nil
}

// Cancel cancels the recurring payment (for BLIK it unregisters the alias at
// BLIK); later charges with the alias are rejected. The checksum ends with the
// operation name, so a status checksum cannot cancel. It returns the new state,
// RecurringStateUnregistered.
func (s *RecurringService) Cancel(ctx context.Context, alias string, opts ...RecurringCancelOption) (RecurringState, error) {
	options := &recurringCancelOptions{}
	for _, apply := range opts {
		apply(options)
	}

	body := wire.NewBody()
	body.Set("service", s.client.service)
	body.Set("alias", alias)
	if options.reason != nil {
		body.Set("reason", *options.reason)
	}
	body.Set("checksum", s.client.checksum.SecretSecond(s.client.service, []any{alias, "cancel"}))

	data, err := s.client.postJSONObject(ctx, hostAPIPayments, "/api/v1_0/payments/recurring/cancel", body)
	if err != nil {
		return "", err
	}
	if status, ok := phpObject(data["data"])["status"].(string); ok {
		return RecurringState(status), nil
	}
	return RecurringStateUnregistered, nil
}

// RecurringStatus is the state and terms of a recurring payment.
type RecurringStatus struct {
	raw          map[string]any
	registration *RecurringRegistrationInfo
}

func recurringStatusFromAPI(data map[string]any) *RecurringStatus {
	status := &RecurringStatus{raw: data}
	if registration, ok := phpArray(data["registration"]); ok {
		status.registration = &RecurringRegistrationInfo{raw: registration}
	}
	return status
}

// Alias returns the alias of the recurring payment.
func (s *RecurringStatus) Alias() string { return strictString(s.raw, "alias") }

// Method returns the payment method of the recurring payment, e.g. blik.
func (s *RecurringStatus) Method() string { return strictString(s.raw, "method") }

// Status returns the state, preserved verbatim even when unknown, empty when
// the API reports none.
func (s *RecurringStatus) Status() RecurringState {
	return RecurringState(strictString(s.raw, "status"))
}

// IsActive reports whether the recurring payment can be charged.
func (s *RecurringStatus) IsActive() bool { return s.Status() == RecurringStateActive }

// ExpirationDate returns when the recurring payment expires (YYYY-MM-DD).
func (s *RecurringStatus) ExpirationDate() string { return strictString(s.raw, "expiration_date") }

// Registration returns the registered terms, nil when the API sent none.
func (s *RecurringStatus) Registration() *RecurringRegistrationInfo { return s.registration }

// Raw returns the decoded data object.
func (s *RecurringStatus) Raw() map[string]any { return s.raw }

// RecurringRegistrationInfo carries the terms of a registered recurring
// payment. Amounts are in minor units (grosz); terms not given at registration
// are empty or nil.
type RecurringRegistrationInfo struct {
	raw map[string]any
}

// TransactionID returns the transactionId of the registering payment.
func (i *RecurringRegistrationInfo) TransactionID() string {
	return strictString(i.raw, "transaction_id")
}

// Label returns the label shown to the customer.
func (i *RecurringRegistrationInfo) Label() string { return strictString(i.raw, "label") }

// Model returns the recurring model, preserved verbatim even when unknown.
func (i *RecurringRegistrationInfo) Model() RecurringModel {
	return RecurringModel(strictString(i.raw, "model"))
}

// Frequency returns the charge frequency, e.g. 1M.
func (i *RecurringRegistrationInfo) Frequency() string { return strictString(i.raw, "frequency") }

// LimitAmt returns the single charge limit in minor units, nil when absent.
func (i *RecurringRegistrationInfo) LimitAmt() *int64 { return i.amount("limit_amt") }

// TotLimitAmt returns the total limit in minor units, nil when absent.
func (i *RecurringRegistrationInfo) TotLimitAmt() *int64 { return i.amount("tot_limit_amt") }

// IsLimitAmtFixed reports whether every charge must equal LimitAmt, nil when absent.
func (i *RecurringRegistrationInfo) IsLimitAmtFixed() *bool {
	if value, ok := i.raw["is_limit_amt_fixed"].(bool); ok {
		return &value
	}
	return nil
}

// InitDate returns the date of the first charge (YYYY-MM-DD).
func (i *RecurringRegistrationInfo) InitDate() string { return strictString(i.raw, "init_date") }

// TermsURL returns the terms the customer accepted.
func (i *RecurringRegistrationInfo) TermsURL() string { return strictString(i.raw, "terms_url") }

// TermsVersion returns the version of the accepted terms.
func (i *RecurringRegistrationInfo) TermsVersion() string {
	return strictString(i.raw, "terms_version")
}

// RegisteredAt returns when the recurring payment was registered (ISO 8601 with offset).
func (i *RecurringRegistrationInfo) RegisteredAt() string {
	return strictString(i.raw, "registered_at")
}

// Raw returns the decoded registration object.
func (i *RecurringRegistrationInfo) Raw() map[string]any { return i.raw }

// amount accepts an integer or a string of digits, like the PHP SDK.
func (i *RecurringRegistrationInfo) amount(key string) *int64 {
	if value, ok := jsonInt(i.raw[key]); ok {
		return &value
	}
	if value, ok := digitsInt(i.raw[key]); ok {
		return &value
	}
	return nil
}

// RecurringRetryResult is the outcome of retrying a declined recurring charge.
// RecurringRetryStatusFailed is a business result (HTTP 200), not an error.
type RecurringRetryResult struct {
	raw   map[string]any
	retry map[string]any
}

func recurringRetryResultFromAPI(data map[string]any) *RecurringRetryResult {
	return &RecurringRetryResult{raw: data, retry: phpObject(data["retry"])}
}

// TransactionID returns the transactionId of the retried charge.
func (r *RecurringRetryResult) TransactionID() string { return stringField(r.raw, "transactionId") }

// Status returns the retry status, preserved verbatim even when unknown.
func (r *RecurringRetryResult) Status() RecurringRetryStatus {
	return RecurringRetryStatus(strictString(r.retry, "status"))
}

// IsPending reports whether the retry went to the bank and waits for its outcome.
func (r *RecurringRetryResult) IsPending() bool { return r.Status() == RecurringRetryStatusPending }

// IsFailed reports whether the bank declined the retry at once.
func (r *RecurringRetryResult) IsFailed() bool { return r.Status() == RecurringRetryStatusFailed }

// Count returns which retry this was (1-3), nil when the API sent none.
func (r *RecurringRetryResult) Count() *int64 { return optionalJSONInt(r.retry, "count") }

// ErrorCode returns the decline code of a failed retry, e.g. INSUFFICIENT_FUNDS.
func (r *RecurringRetryResult) ErrorCode() string { return strictString(r.retry, "error") }

// ErrorDescription returns the provider's description of a failed retry.
func (r *RecurringRetryResult) ErrorDescription() string {
	return strictString(r.retry, "error_description")
}

// Raw returns the decoded data object.
func (r *RecurringRetryResult) Raw() map[string]any { return r.raw }
