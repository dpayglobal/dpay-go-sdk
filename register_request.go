package dpay

import (
	"fmt"
	"net"

	"github.com/dpayglobal/dpay-go-sdk/internal/php"
	"github.com/dpayglobal/dpay-go-sdk/internal/wire"
)

// RegisterPaymentRequest describes a payment to register. Amount, TransactionType
// and URLs are required; every pointer field is omitted from the request when nil.
type RegisterPaymentRequest struct {
	// Amount is the transaction amount.
	Amount Money
	// TransactionType selects the payment flow.
	TransactionType TransactionType
	// URLs are the success and failure pages and the optional IPN endpoint.
	URLs ReturnURLs

	// Description is shown to the payer.
	Description *string
	// Custom is an opaque merchant reference echoed back in the IPN.
	Custom *string
	// Payer carries the payer identity.
	Payer *Payer
	// AcceptTos marks the terms of service as accepted.
	AcceptTos *bool
	// Channel preselects a payment channel.
	Channel *string

	// CreditCard toggles the card channel, sent as 0 or 1.
	CreditCard *bool
	// Paysafecard toggles the paysafecard channel, sent as 0 or 1.
	Paysafecard *bool
	// Blik toggles the BLIK channel, sent as 0 or 1.
	Blik *bool
	// Installment toggles installments, sent as 0 or 1.
	Installment *bool
	// PayPal toggles PayPal, sent as 0 or 1.
	PayPal *bool
	// NoBanks hides bank transfers, sent as 0 or 1.
	NoBanks *bool

	// PhoneNumber is the payer phone number for direct carrier billing.
	PhoneNumber *string
	// CurrencyCode overrides the transaction currency.
	CurrencyCode Currency
	// PartnerPlatform identifies the integration platform, matching ^[A-Z0-9]{1,64}$.
	PartnerPlatform *string
	// UserAgent is the payer browser User-Agent: required with BlikCode and
	// BlikAlias, optional with RecurringAlias.
	UserAgent *string
	// UserIP is the payer IP address: required with BlikCode and BlikAlias,
	// optional with RecurringAlias. It must be an IPv4 or IPv6 address.
	UserIP *string

	// BlikCode is a six-digit BLIK code, mutually exclusive with BlikAlias.
	BlikCode *string
	// BlikAlias pays with a registered OneClick alias, mutually exclusive with
	// BlikCode, alias registration and recurring payments.
	BlikAlias *string
	// RegisterBlikAlias registers a BLIK OneClick alias during this payment.
	RegisterBlikAlias *BlikAliasRegistration
	// RecurringRegistration registers a recurring payment together with this
	// payment. It requires BlikCode and TransactionTypeTransfers; the amount may
	// be 0 (consent only) or an initial fee.
	RecurringRegistration *RecurringRegistration
	// RecurringAlias charges a registered recurring payment server-to-server,
	// without a BLIK code: TransactionTypeTransfers, amount above 0, 1-128
	// characters. The alias is appended to the checksum, binding the charge to
	// that customer.
	RecurringAlias *string
	// AliasIPNURL receives alias lifecycle notifications.
	AliasIPNURL *string
	// NoDelay requests immediate processing.
	NoDelay *bool

	// CardRecurring registers a card-on-file mandate, mutually exclusive with CardRecurringAlias.
	CardRecurring *CardRecurringRegistration
	// CardRecurringAlias charges a stored card, mutually exclusive with CardRecurring.
	CardRecurringAlias *string
	// AuthorizeOnly authorizes without capturing.
	AuthorizeOnly *bool
	// CardRecurringOperation selects the card-on-file operation.
	CardRecurringOperation CardRecurringOperation

	// Payout splits the payment into 1:1 payouts.
	Payout *PayoutInstruction
	// BillingAddress is a free-form object; use NewFields to control key order.
	BillingAddress any
	// ShippingAddress is a free-form object; use NewFields to control key order.
	ShippingAddress any
	// DeviceInfo describes the payer browser, required by 3-D Secure.
	DeviceInfo *DeviceInfo
	// Products lists the ordered items; use NewFields for each entry to control key order.
	Products []any
	// Efaktura requests a KSeF e-invoice; allowed only with TransactionTypeTransfers.
	Efaktura *bool
	// Invoice carries the e-invoice details.
	Invoice *InvoiceDetails

	// Webhook sends the events of this payment (and later of its refunds and
	// recurring payment) also to this URL, signed with the webhook secret of the
	// service. It does not enter the checksum.
	Webhook *WebhookTarget
	// Reference is your reference of the payment, 1-64 characters without
	// control characters (surrounding spaces are trimmed), returned as
	// references.merchant in webhooks. It does not enter the checksum.
	Reference *string
}

