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
*/
package dpay
