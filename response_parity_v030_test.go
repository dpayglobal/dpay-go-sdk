package dpay

import (
	"testing"
)

// The 0.3.0 entries of testdata/golden_responses.json: what the getters of the
// PHP SDK 0.3.0 return for the matching entries of response_fixtures.json
// (Money as {minor, currency}, null for an absent value).

func compareMoney(t *testing.T, name string, got Money, want any) int {
	t.Helper()
	wantMoney, ok := want.(map[string]any)
	if !ok {
		t.Errorf("%s = %v, want %v", name, got, want)
		return 0
	}
	return compare(t, name+".minor", got.Minor(), wantMoney["minor"]) +
		compare(t, name+".currency", string(got.Currency()), wantMoney["currency"])
}

func compareOptionalMoney(t *testing.T, name string, got Money, present bool, want any) int {
	t.Helper()
	if want == nil {
		if present {
			t.Errorf("%s = %v, want absent", name, got)
			return 0
		}
		return 1
	}
	if !present {
		t.Errorf("%s absent, want %v", name, want)
		return 0
	}
	return compareMoney(t, name, got, want)
}

func compareMap(t *testing.T, name string, got map[string]any, want any) int {
	t.Helper()
	if want == nil {
		if got != nil {
			t.Errorf("%s = %v, want nil", name, got)
			return 0
		}
		return 1
	}
	return compareJSON(t, name, got, want)
}

func compareFailure(t *testing.T, name string, got *Failure, want any) int {
	t.Helper()
	wantFailure, ok := want.(map[string]any)
	if !ok {
		if got != nil {
			t.Errorf("%s present, want nil", name)
			return 0
		}
		return 1
	}
	if got == nil {
		t.Errorf("%s = nil, want %v", name, want)
		return 0
	}
	return compare(t, name+".code", string(got.Code()), wantFailure["code"]) +
		compare(t, name+".message", got.Message(), wantFailure["message"]) +
		compare(t, name+".category", string(got.Category()), wantFailure["category"]) +
		compare(t, name+".retryable", got.IsRetryable(), wantFailure["retryable"]) +
		compare(t, name+".provider_code", got.ProviderCode(), wantFailure["provider_code"])
}

func comparePayoutDetailsV030(t *testing.T, name string, got *PayoutDetails, want map[string]any) int {
	t.Helper()
	checked := compare(t, name+".id", got.ID(), want["id"]) +
		compare(t, name+".state", got.State(), want["state"]) +
		compare(t, name+".currency", string(got.Currency()), want["currency"]) +
		compareMoney(t, name+".net", got.Net(), want["net"]) +
		compareMoney(t, name+".fee", got.Fee(), want["fee"]) +
		compareMoney(t, name+".gross", got.Gross(), want["gross"]) +
		compare(t, name+".export_date", got.ExportDate(), want["export_date"]) +
		compare(t, name+".paysafecard", got.IsPaysafecard(), want["paysafecard"])
	if want["receiver_amount"] != nil {
		amount, present := got.Receiver().Amount()
		checked += compareOptionalMoney(t, name+".receiver_amount", amount, present, want["receiver_amount"])
	}
	return checked
}

func comparePageMeta(t *testing.T, name string, got *PageMeta, want any) int {
	t.Helper()
	wantMeta := want.(map[string]any)
	return compare(t, name+".page", got.Page(), wantMeta["page"]) +
		compare(t, name+".per_page", got.PerPage(), wantMeta["per_page"]) +
		compare(t, name+".total", got.Total(), wantMeta["total"]) +
		compare(t, name+".total_pages", got.TotalPages(), wantMeta["total_pages"]) +
		compare(t, name+".has_more", got.HasMore(), wantMeta["has_more"])
}

