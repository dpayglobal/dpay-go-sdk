package dpay_test

import (
	"context"
	"errors"
	"fmt"
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
		body := make([]byte, r.ContentLength)
		r.Body.Read(body)

		event, err := dpay.VerifyIPN(body, "secret_hash")
		if err != nil {
			http.Error(w, "invalid signature", http.StatusBadRequest)
			return
		}
		if event.IsTransfer() || event.IsCapture() {
			markOrderAsPaid(event.ID(), event.Amount())
		}
		fmt.Fprint(w, dpay.IPNAck)
	}
	_ = handler
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
