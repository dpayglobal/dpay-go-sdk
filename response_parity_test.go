package dpay

import (
	"encoding/json"
	"os"
	"reflect"
	"testing"
)

func loadJSONObject(t *testing.T, path string) map[string]any {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("brakuje golden vectors w testdata - sa czescia repo, patrz docs/golden/README.md: %v", err)
	}
	var data map[string]any
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	return data
}

func toSlice(value any) []any {
	if slice, ok := value.([]any); ok {
		return slice
	}
	return nil
}

func zeroFor(got any) any {
	switch got.(type) {
	case string:
		return ""
	case bool:
		return false
	case int64, float64:
		return float64(0)
	}
	return nil
}

func compare(t *testing.T, name string, got any, want any) int {
	t.Helper()
	if want == nil {
		want = zeroFor(got)
	}
	if number, ok := want.(float64); ok {
		switch typed := got.(type) {
		case int64:
			if float64(typed) != number {
				t.Errorf("%s = %v, want %v", name, got, want)
				return 0
			}
			return 1
		case float64:
			if typed != number {
				t.Errorf("%s = %v, want %v", name, got, want)
				return 0
			}
			return 1
		}
	}
	if got != want {
		t.Errorf("%s = %#v, want %#v", name, got, want)
		return 0
	}
	return 1
}

func comparePtrInt(t *testing.T, name string, got *int64, want any) int {
	t.Helper()
	if want == nil {
		if got != nil {
			t.Errorf("%s = %v, want nil", name, *got)
			return 0
		}
		return 1
	}
	if got == nil {
		t.Errorf("%s = nil, want %v", name, want)
		return 0
	}
	return compare(t, name, *got, want)
}