func TestResponseParityWithPHPSDK030(t *testing.T) {
	fixtures := loadJSONObject(t, "testdata/response_fixtures.json")
	golden := loadJSONObject(t, "testdata/golden_responses.json")
	checked := 0

	for _, key := range []string{"transaction_fees", "transaction_fees_strings"} {
		transaction := transactionFromAPI(fixtures[key].(map[string]any))
		want := golden[key].(map[string]any)
		checked += compare(t, key+".rate", transaction.Rate(), want["rate"])
		checked += comparePtrInt(t, key+".minimal_fee", transaction.MinimalFee(), want["minimal_fee"])
		checked += comparePtrInt(t, key+".permanent_fee", transaction.PermanentFee(), want["permanent_fee"])
		wantURLs := want["urls"].(map[string]any)
		urls := transaction.URLs()
		checked += compare(t, key+".urls.success", urls.Success(), wantURLs["success"])
		checked += compare(t, key+".urls.fail", urls.Fail(), wantURLs["fail"])
		checked += compare(t, key+".urls.ipn", urls.IPN(), wantURLs["ipn"])
	}

	for key, currency := range map[string]Currency{"registered_multibanco": CurrencyEUR, "registered_multibanco_sparse": CurrencyPLN, "registered_paid_transaction": CurrencyEUR} {
		payment := registeredPaymentFromAPI(fixtures[key].(map[string]any), currency)
		want := golden[key].(map[string]any)
		if wantMultibanco, ok := want["multibanco"].(map[string]any); ok {
			multibanco := payment.Multibanco()
			amount, present := multibanco.Amount()
			checked += compare(t, key+".entity", multibanco.Entity(), wantMultibanco["entity"])
			checked += compare(t, key+".reference", multibanco.Reference(), wantMultibanco["reference"])
			checked += compareOptionalMoney(t, key+".amount", amount, present, wantMultibanco["amount"])
			checked += compare(t, key+".currency", string(multibanco.Currency()), wantMultibanco["currency"])
			checked += compare(t, key+".expires_at", multibanco.ExpiresAt(), wantMultibanco["expires_at"])
		} else if payment.Multibanco() != nil {
			t.Errorf("%s.multibanco present, want nil", key)
		}
		if wantTransaction, ok := want["paid_transaction"].(map[string]any); ok {
			transaction := payment.PaidTransaction()
			value, present := transaction.Value()
			checked += compare(t, key+".payment_id", transaction.PaymentID(), wantTransaction["payment_id"])
			checked += compare(t, key+".status", string(transaction.Status()), wantTransaction["status"])
			checked += compare(t, key+".is_paid", transaction.IsPaid(), wantTransaction["is_paid"])
			checked += compareOptionalMoney(t, key+".value", value, present, wantTransaction["value"])
			checked += compare(t, key+".email", transaction.Email(), wantTransaction["email"])
			checked += compare(t, key+".description", transaction.Description(), wantTransaction["description"])
			checked += compare(t, key+".name", transaction.Name(), wantTransaction["name"])
			checked += compare(t, key+".surname", transaction.Surname(), wantTransaction["surname"])
			checked += compare(t, key+".pid", transaction.PID(), wantTransaction["pid"])
			checked += compare(t, key+".mtid", transaction.MTID(), wantTransaction["mtid"])
			checked += compare(t, key+".creation_date", transaction.CreationDate(), wantTransaction["creation_date"])
			checked += compare(t, key+".payment_date", transaction.PaymentDate(), wantTransaction["payment_date"])
		} else if payment.PaidTransaction() != nil {
			t.Errorf("%s.paid_transaction present, want nil", key)
		}
	}

	page := payoutPageFromAPI(fixtures["payout_list"].(map[string]any))
	wantPage := golden["payout_list"].(map[string]any)
	checked += compare(t, "payout_list.has_more", page.HasMore(), wantPage["has_more"])
	checked += comparePageMeta(t, "payout_list.meta", page.Meta(), wantPage["meta"])
	wantItems := toSlice(wantPage["items"])
	checked += compare(t, "payout_list.items_len", int64(len(page.Items())), float64(len(wantItems)))
	for index, entry := range wantItems {
		want, item := entry.(map[string]any), page.Items()[index]
		name := "payout_list.item"
		checked += compare(t, name+".id", item.ID(), want["id"])
		checked += compare(t, name+".type", string(item.Type()), want["type"])
		checked += compare(t, name+".state", item.State(), want["state"])
		checked += compare(t, name+".is_waiting", item.IsWaiting(), want["is_waiting"])
		checked += compare(t, name+".is_processed", item.IsProcessed(), want["is_processed"])
		checked += compare(t, name+".is_failed", item.IsFailed(), want["is_failed"])
		checked += compareMoney(t, name+".net", item.Net(), want["net"])
		checked += compareMoney(t, name+".fee", item.Fee(), want["fee"])
		checked += compareMoney(t, name+".gross", item.Gross(), want["gross"])
		checked += compare(t, name+".currency", string(item.Currency()), want["currency"])
		checked += compare(t, name+".creation_date", item.CreationDate(), want["creation_date"])
		checked += compare(t, name+".export_date", item.ExportDate(), want["export_date"])
		checked += compare(t, name+".declined", item.IsDeclined(), want["declined"])
	}

	settlement := payoutSettlementFromAPI(fixtures["payout_settlement"].(map[string]any))
	wantSettlement := golden["payout_settlement"].(map[string]any)
	checked += comparePayoutDetailsV030(t, "payout_settlement.settlement", settlement.Settlement(), wantSettlement["settlement"].(map[string]any))
	checked += compareMoney(t, "payout_settlement.summary_net", settlement.SummaryNet(), wantSettlement["summary_net"])
	checked += compareMoney(t, "payout_settlement.summary_fee", settlement.SummaryFee(), wantSettlement["summary_fee"])
	checked += compareMoney(t, "payout_settlement.summary_gross", settlement.SummaryGross(), wantSettlement["summary_gross"])
	checked += compare(t, "payout_settlement.summary_count", settlement.SummaryCount(), wantSettlement["summary_count"])
	checked += comparePageMeta(t, "payout_settlement.meta", settlement.Meta(), wantSettlement["meta"])
	wantPositions := toSlice(wantSettlement["positions"])
	checked += compare(t, "payout_settlement.positions_len", int64(len(settlement.Positions())), float64(len(wantPositions)))
	for index, entry := range wantPositions {
		want, position := entry.(map[string]any), settlement.Positions()[index]
		name := "payout_settlement.position"
		checked += compare(t, name+".payment_id", position.PaymentID(), want["payment_id"])
		checked += compare(t, name+".date", position.Date(), want["date"])
		checked += compareMoney(t, name+".net", position.Net(), want["net"])
		checked += compareMoney(t, name+".fee", position.Fee(), want["fee"])
		checked += compareMoney(t, name+".gross", position.Gross(), want["gross"])
		checked += comparePtrInt(t, name+".service_id", position.ServiceID(), want["service_id"])
		checked += compare(t, name+".service_name", position.ServiceName(), want["service_name"])
		checked += compare(t, name+".custom", position.Custom(), want["custom"])
		checked += compare(t, name+".ipksef", position.IPKSeF(), want["ipksef"])
	}

	checked += compareTypedWebhookParity(t, fixtures, golden)

	session := cardPaymentResultFromAPI(fixtures["card_apple_session"].(map[string]any)).ApplePayMerchantSession()
	checked += compareMap(t, "card_apple_session.session", session, golden["card_apple_session"].(map[string]any)["session"])
	for _, key := range []string{"card_declined", "card_declined_plain"} {
		cardErr := cardPaymentErrorFromAPI(fixtures[key].(map[string]any))
		want := golden[key].(map[string]any)
		checked += compare(t, key+".error_code", cardErr.ErrorCode, want["error_code"])
		wantCardError, ok := want["card_error"].(map[string]any)
		if !ok {
			if cardErr.CardError != nil {
				t.Errorf("%s.card_error present, want nil", key)
			}
			continue
		}
		checked += compare(t, key+".code", cardErr.CardError.Code, wantCardError["code"])
		checked += compare(t, key+".decline_code", cardErr.CardError.DeclineCode, wantCardError["decline_code"])
		checked += compare(t, key+".decline_message", cardErr.CardError.DeclineMessage, wantCardError["decline_message"])
	}

	t.Logf("matching 0.3.0 response values: %d", checked)
}

