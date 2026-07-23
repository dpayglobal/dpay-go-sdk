package dpay

// TransactionType selects the payment flow a registration starts.
type TransactionType string

// Transaction types accepted by payments/register.
const (
	TransactionTypeTransfers     TransactionType = "transfers"
	TransactionTypeDCBGateway    TransactionType = "dcb_gateway"
	TransactionTypeCardAuth      TransactionType = "card_auth"
	TransactionTypeMBWayDirect   TransactionType = "mb_way_direct"
	TransactionTypeBizumDirect   TransactionType = "bizum_direct"
	TransactionTypeBlikRecurring TransactionType = "blik_recurring"
	TransactionTypeCardRecurring TransactionType = "card_recurring"
)

// Valid reports whether the value is one of the known transaction types.
func (t TransactionType) Valid() bool {
	switch t {
	case TransactionTypeTransfers, TransactionTypeDCBGateway, TransactionTypeCardAuth,
		TransactionTypeMBWayDirect, TransactionTypeBizumDirect,
		TransactionTypeBlikRecurring, TransactionTypeCardRecurring:
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

// BlikAliasType distinguishes a device alias from a PayID alias.
type BlikAliasType string

// BLIK alias types.
const (
	BlikAliasTypeUID   BlikAliasType = "UID"
	BlikAliasTypePayID BlikAliasType = "PAYID"
)

// Valid reports whether the value is one of the known alias types.
func (t BlikAliasType) Valid() bool {
	return t == BlikAliasTypeUID || t == BlikAliasTypePayID
}

// BlikRecurringModel is the mandate model of a BLIK recurring registration.
type BlikRecurringModel string

// BLIK recurring models: automatic, manual and on-demand.
const (
	BlikRecurringModelAutomatic BlikRecurringModel = "A"
	BlikRecurringModelManual    BlikRecurringModel = "M"
	BlikRecurringModelOnDemand  BlikRecurringModel = "O"
)

// Valid reports whether the value is one of the known recurring models.
func (m BlikRecurringModel) Valid() bool {
	return m == BlikRecurringModelAutomatic || m == BlikRecurringModelManual || m == BlikRecurringModelOnDemand
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
	IPNTypeCapture  IPNType = "capture"
	IPNTypeDCB      IPNType = "dcb"
)

// Valid reports whether the value is one of the known IPN types.
func (t IPNType) Valid() bool {
	return t == IPNTypeTransfer || t == IPNTypeCapture || t == IPNTypeDCB
}