func TestResponseParityWithPHPSDK(t *testing.T) {
	fixtures := loadJSONObject(t, "testdata/response_fixtures.json")
	golden := loadJSONObject(t, "testdata/golden_responses.json")
	checked := 0

	for _, key := range []string{"transaction_full", "transaction_broken", "transaction_empty"} {
		transaction := transactionFromAPI(fixtures[key].(map[string]any))
		want := golden[key].(map[string]any)
		checked += compare(t, key+".id", transaction.ID(), want["id"])
		checked += compare(t, key+".value", transaction.Value().Minor(), want["value"])
		checked += compare(t, key+".status", string(transaction.Status()), want["status"])
		checked += compare(t, key+".is_paid", transaction.IsPaid(), want["is_paid"])
		checked += compare(t, key+".payment_method", transaction.PaymentMethod(), want["payment_method"])
		checked += compare(t, key+".creation_date", transaction.CreationDate(), want["creation_date"])
		checked += compare(t, key+".payment_date", transaction.PaymentDate(), want["payment_date"])
		checked += compare(t, key+".settled", transaction.IsSettled(), want["settled"])
		checked += compare(t, key+".refunded", transaction.IsRefunded(), want["refunded"])
		checked += compare(t, key+".refunded_amount", transaction.RefundedAmount().Minor(), want["refunded_amount"])
		checked += compare(t, key+".available_refund_amount", transaction.AvailableRefundAmount().Minor(), want["available_refund_amount"])
		checked += compare(t, key+".fully_refunded", transaction.IsFullyRefunded(), want["fully_refunded"])
		checked += compare(t, key+".direct", transaction.IsDirect(), want["direct"])
		checked += compare(t, key+".gateway_id", transaction.GatewayID(), want["gateway_id"])
		wantRefunds := toSlice(want["refunds"])
		checked += compare(t, key+".refunds_len", int64(len(transaction.Refunds())), float64(len(wantRefunds)))
		for index, entry := range wantRefunds {
			wantRefund := entry.(map[string]any)
			gotRefund := transaction.Refunds()[index]
			checked += compare(t, key+".refund.payment_id", gotRefund.PaymentID(), wantRefund["payment_id"])
			checked += compare(t, key+".refund.value", gotRefund.Value().Minor(), wantRefund["value"])
			checked += compare(t, key+".refund.status", string(gotRefund.Status()), wantRefund["status"])
			checked += compare(t, key+".refund.creation_date", gotRefund.CreationDate(), wantRefund["creation_date"])
		}
	}

	for _, key := range []string{"registered_redirect", "registered_paid", "registered_inline", "registered_recurring"} {
		payment := registeredPaymentFromAPI(fixtures[key].(map[string]any))
		want := golden[key].(map[string]any)
		checked += compare(t, key+".transaction_id", payment.TransactionID(), want["transaction_id"])
		checked += compare(t, key+".message", payment.Message(), want["message"])
		checked += compare(t, key+".redirect_url", payment.RedirectURL(), want["redirect_url"])
		checked += compare(t, key+".is_paid", payment.IsPaid(), want["is_paid"])
		checked += compare(t, key+".is_inline", payment.IsInlineProcessing(), want["is_inline"])
		checked += compare(t, key+".ipksef", payment.IPKSeF(), want["ipksef"])
		checked += compare(t, key+".card_recurring_alias", payment.CardRecurringAlias(), want["card_recurring_alias"])
		// the 0.1.x entries predate these getters: absent means null and [] in PHP
		checked += compare(t, key+".recurring_alias", payment.RecurringAlias(), want["recurring_alias"])
		checked += compareStrings(t, key+".recurring_methods", payment.RecurringMethods(), want["recurring_methods"])
	}

	for _, key := range []string{"refund_accepted", "refund_rejected"} {
		refund := refundFromAPI(fixtures[key].(map[string]any))
		want := golden[key].(map[string]any)
		checked += compare(t, key+".accepted", refund.IsAccepted(), want["accepted"])
		checked += compare(t, key+".message", refund.Message(), want["message"])
	}

	availability := refundAvailabilityFromAPI(fixtures["availability"].(map[string]any), 409)
	wantAvailability := golden["availability"].(map[string]any)
	checked += compare(t, "availability.available", availability.IsAvailable(), wantAvailability["available"])
	checked += compare(t, "availability.message", availability.Message(), wantAvailability["message"])
	checked += compare(t, "availability.http_status", int64(availability.HTTPStatus()), wantAvailability["http_status"])

	for _, key := range []string{"bank_full", "bank_sparse"} {
		bank := bankFromAPI(fixtures[key].(map[string]any))
		want := golden[key].(map[string]any)
		checked += compare(t, key+".id", bank.ID(), want["id"])
		checked += compare(t, key+".name", bank.Name(), want["name"])
		checked += compare(t, key+".image", bank.Image(), want["image"])
		checked += compare(t, key+".on_from", bank.OnFrom(), want["on_from"])
		checked += compare(t, key+".on_to", bank.OnTo(), want["on_to"])
		checked += compare(t, key+".test", bank.IsTest(), want["test"])
		checked += compare(t, key+".type", bank.Type(), want["type"])
		checked += comparePtrInt(t, key+".iterator", bank.Iterator(), want["iterator"])
	}

	for _, key := range []string{"payout_full", "payout_failed"} {
		payout := payoutDetailsFromAPI(fixtures[key].(map[string]any))
		want := golden[key].(map[string]any)
		checked += compare(t, key+".id", payout.ID(), want["id"])
		checked += compare(t, key+".state", payout.State(), want["state"])
		checked += compare(t, key+".is_waiting", payout.IsWaiting(), want["is_waiting"])
		checked += compare(t, key+".is_processed", payout.IsProcessed(), want["is_processed"])
		checked += compare(t, key+".is_failed", payout.IsFailed(), want["is_failed"])
		checked += compare(t, key+".net", payout.Net().Minor(), want["net"])
		checked += compare(t, key+".fee", payout.Fee().Minor(), want["fee"])
		checked += compare(t, key+".gross", payout.Gross().Minor(), want["gross"])
		checked += compare(t, key+".creation_date", payout.CreationDate(), want["creation_date"])
		checked += compare(t, key+".direct_settlement", payout.IsDirectSettlement(), want["direct_settlement"])
		checked += compare(t, key+".nrb", payout.NRB(), want["nrb"])
		checked += compare(t, key+".declined", payout.IsDeclined(), want["declined"])
		checked += compare(t, key+".decline_reason", payout.DeclineReason(), want["decline_reason"])
		checked += compare(t, key+".decline_status", payout.DeclineStatus(), want["decline_status"])

		if wantReceiver, ok := want["receiver"].(map[string]any); ok {
			receiver := payout.Receiver()
			if receiver == nil {
				t.Errorf("%s.receiver = nil, want an object", key)
				continue
			}
			checked += compare(t, key+".receiver.nrb", receiver.NRB(), wantReceiver["nrb"])
			checked += compare(t, key+".receiver.title", receiver.Title(), wantReceiver["title"])
			checked += compare(t, key+".receiver.service", receiver.Service(), wantReceiver["service"])
			checked += compare(t, key+".receiver.name", receiver.ReceiverName(), wantReceiver["receiver_name"])
			checked += compare(t, key+".receiver.address", receiver.ReceiverAddress(), wantReceiver["receiver_address"])
			amount, present := receiver.Amount()
			if wantReceiver["amount"] == nil {
				if present {
					t.Errorf("%s.receiver.amount present, want absent", key)
				}
			} else {
				checked += compare(t, key+".receiver.amount", amount.Minor(), wantReceiver["amount"])
			}
		} else if payout.Receiver() != nil {
			t.Errorf("%s.receiver present, want nil", key)
		}
	}

	alias := blikAliasFromAPI(fixtures["blik_alias"].(map[string]any))
	wantAlias := golden["blik_alias"].(map[string]any)
	checked += compare(t, "blik_alias.value", alias.Value(), wantAlias["value"])
	checked += compare(t, "blik_alias.type", string(alias.Type()), wantAlias["type"])
	checked += compare(t, "blik_alias.status", alias.Status(), wantAlias["status"])
	checked += compare(t, "blik_alias.is_active", alias.IsActive(), wantAlias["is_active"])
	checked += compare(t, "blik_alias.expiration_date", alias.ExpirationDate(), wantAlias["expiration_date"])
	wantApps := toSlice(wantAlias["apps"])
	checked += compare(t, "blik_alias.apps_len", int64(len(alias.Apps())), float64(len(wantApps)))
	for index, entry := range wantApps {
		wantApp := entry.(map[string]any)
		checked += compare(t, "blik_alias.app.key", alias.Apps()[index].Key(), wantApp["key"])
		checked += compare(t, "blik_alias.app.label", alias.Apps()[index].Label(), wantApp["label"])
	}

	checked += compareRecurringParity(t, fixtures, golden)
	checked += compareWebhookEventParity(t, fixtures, golden)

	for _, key := range []string{"card_form", "card_url", "card_bad_base64", "card_dcc"} {
		result := cardPaymentResultFromAPI(fixtures[key].(map[string]any))
		want := golden[key].(map[string]any)
		checked += compare(t, key+".redirect_type", string(result.RedirectType()), want["redirect_type"])
		checked += compare(t, key+".is_success", result.IsSuccess(), want["is_success"])
		checked += compare(t, key+".requires_form", result.RequiresThreeDSForm(), want["requires_form"])
		checked += compare(t, key+".requires_redirect", result.RequiresRedirect(), want["requires_redirect"])
		checked += compare(t, key+".has_dcc_offer", result.HasDCCOffer(), want["has_dcc_offer"])
		checked += compare(t, key+".form_html", result.ThreeDSFormHTML(), want["form_html"])
		checked += compare(t, key+".redirect_url", result.RedirectURL(), want["redirect_url"])

		if wantOffer, ok := want["dcc_offer"].(map[string]any); ok {
			offer := result.DCCOffer()
			if offer == nil {
				t.Errorf("%s.dcc_offer = nil, want an object", key)
				continue
			}
			checked += compare(t, key+".dcc.id", offer.CurrencyConversionID(), wantOffer["currency_conversion_id"])
			checked += compare(t, key+".dcc.original", offer.OriginalAmount().Minor(), wantOffer["original_amount"])
			checked += compare(t, key+".dcc.original_currency", string(offer.OriginalAmount().Currency()), wantOffer["original_currency"])
			checked += compare(t, key+".dcc.converted", offer.ConvertedAmount().Minor(), wantOffer["converted_amount"])
			checked += compare(t, key+".dcc.converted_currency", string(offer.ConvertedAmount().Currency()), wantOffer["converted_currency"])
			checked += compare(t, key+".dcc.rate", offer.ExchangeRate(), wantOffer["exchange_rate"])
			checked += compare(t, key+".dcc.valid_until", offer.ValidUntil(), wantOffer["valid_until"])
			checked += compare(t, key+".dcc.declaration", offer.DeclarationText(), wantOffer["declaration_text"])
			checked += compare(t, key+".dcc.eea", offer.IsEuropeanEconomicArea(), wantOffer["eea"])
			for index, entry := range toSlice(wantOffer["markup"]) {
				wantMarkup := entry.(map[string]any)
				checked += compare(t, key+".dcc.markup.rate", offer.Markup()[index].Rate(), wantMarkup["rate"])
				checked += compare(t, key+".dcc.markup.info", offer.Markup()[index].AdditionalInfo(), wantMarkup["additional_info"])
			}
		} else if result.DCCOffer() != nil {
			t.Errorf("%s.dcc_offer present, want nil", key)
		}
	}

	t.Logf("matching response values: %d", checked)
}

