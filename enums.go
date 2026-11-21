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

// RetryRefusal is why Recurring.Retry was refused (HTTP 400, nothing sent to
// BLIK): the field error retry of the ErrInvalidRequest error,
// APIError.FieldErrors["retry"][0]. With RetryRefusalDeclineNotRetryable the
// decline code is in FieldErrors["decline_reason"][0].
type RetryRefusal string

// Retry refusals.
const (
	RetryRefusalPaymentNotDeclined  RetryRefusal = "PAYMENT_NOT_DECLINED"
	RetryRefusalDeclineNotRetryable RetryRefusal = "DECLINE_NOT_RETRYABLE"
	RetryRefusalRetryLimitReached   RetryRefusal = "RETRY_LIMIT_REACHED"
	RetryRefusalRetryWindowExpired  RetryRefusal = "RETRY_WINDOW_EXPIRED"
	RetryRefusalAliasNotAvailable   RetryRefusal = "ALIAS_NOT_AVAILABLE"
	RetryRefusalRecurringNotEnabled RetryRefusal = "RECURRING_NOT_ENABLED"
	RetryRefusalPaymentNotFound     RetryRefusal = "PAYMENT_NOT_FOUND"
	RetryRefusalServiceUnavailable  RetryRefusal = "Service unavailable"
)

// RetryRefusals returns every known retry refusal.
func RetryRefusals() []RetryRefusal {
	return []RetryRefusal{
		RetryRefusalPaymentNotDeclined, RetryRefusalDeclineNotRetryable, RetryRefusalRetryLimitReached,
		RetryRefusalRetryWindowExpired, RetryRefusalAliasNotAvailable, RetryRefusalRecurringNotEnabled,
		RetryRefusalPaymentNotFound, RetryRefusalServiceUnavailable,
	}
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
	// IPNTypeAliasUpdate is a BLIK OneClick alias or recurring payment status
	// change, sent to the alias IPN URL (or the IPN URL without it).
	IPNTypeAliasUpdate IPNType = "alias_update"
)

// Valid reports whether the value is one of the known IPN types.
func (t IPNType) Valid() bool {
	return t == IPNTypeTransfer || t == IPNTypeCapture || t == IPNTypeDCB || t == IPNTypeAliasUpdate
}

// AliasChangeType is the change_type of an alias_update IPN.
type AliasChangeType string

// Alias change types.
const (
	AliasChangeRegister   AliasChangeType = "ALIAS_REGISTER"
	AliasChangeUpdate     AliasChangeType = "ALIAS_UPDATE"
	AliasChangeUnregister AliasChangeType = "ALIAS_UNREGISTER"
	AliasChangeExpired    AliasChangeType = "ALIAS_EXPIRED"
	AliasChangeDeclined   AliasChangeType = "ALIAS_DECLINED"
)

// AliasType is the alias_type of an alias_update IPN.
type AliasType string

// Alias types of an alias_update IPN.
const (
	// AliasTypeUID is a BLIK OneClick alias.
	AliasTypeUID AliasType = "UID"
	// AliasTypePayID is a recurring payment.
	AliasTypePayID AliasType = "PAYID"
)

// AliasStatus is the alias_status of an alias_update IPN.
type AliasStatus string

// Alias statuses.
const (
	AliasStatusActive       AliasStatus = "ACTIVE"
	AliasStatusInactive     AliasStatus = "INACTIVE"
	AliasStatusUnregistered AliasStatus = "UNREGISTERED"
	AliasStatusExpired      AliasStatus = "EXPIRED"
	AliasStatusDeclined     AliasStatus = "DECLINED"
)

// PayoutState is the state code of a payout.
type PayoutState int

// Payout states.
const (
	PayoutStateWaiting   PayoutState = 0
	PayoutStateProcessed PayoutState = 1
	PayoutStateFailed    PayoutState = -1
)

// Valid reports whether the value is one of the known payout states.
func (s PayoutState) Valid() bool {
	return s == PayoutStateWaiting || s == PayoutStateProcessed || s == PayoutStateFailed
}

// PayoutType is the kind of a payout in the payout list.
type PayoutType string

// Payout types.
const (
	PayoutTypeStandard    PayoutType = "standard"
	PayoutTypeDirect      PayoutType = "direct"
	PayoutTypePaysafecard PayoutType = "paysafecard"
)

// Valid reports whether the value is one of the known payout types.
func (t PayoutType) Valid() bool {
	return t == PayoutTypeStandard || t == PayoutTypeDirect || t == PayoutTypePaysafecard
}

// RegisteredTransactionStatus is the status of the transaction returned with a
// registration answered "Transaction paid". It differs from TransactionStatus.
type RegisteredTransactionStatus string

// Registered transaction statuses.
const (
	RegisteredTransactionStatusPending  RegisteredTransactionStatus = "pending"
	RegisteredTransactionStatusPaid     RegisteredTransactionStatus = "paid"
	RegisteredTransactionStatusCaptured RegisteredTransactionStatus = "captured"
	RegisteredTransactionStatusCanceled RegisteredTransactionStatus = "canceled"
)

// DeclineCode is the code of a failure in a webhook object (failure.code): the
// docs page "Decline codes" (backend FailureCatalog).
type DeclineCode string

