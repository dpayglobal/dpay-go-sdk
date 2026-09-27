package dpay

// TransactionType selects the payment flow a registration starts.
type TransactionType string

// Transaction types accepted by payments/register. Recurring payments use
// TransactionTypeTransfers with RegisterPaymentRequest.RecurringRegistration or
// RecurringAlias; the API rejects blik_recurring and bizum_direct.
const (
	TransactionTypeTransfers     TransactionType = "transfers"
	TransactionTypeDCBGateway    TransactionType = "dcb_gateway"
	TransactionTypeCardAuth      TransactionType = "card_auth"
	TransactionTypeMBWayDirect   TransactionType = "mb_way_direct"
	TransactionTypeCardRecurring TransactionType = "card_recurring"
)

// Valid reports whether the value is one of the known transaction types.
func (t TransactionType) Valid() bool {
	switch t {
	case TransactionTypeTransfers, TransactionTypeDCBGateway, TransactionTypeCardAuth,
		TransactionTypeMBWayDirect, TransactionTypeCardRecurring:
		return true
	}
	return false
}

// TransactionStatus is the lifecycle state of a transaction.
type TransactionStatus string

// Transaction statuses reported by pbl/details.
const (
	TransactionStatusPaid       TransactionStatus = "paid"
	TransactionStatusCreated    TransactionStatus = "created"
	TransactionStatusProcessing TransactionStatus = "processing"
	TransactionStatusExpired    TransactionStatus = "expired"
	TransactionStatusCaptured   TransactionStatus = "captured"
)

// Valid reports whether the value is one of the known transaction statuses.
func (s TransactionStatus) Valid() bool {
	switch s {
	case TransactionStatusPaid, TransactionStatusCreated, TransactionStatusProcessing,
		TransactionStatusExpired, TransactionStatusCaptured:
		return true
	}
	return false
}

// BlikAliasType is the type of a BLIK OneClick alias. Recurring payments (PAYID
// aliases) are handled by Client.Recurring.
type BlikAliasType string

// BLIK alias types.
const (
	BlikAliasTypeUID BlikAliasType = "UID"
)

// Valid reports whether the value is one of the known alias types.
func (t BlikAliasType) Valid() bool {
	return t == BlikAliasTypeUID
}

// RecurringModel decides who approves the charges of a recurring payment.
type RecurringModel string

// Recurring payment models.
const (
	// RecurringModelAutomatic (A): fixed amount, frequency, total limit, start and
	// expiry date, all required; the customer's bank approves matching charges.
	RecurringModelAutomatic RecurringModel = "A"
	// RecurringModelManual (M): the customer confirms every charge in the banking
	// app; frequency and limits are optional.
	RecurringModelManual RecurringModel = "M"
	// RecurringModelOnDemand (O): no frequency or limits, the merchant charges any
	// amount within the active ranges of the service (at most 2000 PLN).
	RecurringModelOnDemand RecurringModel = "O"
)

// Valid reports whether the value is one of the known recurring models.
func (m RecurringModel) Valid() bool {
	return m == RecurringModelAutomatic || m == RecurringModelManual || m == RecurringModelOnDemand
}

// RecurringMethod is a payment method of a recurring payment.
type RecurringMethod string

// Recurring payment methods.
const (
	RecurringMethodBlik RecurringMethod = "blik"
)

// Valid reports whether the value is one of the known recurring methods.
func (m RecurringMethod) Valid() bool {
	return m == RecurringMethodBlik
}

// RecurringState is the state of a recurring payment, reported by
// Recurring.Status and returned by Recurring.Cancel.
type RecurringState string

// Recurring payment states. IN_PROGRESS is a state of the registering or
// charging transaction, not of the recurring payment.
const (
	RecurringStateActive RecurringState = "ACTIVE"
	// RecurringStateInactive means the registration waits for the customer.
	RecurringStateInactive     RecurringState = "INACTIVE"
	RecurringStateUnregistered RecurringState = "UNREGISTERED"
	RecurringStateExpired      RecurringState = "EXPIRED"
	RecurringStateDeclined     RecurringState = "DECLINED"
)

// Valid reports whether the value is one of the known recurring states.
func (s RecurringState) Valid() bool {
	switch s {
	case RecurringStateActive, RecurringStateInactive, RecurringStateUnregistered,
		RecurringStateExpired, RecurringStateDeclined:
		return true
	}
	return false
}

// RecurringRetryStatus is the outcome of retrying a declined recurring charge.
type RecurringRetryStatus string

// Recurring retry statuses.
const (
	// RecurringRetryStatusPending: the retry went to the bank; the outcome comes
	// like for a charge (webhook, IPN, transaction details).
	RecurringRetryStatusPending RecurringRetryStatus = "pending"
	// RecurringRetryStatusFailed: the bank declined the retry at once.
	RecurringRetryStatusFailed  RecurringRetryStatus = "failed"
	RecurringRetryStatusSuccess RecurringRetryStatus = "success"
)

// Valid reports whether the value is one of the known retry statuses.
func (s RecurringRetryStatus) Valid() bool {
	return s == RecurringRetryStatusPending || s == RecurringRetryStatusFailed || s == RecurringRetryStatusSuccess
}

// WebhookEventType is the type of a webhook event (the "type" field of the envelope).
type WebhookEventType string