func compareTypedWebhookParity(t *testing.T, fixtures, golden map[string]any) int {
	t.Helper()
	checked := 0
	for _, key := range []string{"webhook_payment", "webhook_payment_failed", "webhook_refund", "webhook_recurring", "webhook_payout", "webhook_fallback", "webhook_event"} {
		event, _ := webhookEventFromJSON(fixtures[key])
		want := golden[key+"_typed"].(map[string]any)
		for _, kind := range []struct {
			name    string
			present bool
		}{{"payment", event.Payment() != nil}, {"refund", event.Refund() != nil}, {"recurring_payment", event.RecurringPayment() != nil}, {"payout", event.Payout() != nil}} {
			checked += compare(t, key+"."+kind.name+".present", kind.present, want[kind.name] != nil)
		}
		if wantPayment, ok := want["payment"].(map[string]any); ok {
			payment, name := event.Payment(), key+".payment"
			checked += compare(t, name+".id", payment.ID(), wantPayment["id"])
			checked += compare(t, name+".status", string(payment.Status()), wantPayment["status"])
			checked += compare(t, name+".currency", string(payment.Currency()), wantPayment["currency"])
			checked += compareMoney(t, name+".amount", payment.Amount(), wantPayment["amount"])
			checked += compareMoney(t, name+".amount_refunded", payment.AmountRefunded(), wantPayment["amount_refunded"])
			checked += compareMoney(t, name+".amount_captured", payment.AmountCaptured(), wantPayment["amount_captured"])
			checked += compare(t, name+".description", payment.Description(), wantPayment["description"])
			checked += compare(t, name+".created", payment.Created(), wantPayment["created"])
			checked += compare(t, name+".paid_at", payment.PaidAt(), wantPayment["paid_at"])
			checked += compareMap(t, name+".payment_method", payment.PaymentMethod(), wantPayment["payment_method"])
			checked += compare(t, name+".payment_method_type", payment.PaymentMethodType(), wantPayment["payment_method_type"])
			checked += compareFailure(t, name+".failure", payment.Failure(), wantPayment["failure"])
			checked += compare(t, name+".is_recurring", payment.IsRecurring(), wantPayment["is_recurring"])
			checked += compare(t, name+".recurring_alias", payment.RecurringAlias(), wantPayment["recurring_alias"])
			checked += compare(t, name+".recurring_registration", payment.RecurringRegistration(), wantPayment["recurring_registration"])
			checked += compare(t, name+".recurring_sequence", string(payment.RecurringSequence()), wantPayment["recurring_sequence"])
			checked += compare(t, name+".merchant_reference", payment.MerchantReference(), wantPayment["merchant_reference"])
			checked += compare(t, name+".provider_reference", payment.ProviderReference(), wantPayment["provider_reference"])
			checked += compare(t, name+".custom", payment.Custom(), wantPayment["custom"])
		}
		if wantRefund, ok := want["refund"].(map[string]any); ok {
			refund, name := event.Refund(), key+".refund"
			checked += compare(t, name+".id", refund.ID(), wantRefund["id"])
			checked += compare(t, name+".payment", refund.Payment(), wantRefund["payment"])
			checked += compare(t, name+".status", string(refund.Status()), wantRefund["status"])
			checked += compare(t, name+".currency", string(refund.Currency()), wantRefund["currency"])
			checked += compareMoney(t, name+".amount", refund.Amount(), wantRefund["amount"])
			checked += compare(t, name+".created", refund.Created(), wantRefund["created"])
			checked += compare(t, name+".succeeded_at", refund.SucceededAt(), wantRefund["succeeded_at"])
			checked += compareFailure(t, name+".failure", refund.Failure(), wantRefund["failure"])
		}
		if wantRecurring, ok := want["recurring_payment"].(map[string]any); ok {
			recurring, name := event.RecurringPayment(), key+".recurring_payment"
			checked += compare(t, name+".id", recurring.ID(), wantRecurring["id"])
			checked += compare(t, name+".alias", recurring.Alias(), wantRecurring["alias"])
			checked += compare(t, name+".status", string(recurring.Status()), wantRecurring["status"])
			checked += compare(t, name+".canceled_by", string(recurring.CanceledBy()), wantRecurring["canceled_by"])
			checked += compareMap(t, name+".payment_method", recurring.PaymentMethod(), wantRecurring["payment_method"])
			checked += compare(t, name+".payment_method_type", recurring.PaymentMethodType(), wantRecurring["payment_method_type"])
			checked += compare(t, name+".model", recurring.Model(), wantRecurring["model"])
			checked += compare(t, name+".frequency", recurring.Frequency(), wantRecurring["frequency"])
			checked += comparePtrInt(t, name+".limit_amount", recurring.LimitAmount(), wantRecurring["limit_amount"])
			checked += comparePtrInt(t, name+".total_limit_amount", recurring.TotalLimitAmount(), wantRecurring["total_limit_amount"])
			checked += compare(t, name+".starts_on", recurring.StartsOn(), wantRecurring["starts_on"])
			checked += compare(t, name+".expires_on", recurring.ExpiresOn(), wantRecurring["expires_on"])
			checked += compare(t, name+".created", recurring.Created(), wantRecurring["created"])
		}
		if wantPayout, ok := want["payout"].(map[string]any); ok {
			payout, name := event.Payout(), key+".payout"
			checked += compare(t, name+".id", payout.ID(), wantPayout["id"])
			checked += compare(t, name+".status", string(payout.Status()), wantPayout["status"])
			checked += compare(t, name+".currency", string(payout.Currency()), wantPayout["currency"])
			checked += compareMoney(t, name+".amount", payout.Amount(), wantPayout["amount"])
			checked += compareMoney(t, name+".fee", payout.Fee(), wantPayout["fee"])
			checked += compare(t, name+".bank_account_last4", payout.BankAccountLast4(), wantPayout["bank_account_last4"])
			checked += compare(t, name+".created", payout.Created(), wantPayout["created"])
			checked += compare(t, name+".paid_at", payout.PaidAt(), wantPayout["paid_at"])
			checked += compareFailure(t, name+".failure", payout.Failure(), wantPayout["failure"])
		}
	}
	return checked
}