// Decline codes.
const (
	DeclineCodeInsufficientFunds         DeclineCode = "insufficient_funds"
	DeclineCodeLimitExceeded             DeclineCode = "limit_exceeded"
	DeclineCodeCustomerDeclined          DeclineCode = "customer_declined"
	DeclineCodeCustomerTimeout           DeclineCode = "customer_timeout"
	DeclineCodeInvalidCode               DeclineCode = "invalid_code"
	DeclineCodeExpired                   DeclineCode = "expired"
	DeclineCodeAppUpdateRequired         DeclineCode = "app_update_required"
	DeclineCodeIssuerDeclined            DeclineCode = "issuer_declined"
	DeclineCodeCardInvalid               DeclineCode = "card_invalid"
	DeclineCodeAuthenticationRequired    DeclineCode = "authentication_required"
	DeclineCodeAliasInvalid              DeclineCode = "alias_invalid"
	DeclineCodeUnsupportedByBank         DeclineCode = "unsupported_by_bank"
	DeclineCodeGenericDecline            DeclineCode = "generic_decline"
	DeclineCodeSecurityDeclined          DeclineCode = "security_declined"
	DeclineCodeBlocked                   DeclineCode = "blocked"
	DeclineCodeRecurringConditionsNotMet DeclineCode = "recurring_conditions_not_met"
	DeclineCodeInvalidRequest            DeclineCode = "invalid_request"
	DeclineCodeRefundWindowExpired       DeclineCode = "refund_window_expired"
	DeclineCodeRefundAmountExceeded      DeclineCode = "refund_amount_exceeded"
	DeclineCodePayoutFailed              DeclineCode = "payout_failed"
	DeclineCodeRefundFailed              DeclineCode = "refund_failed"
	DeclineCodeProcessingError           DeclineCode = "processing_error"
)

// DeclineCodes returns every known decline code, in the order of the docs.
func DeclineCodes() []DeclineCode {
	return []DeclineCode{
		DeclineCodeInsufficientFunds, DeclineCodeLimitExceeded, DeclineCodeCustomerDeclined,
		DeclineCodeCustomerTimeout, DeclineCodeInvalidCode, DeclineCodeExpired, DeclineCodeAppUpdateRequired,
		DeclineCodeIssuerDeclined, DeclineCodeCardInvalid, DeclineCodeAuthenticationRequired,
		DeclineCodeAliasInvalid, DeclineCodeUnsupportedByBank, DeclineCodeGenericDecline,
		DeclineCodeSecurityDeclined, DeclineCodeBlocked, DeclineCodeRecurringConditionsNotMet,
		DeclineCodeInvalidRequest, DeclineCodeRefundWindowExpired, DeclineCodeRefundAmountExceeded,
		DeclineCodePayoutFailed, DeclineCodeRefundFailed, DeclineCodeProcessingError,
	}
}

// Valid reports whether the value is one of the known decline codes.
func (c DeclineCode) Valid() bool {
	for _, code := range DeclineCodes() {
		if code == c {
			return true
		}
	}
	return false
}

// FailureCategory is the category of a failure in a webhook object.
type FailureCategory string

// Failure categories.
const (
	FailureCategoryCustomer FailureCategory = "customer"
	FailureCategoryMerchant FailureCategory = "merchant"
	FailureCategoryRisk     FailureCategory = "risk"
	FailureCategorySystem   FailureCategory = "system"
)

// FailureCategories returns every known failure category.
func FailureCategories() []FailureCategory {
	return []FailureCategory{FailureCategoryCustomer, FailureCategoryMerchant, FailureCategoryRisk, FailureCategorySystem}
}

// PaymentObjectStatus is the status of a payment object in a webhook event.
type PaymentObjectStatus string

// Payment object statuses.
const (
	PaymentObjectStatusPending     PaymentObjectStatus = "pending"
	PaymentObjectStatusProcessing  PaymentObjectStatus = "processing"
	PaymentObjectStatusSucceeded   PaymentObjectStatus = "succeeded"
	PaymentObjectStatusCaptured    PaymentObjectStatus = "captured"
	PaymentObjectStatusFailed      PaymentObjectStatus = "failed"
	PaymentObjectStatusReversed    PaymentObjectStatus = "reversed"
	PaymentObjectStatusChargedBack PaymentObjectStatus = "charged_back"
)

// RecurringSequence tells the registering payment of a recurring payment from its charges.
type RecurringSequence string

// Recurring sequences.
const (
	RecurringSequenceFirst      RecurringSequence = "first"
	RecurringSequenceSubsequent RecurringSequence = "subsequent"
)

// RefundObjectStatus is the status of a refund object in a webhook event.
type RefundObjectStatus string

// Refund object statuses.
const (
	RefundObjectStatusSucceeded RefundObjectStatus = "succeeded"
	RefundObjectStatusFailed    RefundObjectStatus = "failed"
)

// RecurringPaymentObjectStatus is the status of a recurring_payment object in a webhook event.
type RecurringPaymentObjectStatus string

// Recurring payment object statuses.
const (
	RecurringPaymentObjectStatusPending  RecurringPaymentObjectStatus = "pending"
	RecurringPaymentObjectStatusActive   RecurringPaymentObjectStatus = "active"
	RecurringPaymentObjectStatusCanceled RecurringPaymentObjectStatus = "canceled"
	RecurringPaymentObjectStatusExpired  RecurringPaymentObjectStatus = "expired"
	RecurringPaymentObjectStatusDeclined RecurringPaymentObjectStatus = "declined"
)

// RecurringCanceledBy tells who canceled a recurring payment.
type RecurringCanceledBy string

// Who canceled a recurring payment.
const (
	RecurringCanceledByMerchant RecurringCanceledBy = "merchant"
	RecurringCanceledByCustomer RecurringCanceledBy = "customer"
	RecurringCanceledByDpay     RecurringCanceledBy = "dpay"
)

// PayoutObjectStatus is the status of a payout object in a webhook event.
type PayoutObjectStatus string

// Payout object statuses.
const (
	PayoutObjectStatusPending PayoutObjectStatus = "pending"
	PayoutObjectStatusPaid    PayoutObjectStatus = "paid"
	PayoutObjectStatusFailed  PayoutObjectStatus = "failed"
)