// Validate reports the first problem with the request, using the same messages
// as the PHP SDK. It is called automatically before the request is sent.
func (r *RegisterPaymentRequest) Validate() error {
	if !r.TransactionType.Valid() {
		return newValidationError(fmt.Sprintf("Invalid transaction type %q", string(r.TransactionType)))
	}
	if err := r.URLs.Validate(); err != nil {
		return err
	}
	if r.Payer != nil {
		if err := r.Payer.Validate(); err != nil {
			return err
		}
	}
	if r.CurrencyCode != "" && !r.CurrencyCode.Valid() {
		return newValidationError(fmt.Sprintf("Invalid currency code %q", string(r.CurrencyCode)))
	}
	if r.PartnerPlatform != nil && !partnerPattern.MatchString(*r.PartnerPlatform) {
		return newValidationError("Partner platform must match ^[A-Z0-9]{1,64}$")
	}
	if r.UserIP != nil && net.ParseIP(*r.UserIP) == nil {
		return newValidationError(fmt.Sprintf("Invalid user IP %q", *r.UserIP))
	}
	if r.BlikCode != nil && !blikCodePattern.MatchString(*r.BlikCode) {
		return newValidationError("BLIK code must be exactly 6 digits")
	}
	if r.BlikCode != nil && r.BlikAlias != nil {
		return newValidationError("blik_code cannot be combined with blik_alias")
	}
	if r.BlikAlias != nil && (r.RegisterBlikAlias != nil || r.RecurringRegistration != nil || r.RecurringAlias != nil) {
		return newValidationError("blik_alias cannot be combined with blik_code, alias registration or recurring payments")
	}
	if r.RegisterBlikAlias != nil {
		if err := r.RegisterBlikAlias.Validate(); err != nil {
			return err
		}
	}
	if r.RecurringRegistration != nil {
		if err := r.RecurringRegistration.validateFields(); err != nil {
			return err
		}
	}
	if r.RecurringAlias != nil {
		if err := validateRecurringAlias(*r.RecurringAlias); err != nil {
			return err
		}
	}
	if r.Webhook != nil {
		if err := r.Webhook.validateFor(PaymentRegistrationEventTypes(), "a payment registration"); err != nil {
			return err
		}
	}
	if r.Reference != nil {
		reference := php.Trim(*r.Reference)
		if length := runeLen(reference); length == 0 || length > 64 || controlCharacterPattern.MatchString(reference) {
			return newValidationError("Reference must be 1-64 characters without control characters")
		}
	}
	if r.AliasIPNURL != nil && !validURL(*r.AliasIPNURL) {
		return newValidationError(fmt.Sprintf("Invalid alias IPN URL %q", *r.AliasIPNURL))
	}
	if r.CardRecurring != nil && r.CardRecurringAlias != nil {
		return newValidationError("register_card_recurring cannot be combined with card_recurring_alias")
	}
	if r.CardRecurring != nil {
		if err := r.CardRecurring.Validate(); err != nil {
			return err
		}
	}
	if r.CardRecurringOperation != "" && !r.CardRecurringOperation.Valid() {
		return newValidationError(fmt.Sprintf("Invalid card recurring operation %q", string(r.CardRecurringOperation)))
	}
	if r.Payout != nil {
		if err := r.Payout.Validate(); err != nil {
			return err
		}
	}
	if r.DeviceInfo != nil {
		if err := r.DeviceInfo.Validate(); err != nil {
			return err
		}
	}
	if r.Efaktura != nil && r.TransactionType != TransactionTypeTransfers {
		return newValidationError(`efaktura is allowed only for transactionType "transfers"`)
	}
	if r.Invoice != nil {
		if err := r.Invoice.Validate(); err != nil {
			return err
		}
	}
	if err := r.validateRecurringCombination(); err != nil {
		return err
	}
	if r.RecurringRegistration != nil {
		return r.RecurringRegistration.validateModel()
	}
	return nil
}

