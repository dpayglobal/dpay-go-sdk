package dpay

import (
	"errors"
	"strconv"
)

// Sentinel errors for classifying failures with errors.Is.
var (
	ErrAPI             = errors.New("dpay: API error")
	ErrAuthentication  = errors.New("dpay: authentication failed")
	ErrInvalidRequest  = errors.New("dpay: invalid request")
	ErrAccessDenied    = errors.New("dpay: access denied")
	ErrNotFound        = errors.New("dpay: not found")
	ErrRateLimit       = errors.New("dpay: rate limited")
	ErrServer          = errors.New("dpay: server error")
	ErrPaymentRejected = errors.New("dpay: payment rejected")
	ErrCardPayment     = errors.New("dpay: card payment failed")
	ErrTransport       = errors.New("dpay: transport failure")
	ErrSignature       = errors.New("dpay: signature verification failed")
	ErrCardEncryption  = errors.New("dpay: card encryption failed")
	ErrInvalidArgument = errors.New("dpay: invalid argument")
)

// APIError is returned whenever the dpay API reports a failure. Use errors.Is
// with one of the sentinels to classify it and errors.As to read its fields.
type APIError struct {
	kind    error
	message string

	// HTTPStatus is the response status. Rejections reported with HTTP 200 keep 200 here.
	HTTPStatus int
	// ErrorCode is the API error code (the code field, e.g. CHECKSUM_REQUIRED or
	// WEBHOOK_URL_INVALID, else errorcode), empty when the response carries none.
	ErrorCode string
	// Reason is the detailed reason next to the code, e.g. https_required for
	// WEBHOOK_URL_INVALID, empty when the response carries none.
	Reason string
	// FieldErrors maps a field name to its validation messages.
	FieldErrors map[string][]string
	// RawBody is the unparsed response body.
	RawBody string
}

// Error implements error.
func (e *APIError) Error() string {
	return "dpay: " + e.message + " (HTTP " + strconv.Itoa(e.HTTPStatus) + ")"
}

// Is reports whether the error matches ErrAPI or its specific sentinel.
func (e *APIError) Is(target error) bool {
	return target == ErrAPI || target == e.kind
}

// RateLimitError is returned for HTTP 429 and carries the rate limit headers.
type RateLimitError struct {
	*APIError

	// RetryAfter is the Retry-After header in seconds, nil when absent.
	RetryAfter *int
	// Limit is the X-RateLimit-Limit header, nil when absent.
	Limit *int
	// Remaining is the X-RateLimit-Remaining header, nil when absent.
	Remaining *int
}

// Unwrap exposes the embedded APIError to errors.As.
func (e *RateLimitError) Unwrap() error { return e.APIError }

// PaymentRejectedError is returned when payment registration is rejected with HTTP 200.
type PaymentRejectedError struct {
	*APIError

	// TransactionID is the identifier the API assigned before rejecting, may be empty.
	TransactionID string
	// ErrorDescription is the provider's description of the decline, empty when it sent none.
	ErrorDescription string
}

// Unwrap exposes the embedded APIError to errors.As.
func (e *PaymentRejectedError) Unwrap() error { return e.APIError }

// CardPaymentError is returned when a card operation fails with HTTP 200 and success != true.
type CardPaymentError struct {
	*APIError
}

// Unwrap exposes the embedded APIError to errors.As.
func (e *CardPaymentError) Unwrap() error { return e.APIError }

// TransportError is returned when the request never produced an API response:
// network, DNS, TLS or context failures. The payment status is unknown.
type TransportError struct {
	message string
	cause   error
}

func newTransportError(message string, cause error) *TransportError {
	return &TransportError{message: message, cause: cause}
}

// Error implements error.
func (e *TransportError) Error() string {
	if e.cause == nil {
		return "dpay: " + e.message
	}
	return "dpay: " + e.message + ": " + e.cause.Error()
}

// Unwrap returns the underlying cause.
func (e *TransportError) Unwrap() error { return e.cause }

// Is reports whether the error matches ErrTransport.
func (e *TransportError) Is(target error) bool { return target == ErrTransport }

// SignatureError is returned when an IPN or webhook payload is malformed, its
// signature does not match or its timestamp is outside the tolerance.
type SignatureError struct {
	message string
}

// Error implements error.
func (e *SignatureError) Error() string { return "dpay: " + e.message }

// Is reports whether the error matches ErrSignature.
func (e *SignatureError) Is(target error) bool { return target == ErrSignature }

// CardEncryptionError is returned when card data cannot be encrypted.
type CardEncryptionError struct {
	message string
	cause   error
}

// Error implements error.
func (e *CardEncryptionError) Error() string { return "dpay: " + e.message }

// Unwrap returns the underlying cause, if any.
func (e *CardEncryptionError) Unwrap() error { return e.cause }

// Is reports whether the error matches ErrCardEncryption.
func (e *CardEncryptionError) Is(target error) bool { return target == ErrCardEncryption }

// ValidationError is returned when an argument or request field is invalid.
// Its message is identical to the one the PHP SDK raises.
type ValidationError struct {
	message string
}

func newValidationError(message string) *ValidationError {
	return &ValidationError{message: message}
}

// Error implements error.
func (e *ValidationError) Error() string { return "dpay: " + e.message }

// Is reports whether the error matches ErrInvalidArgument.
func (e *ValidationError) Is(target error) bool { return target == ErrInvalidArgument }