// Webhook event types. MerchantEventTypes lists the ones a merchant endpoint can
// subscribe to and the Events API can filter on.
const (
	WebhookEventTypePaymentSucceeded          WebhookEventType = "payment.succeeded"
	WebhookEventTypePaymentFailed             WebhookEventType = "payment.failed"
	WebhookEventTypePaymentCaptured           WebhookEventType = "payment.captured"
	WebhookEventTypeRefundSucceeded           WebhookEventType = "refund.succeeded"
	WebhookEventTypeRefundFailed              WebhookEventType = "refund.failed"
	WebhookEventTypeRecurringPaymentActivated WebhookEventType = "recurring_payment.activated"
	WebhookEventTypeRecurringPaymentCanceled  WebhookEventType = "recurring_payment.canceled"
	WebhookEventTypeRecurringPaymentExpired   WebhookEventType = "recurring_payment.expired"
	WebhookEventTypeRecurringPaymentDeclined  WebhookEventType = "recurring_payment.declined"
	WebhookEventTypePayoutPaid                WebhookEventType = "payout.paid"
	WebhookEventTypePayoutFailed              WebhookEventType = "payout.failed"
	// WebhookEventTypeWebhookTest is sent by the test button of an endpoint in the panel.
	WebhookEventTypeWebhookTest WebhookEventType = "webhook.test"
)

// Valid reports whether the value is one of the known event types.
func (t WebhookEventType) Valid() bool {
	return t == WebhookEventTypeWebhookTest || containsEventType(MerchantEventTypes(), t)
}

// RedirectType tells the merchant what to do with a card payment result.
type RedirectType string

// Redirect types returned by card operations.
const (
	RedirectTypeSuccess  RedirectType = "SUCCESS"
	RedirectTypeForm     RedirectType = "FORM"
	RedirectTypeURL      RedirectType = "URL"
	RedirectTypeDCCOffer RedirectType = "DCC_OFFER"
)

// Valid reports whether the value is one of the known redirect types.
func (t RedirectType) Valid() bool {
	switch t {
	case RedirectTypeSuccess, RedirectTypeForm, RedirectTypeURL, RedirectTypeDCCOffer:
		return true
	}
	return false
}

// DCCDecision is the cardholder's answer to a dynamic currency conversion offer.
type DCCDecision string

// DCC decisions.
const (
	DCCDecisionAccept DCCDecision = "accept"
	DCCDecisionReject DCCDecision = "reject"
)

// Valid reports whether the value is one of the known DCC decisions.
func (d DCCDecision) Valid() bool {
	return d == DCCDecisionAccept || d == DCCDecisionReject
}

// CardRecurringFrequency is the charge interval of a card mandate.
type CardRecurringFrequency string

// Card recurring frequencies.
const (
	CardRecurringFrequencyDaily      CardRecurringFrequency = "DAILY"
	CardRecurringFrequencyWeekly     CardRecurringFrequency = "WEEKLY"
	CardRecurringFrequencyBiweekly   CardRecurringFrequency = "BIWEEKLY"
	CardRecurringFrequencyMonthly    CardRecurringFrequency = "MONTHLY"
	CardRecurringFrequencyQuarterly  CardRecurringFrequency = "QUARTERLY"
	CardRecurringFrequencySemiannual CardRecurringFrequency = "SEMIANNUAL"
	CardRecurringFrequencyAnnual     CardRecurringFrequency = "ANNUAL"
)

// Valid reports whether the value is one of the known frequencies.
func (f CardRecurringFrequency) Valid() bool {
	switch f {
	case CardRecurringFrequencyDaily, CardRecurringFrequencyWeekly, CardRecurringFrequencyBiweekly,
		CardRecurringFrequencyMonthly, CardRecurringFrequencyQuarterly,
		CardRecurringFrequencySemiannual, CardRecurringFrequencyAnnual:
		return true
	}
	return false
}

// CardRecurringOperation selects what a card-on-file registration does.
type CardRecurringOperation string

// Card recurring operations.
const (
	CardRecurringOperationAddCard    CardRecurringOperation = "add_card"
	CardRecurringOperationCOFInitial CardRecurringOperation = "cof_initial"
	CardRecurringOperationCharge     CardRecurringOperation = "charge"
)

// Valid reports whether the value is one of the known operations.
func (o CardRecurringOperation) Valid() bool {
	switch o {
	case CardRecurringOperationAddCard, CardRecurringOperationCOFInitial, CardRecurringOperationCharge:
		return true
	}
	return false
}

// PayoutFeeMode decides whether payout positions are net or gross of fees.
type PayoutFeeMode string

// Payout fee modes.
const (
	PayoutFeeModeNet   PayoutFeeMode = "net"
	PayoutFeeModeGross PayoutFeeMode = "gross"
)

// Valid reports whether the value is one of the known fee modes.
func (m PayoutFeeMode) Valid() bool {
	return m == PayoutFeeModeNet || m == PayoutFeeModeGross
}

// IPNType is the kind of event an IPN notification reports.
type IPNType string

// IPN types.
const (
	IPNTypeTransfer IPNType = "transfer"
	// Deprecated: dpay no longer sends capture IPNs - use the payment.captured
	// webhook event (WebhookEventTypePaymentCaptured).
	IPNTypeCapture IPNType = "capture"
	IPNTypeDCB     IPNType = "dcb"
)

// Valid reports whether the value is one of the known IPN types.
func (t IPNType) Valid() bool {
	return t == IPNTypeTransfer || t == IPNTypeCapture || t == IPNTypeDCB
}
