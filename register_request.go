package dpay

import (
	"fmt"

	"github.com/dpayglobal/dpay-go-sdk/internal/wire"
)

// RegisterPaymentRequest describes a payment to register. Amount, TransactionType
// and URLs are required; every pointer field is omitted from the request when nil.
type RegisterPaymentRequest struct {
	// Amount is the transaction amount.
	Amount Money
	// TransactionType selects the payment flow.
	TransactionType TransactionType
	// URLs are the success, failure and IPN endpoints.
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
	// UserAgent is the payer browser User-Agent, required with BlikCode and BlikAlias.
	UserAgent *string
	// UserIP is the payer IP address, required with BlikCode and BlikAlias.
	UserIP *string

	// BlikCode is a six-digit BLIK code, mutually exclusive with BlikAlias.
	BlikCode *string
	// BlikAlias pays with a registered alias, mutually exclusive with BlikCode and alias registration.
	BlikAlias *string
	// RegisterBlikAlias registers a BLIK alias during this payment.
	RegisterBlikAlias *BlikAliasRegistration
	// RegisterBlikRecurringAlias registers a BLIK recurring mandate during this payment.
	RegisterBlikRecurringAlias *BlikRecurringRegistration
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
	if r.BlikCode != nil && !blikCodePattern.MatchString(*r.BlikCode) {
		return newValidationError("BLIK code must be exactly 6 digits")
	}
	if r.BlikCode != nil && r.BlikAlias != nil {
		return newValidationError("blik_code cannot be combined with blik_alias")
	}
	if r.BlikAlias != nil && (r.RegisterBlikAlias != nil || r.RegisterBlikRecurringAlias != nil) {
		return newValidationError("blik_alias cannot be combined with blik_code or alias registration")
	}
	if r.RegisterBlikAlias != nil {
		if err := r.RegisterBlikAlias.Validate(); err != nil {
			return err
		}
	}
	if r.RegisterBlikRecurringAlias != nil {
		if err := r.RegisterBlikRecurringAlias.Validate(); err != nil {
			return err
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
	return nil
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
	body.Set("url_ipn", r.URLs.IPN)

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
	if r.RegisterBlikRecurringAlias != nil {
		body.Set("register_blik_recurring_alias", r.RegisterBlikRecurringAlias.toBody())
	}
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