func comparePtrBool(t *testing.T, name string, got *bool, want any) int {
	t.Helper()
	if want == nil {
		if got != nil {
			t.Errorf("%s = %v, want nil", name, *got)
			return 0
		}
		return 1
	}
	if got == nil {
		t.Errorf("%s = nil, want %v", name, want)
		return 0
	}
	return compare(t, name, *got, want)
}

func compareStrings(t *testing.T, name string, got []string, want any) int {
	t.Helper()
	wantList := toSlice(want)
	if len(got) != len(wantList) {
		t.Errorf("%s = %v, want %v", name, got, want)
		return 0
	}
	for index, value := range wantList {
		if got[index] != value {
			t.Errorf("%s[%d] = %q, want %v", name, index, got[index], value)
			return 0
		}
	}
	return 1
}

// compareJSON compares decoded JSON values; PHP encodes an empty array as [],
// which stands for the empty object on the Go side.
func compareJSON(t *testing.T, name string, got map[string]any, want any) int {
	t.Helper()
	if list, ok := want.([]any); ok && len(list) == 0 {
		want = map[string]any{}
	}
	if !reflect.DeepEqual(any(got), want) {
		t.Errorf("%s = %#v, want %#v", name, got, want)
		return 0
	}
	return 1
}

func compareRecurringParity(t *testing.T, fixtures, golden map[string]any) int {
	t.Helper()
	checked := 0
	for _, key := range []string{"recurring_status", "recurring_status_sparse"} {
		status := recurringStatusFromAPI(fixtures[key].(map[string]any))
		want := golden[key].(map[string]any)
		checked += compare(t, key+".alias", status.Alias(), want["alias"])
		checked += compare(t, key+".method", status.Method(), want["method"])
		checked += compare(t, key+".status", string(status.Status()), want["status"])
		checked += compare(t, key+".is_active", status.IsActive(), want["is_active"])
		checked += compare(t, key+".expiration_date", status.ExpirationDate(), want["expiration_date"])

		wantRegistration, ok := want["registration"].(map[string]any)
		registration := status.Registration()
		if !ok {
			if registration != nil {
				t.Errorf("%s.registration present, want nil", key)
			}
			continue
		}
		if registration == nil {
			t.Errorf("%s.registration = nil, want an object", key)
			continue
		}
		checked += compare(t, key+".transaction_id", registration.TransactionID(), wantRegistration["transaction_id"])
		checked += compare(t, key+".label", registration.Label(), wantRegistration["label"])
		checked += compare(t, key+".model", string(registration.Model()), wantRegistration["model"])
		checked += compare(t, key+".frequency", registration.Frequency(), wantRegistration["frequency"])
		checked += comparePtrInt(t, key+".limit_amt", registration.LimitAmt(), wantRegistration["limit_amt"])
		checked += comparePtrInt(t, key+".tot_limit_amt", registration.TotLimitAmt(), wantRegistration["tot_limit_amt"])
		checked += comparePtrBool(t, key+".is_limit_amt_fixed", registration.IsLimitAmtFixed(), wantRegistration["is_limit_amt_fixed"])
		checked += compare(t, key+".init_date", registration.InitDate(), wantRegistration["init_date"])
		checked += compare(t, key+".terms_url", registration.TermsURL(), wantRegistration["terms_url"])
		checked += compare(t, key+".terms_version", registration.TermsVersion(), wantRegistration["terms_version"])
		checked += compare(t, key+".registered_at", registration.RegisteredAt(), wantRegistration["registered_at"])
	}

	for _, key := range []string{"recurring_retry_pending", "recurring_retry_failed"} {
		retry := recurringRetryResultFromAPI(fixtures[key].(map[string]any))
		want := golden[key].(map[string]any)
		checked += compare(t, key+".transaction_id", retry.TransactionID(), want["transaction_id"])
		checked += compare(t, key+".status", string(retry.Status()), want["status"])
		checked += compare(t, key+".is_pending", retry.IsPending(), want["is_pending"])
		checked += compare(t, key+".is_failed", retry.IsFailed(), want["is_failed"])
		checked += comparePtrInt(t, key+".count", retry.Count(), want["count"])
		checked += compare(t, key+".error_code", retry.ErrorCode(), want["error_code"])
		checked += compare(t, key+".error_description", retry.ErrorDescription(), want["error_description"])
	}
	return checked
}

func compareWebhookEventParity(t *testing.T, fixtures, golden map[string]any) int {
	t.Helper()
	checked := 0
	for _, key := range []string{"webhook_event", "webhook_event_sparse"} {
		event, ok := webhookEventFromJSON(fixtures[key])
		if !ok {
			t.Errorf("%s is not an event", key)
			continue
		}
		want := golden[key].(map[string]any)
		checked += compare(t, key+".id", event.ID(), want["id"])
		checked += compare(t, key+".type", string(event.Type()), want["type"])
		checked += compare(t, key+".api_version", event.APIVersion(), want["api_version"])
		checked += compare(t, key+".created", event.Created(), want["created"])
		checked += compare(t, key+".livemode", event.IsLivemode(), want["livemode"])
		checked += compare(t, key+".service", event.Service(), want["service"])
		checked += compare(t, key+".merchant_ref", event.MerchantRef(), want["merchant_ref"])
		checked += compare(t, key+".object_type", event.ObjectType(), want["object_type"])
		checked += compareJSON(t, key+".object", event.Object(), want["object"])
	}
	return checked
}
