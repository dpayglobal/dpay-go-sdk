/*
Package dpay is the official Go SDK for the dpay.pl payments API.

Create a client with the payment point name and secret hash from panel.dpay.pl,
then use one of its eight services:

	client, err := dpay.New("my_shop", "secret_hash")
	payment, err := client.Payments.Register(ctx, request)

Verify webhooks with VerifyWebhook (or a configured WebhookVerifier) on the raw
request body, and IPN notifications with VerifyIPN.

Optional request fields are pointers: nil omits the field, while dpay.Bool(false)
sends false explicitly. Use dpay.String, dpay.Bool, dpay.Int and dpay.Int64 to
build them.

Errors are classified with errors.Is against the package sentinels and inspected
with errors.As on *APIError. A *TransportError means the request never reached
the API, so the payment status is unknown.

The order of fields in a request body is part of the protocol: the checksum is
computed from the values in the order they are sent.

The panel API (Refunds, Payments.Details, Banks.ForService, Payouts) trims
every request string before it verifies the checksum - ASCII whitespace and NUL,
but also Unicode spaces (e.g. U+00A0, U+3000) and invisible characters (e.g.
U+200B, U+FEFF). The SDK sends and signs those values (the service name too)
already trimmed the same way, so " 2026-08-01\n" reaches the API as
"2026-08-01". A transaction ID, service name or date with nothing left is an
ErrInvalidArgument ("transaction_id must not be empty"); a refund reason with
nothing left is sent empty. Payments API requests are not trimmed.
*/
package dpay