// validateRecurringCombination mirrors RegisterPaymentRequest::assertRecurringCombination of the PHP SDK.
func (r *RegisterPaymentRequest) validateRecurringCombination() error {
	if r.RecurringRegistration == nil && r.RecurringAlias == nil {
		return nil
	}
	if r.RecurringRegistration != nil && r.RecurringAlias != nil {
		return newValidationError("recurring_registration cannot be combined with recurring_alias")
	}
	if r.TransactionType != TransactionTypeTransfers {
		return newValidationError(`Recurring payments require transactionType "transfers"`)
	}
	conflicts := []presentField{
		{"blik_alias", r.BlikAlias != nil},
		{"register_blik_alias", r.RegisterBlikAlias != nil},
		{"register_card_recurring", r.CardRecurring != nil},
		{"card_recurring_alias", r.CardRecurringAlias != nil},
	}
	if r.RecurringRegistration != nil {
		conflicts = append(conflicts, presentField{"channel", r.Channel != nil})
		if r.BlikCode == nil {
			return newValidationError("recurring_registration requires the customer's BLIK code (BlikCode)")
		}
	} else {
		conflicts = append(conflicts, presentField{"blik_code", r.BlikCode != nil})
		if r.Amount.Minor() <= 0 {
			return newValidationError("A recurring charge requires an amount above 0")
		}
	}
	for _, conflict := range conflicts {
		if conflict.present {
			return newValidationError(conflict.name + " cannot be combined with a recurring payment")
		}
	}
	return nil
}

// presentField is a request field name and whether it is set.
type presentField struct {
	name    string
	present bool
}

func (r *RegisterPaymentRequest) toBody(service string) (*wire.Body, error) {
	if err := r.Validate(); err != nil {
		return nil, err
	}

	body := wire.NewBody()
	body.Set("service", service)
	body.Set("value", r.Amount.String())
	body.Set("transactionType", string(r.TransactionType))
	body.Set("url_success", r.URLs.Success)
	body.Set("url_fail", r.URLs.Fail)
	if r.URLs.IPN != "" {
		body.Set("url_ipn", r.URLs.IPN)
	}

	body.SetIfNotNil("description", r.Description)
	body.SetIfNotNil("custom", r.Custom)
	if r.Payer != nil {
		body.SetIfNotNil("email", r.Payer.Email)
		body.SetIfNotNil("client_name", r.Payer.FirstName)
		body.SetIfNotNil("client_surname", r.Payer.LastName)
	}
	body.SetIfNotNil("accept_tos", r.AcceptTos)
	body.SetIfNotNil("channel", r.Channel)

	setFlag(body, "creditcard", r.CreditCard)
	setFlag(body, "paysafecard", r.Paysafecard)
	setFlag(body, "blik", r.Blik)
	setFlag(body, "installment", r.Installment)
	setFlag(body, "paypal", r.PayPal)
	setFlag(body, "nobanks", r.NoBanks)

	body.SetIfNotNil("phone_number", r.PhoneNumber)
	if r.CurrencyCode != "" {
		body.Set("currency_code", string(r.CurrencyCode))
	}
	body.SetIfNotNil("partner_platform", r.PartnerPlatform)
	body.SetIfNotNil("user_agent", r.UserAgent)
	body.SetIfNotNil("user_ip", r.UserIP)

	body.SetIfNotNil("blik_code", r.BlikCode)
	body.SetIfNotNil("blik_alias", r.BlikAlias)
	if r.RegisterBlikAlias != nil {
		body.Set("register_blik_alias", r.RegisterBlikAlias.toBody())
	}
	if r.RecurringRegistration != nil {
		body.Set("recurring_registration", r.RecurringRegistration.toBody())
	}
	body.SetIfNotNil("recurring_alias", r.RecurringAlias)
	body.SetIfNotNil("alias_ipn_url", r.AliasIPNURL)
	body.SetIfNotNil("no_delay", r.NoDelay)

	if r.CardRecurring != nil {
		body.Set("register_card_recurring", r.CardRecurring.toBody())
	}
	body.SetIfNotNil("card_recurring_alias", r.CardRecurringAlias)
	body.SetIfNotNil("authorize_only", r.AuthorizeOnly)
	if r.CardRecurringOperation != "" {
		body.Set("card_recurring_operation", string(r.CardRecurringOperation))
	}

	if r.Payout != nil {
		body.Set("payout", r.Payout.toBody())
	}
	if r.BillingAddress != nil {
		body.Set("billing_address", r.BillingAddress)
	}
	if r.ShippingAddress != nil {
		body.Set("shipping_address", r.ShippingAddress)
	}
	if r.DeviceInfo != nil {
		body.Set("device_info", r.DeviceInfo.toBody())
	}
	if r.Products != nil {
		body.Set("products", r.Products)
	}
	body.SetIfNotNil("efaktura", r.Efaktura)
	if r.Invoice != nil {
		body.Set("invoice", r.Invoice.toBody())
	}
	if r.Webhook != nil {
		body.Set("webhook", r.Webhook.toBody())
	}
	if r.Reference != nil {
		body.Set("reference", php.Trim(*r.Reference))
	}
	return body, nil
}

func setFlag(body *wire.Body, key string, flag *bool) {
	if flag == nil {
		return
	}
	if *flag {
		body.Set(key, 1)
		return
	}
	body.Set(key, 0)
}
