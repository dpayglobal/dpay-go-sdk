package dpay_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"

	dpay "github.com/dpayglobal/dpay-go-sdk"
)

func ExampleClient() {
	client, err := dpay.New("my_shop", "secret_hash")
	if err != nil {
		panic(err)
	}

	payment, err := client.Payments.Register(context.Background(), &dpay.RegisterPaymentRequest{
		Amount:          dpay.PLN(1050),
		TransactionType: dpay.TransactionTypeTransfers,
		URLs: dpay.ReturnURLs{
			Success: "https://twojsklep.pl/sukces",
			Fail:    "https://twojsklep.pl/blad",
			IPN:     "https://twojsklep.pl/ipn",
		},
		Description: dpay.String("Zamówienie #1234"),
		Custom:      dpay.String("order-1234"),
	})
	if err != nil {
		panic(err)
	}

	if url := payment.RedirectURL(); url != "" {
		fmt.Println("redirect to", url)
	}
}

func ExampleVerifyIPN() {
	handler := func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "cannot read body", http.StatusBadRequest)
			return
		}

		event, err := dpay.VerifyIPN(body, "secret_hash")
		if err != nil {
			http.Error(w, "invalid signature", http.StatusBadRequest)
			return
		}
		if event.IsTransfer() {
			markOrderAsPaid(event.ID(), event.Amount())
		}
		fmt.Fprint(w, dpay.IPNAck)
	}
	_ = handler
}

func ExampleVerifyWebhook() {
	handler := func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "cannot read body", http.StatusBadRequest)
			return
		}
		// the endpoint secret from the panel; list both secrets during a rotation
		event, err := dpay.VerifyWebhook(body, r.Header, "whsec_...")
		if err != nil {
			http.Error(w, "invalid signature", http.StatusBadRequest)
			return
		}
		if event.Type() == dpay.WebhookEventTypePaymentSucceeded {
			payment := event.Object() // amounts in minor units
			_ = payment["amount"]
		}
		w.WriteHeader(http.StatusOK)
	}
	_ = handler
}

func ExampleRecurringService() {
	client, _ := dpay.New("my_shop", "secret_hash")
	ctx := context.Background()
	urls := dpay.ReturnURLs{Success: "https://twojsklep.pl/sukces", Fail: "https://twojsklep.pl/blad"}

	// registration together with a payment with the customer's BLIK code (0 = consent only)
	_, err := client.Payments.Register(ctx, &dpay.RegisterPaymentRequest{
		Amount: dpay.PLN(0), TransactionType: dpay.TransactionTypeTransfers, URLs: urls,
		BlikCode: dpay.String("777123"), UserAgent: dpay.String("Mozilla/5.0"), UserIP: dpay.String("83.238.17.42"),
		RecurringRegistration: &dpay.RecurringRegistration{
			Label: "Abonament Premium", Model: dpay.RecurringModelOnDemand,
			TermsURL: "https://twojsklep.pl/regulamin", Alias: dpay.String("SUB-1234"),
		},
	})
	if err != nil {
		panic(err)
	}

	// a later charge, sent by your server without a BLIK code
	charge, err := client.Payments.Register(ctx, &dpay.RegisterPaymentRequest{
		Amount: dpay.PLN(4999), TransactionType: dpay.TransactionTypeTransfers, URLs: urls,
		RecurringAlias: dpay.String("SUB-1234"), Description: dpay.String("Abonament Premium 10/2026"),
	})
	if err != nil {
		panic(err)
	}

	status, _ := client.Recurring.Status(ctx, "SUB-1234")
	fmt.Println(status.Status())
	_, _ = client.Recurring.Retry(ctx, charge.TransactionID())
	_, _ = client.Recurring.Cancel(ctx, "SUB-1234", dpay.WithCancelReason("Rezygnacja klienta"))
}

func ExampleEventService_Iterate() {
	client, _ := dpay.New("my_shop", "secret_hash")
	events := client.Events.Iterate(context.Background(), &dpay.EventListParams{
		Types: []dpay.WebhookEventType{dpay.WebhookEventTypePaymentSucceeded},
	})
	for events.Next() {
		fmt.Println(events.Event().ID())
	}
	if err := events.Err(); err != nil {
		fmt.Println(err)
	}
}

func ExampleAPIError() {
	client, _ := dpay.New("my_shop", "secret_hash")
	_, err := client.Payments.Details(context.Background(), "tx-1")

	switch {
	case errors.Is(err, dpay.ErrNotFound):
		fmt.Println("no such transaction")
	case errors.Is(err, dpay.ErrTransport):
		fmt.Println("network failure, payment status unknown")
	case err != nil:
		var apiErr *dpay.APIError
		if errors.As(err, &apiErr) {
			fmt.Println(apiErr.HTTPStatus, apiErr.FieldErrors)
		}
	}
}

func markOrderAsPaid(id, amount string) {}
