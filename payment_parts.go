package dpay

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/dpayglobal/dpay-go-sdk/internal/wire"
)

var (
	emailPattern         = regexp.MustCompile(`^[^@\s]+@[^@\s.]+(\.[^@\s.]+)+$`)
	datePattern          = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)
	blikFrequencyPattern = regexp.MustCompile(`^[1-9][0-9]{0,2}[DWMQY]$`)
	partnerPattern       = regexp.MustCompile(`^[A-Z0-9]{1,64}$`)
	blikCodePattern      = regexp.MustCompile(`^\d{6}$`)
	panPattern           = regexp.MustCompile(`^\d{12,19}$`)
	cvvPattern           = regexp.MustCompile(`^\d{3,4}$`)
	expiryPattern        = regexp.MustCompile(`^(0[1-9]|1[0-2])/\d{2}$`)
)

func validURL(value string) bool {
	if value == "" || strings.ContainsAny(value, " \t\r\n") {
		return false
	}
	parsed, err := url.Parse(value)
	return err == nil && parsed.Scheme != "" && parsed.Host != ""
}

func validEmail(value string) bool {
	return emailPattern.MatchString(value)
}

func validDate(value string) bool {
	return datePattern.MatchString(value)
}

func validateDate(value string) error {
	if !validDate(value) {
		return newValidationError(fmt.Sprintf("Date %q must be in YYYY-MM-DD format", value))
	}
	return nil
}

func runeLen(value string) int {
	return utf8.RuneCountInString(value)
}

// ReturnURLs carries the three URLs dpay redirects to and notifies.
type ReturnURLs struct {
	// Success is where the payer lands after a successful payment.
	Success string
	// Fail is where the payer lands after a failed payment.
	Fail string
	// IPN receives the server-to-server payment notification.
	IPN string
}

// Validate reports whether all three URLs are well formed.
func (u ReturnURLs) Validate() error {
	for _, pair := range []struct {
		name  string
		value string
	}{{"success", u.Success}, {"fail", u.Fail}, {"ipn", u.IPN}} {
		if !validURL(pair.value) {
			return newValidationError(fmt.Sprintf("Invalid %s URL %q", pair.name, pair.value))
		}
	}
	return nil
}

// Payer carries the optional payer identity sent with a registration.
type Payer struct {
	// Email is the payer address, validated when present.
	Email *string
	// FirstName is sent as client_name.
	FirstName *string
	// LastName is sent as client_surname.
	LastName *string
}

// Validate reports whether the email, when present, is well formed.
func (p Payer) Validate() error {
	if p.Email != nil && !validEmail(*p.Email) {
		return newValidationError(fmt.Sprintf("Invalid email %q", *p.Email))
	}
	return nil
}

// DeviceInfo describes the payer's browser, required by 3-D Secure.
type DeviceInfo struct {
	// BrowserAcceptHeader is the Accept header of the payer's browser.
	BrowserAcceptHeader string
	// BrowserLanguage is the browser language tag.
	BrowserLanguage string
	// BrowserColorDepth is the screen color depth in bits.
	BrowserColorDepth int
	// BrowserScreenHeight is the screen height in pixels.
	BrowserScreenHeight int
	// BrowserScreenWidth is the screen width in pixels.
	BrowserScreenWidth int
	// BrowserTZ is the timezone offset in minutes.
	BrowserTZ int
	// BrowserUserAgent is the browser User-Agent string.
	BrowserUserAgent string
	// SystemFamily is the operating system family.
	SystemFamily string
	// GeoLocalization is the payer's coordinates.
	GeoLocalization string
	// DeviceID identifies the device, 1-64 characters. Sent as deviceID.
	DeviceID string
	// ApplicationName identifies the merchant application, 1-64 characters.
	ApplicationName string
	// BrowserJavaEnabled is sent as the string "true" or "false" when set.
	BrowserJavaEnabled *bool
}

// Validate reports whether DeviceID and ApplicationName are within range.
func (d DeviceInfo) Validate() error {
	if length := runeLen(d.DeviceID); length == 0 || length > 64 {
		return newValidationError("Device ID must be 1-64 characters")
	}
	if length := runeLen(d.ApplicationName); length == 0 || length > 64 {
		return newValidationError("Application name must be 1-64 characters")
	}
	return nil
}

func (d DeviceInfo) toBody() *wire.Body {
	body := wire.NewBody()
	body.Set("browserAcceptHeader", d.BrowserAcceptHeader)
	body.Set("browserLanguage", d.BrowserLanguage)
	body.Set("browserColorDepth", d.BrowserColorDepth)
	body.Set("browserScreenHeight", d.BrowserScreenHeight)
	body.Set("browserScreenWidth", d.BrowserScreenWidth)
	body.Set("browserTZ", d.BrowserTZ)
	body.Set("browserUserAgent", d.BrowserUserAgent)
	body.Set("systemFamily", d.SystemFamily)
	body.Set("geoLocalization", d.GeoLocalization)
	body.Set("deviceID", d.DeviceID)
	body.Set("applicationName", d.ApplicationName)
	if d.BrowserJavaEnabled != nil {
		if *d.BrowserJavaEnabled {
			body.Set("browserJavaEnabled", "true")
		} else {
			body.Set("browserJavaEnabled", "false")
		}
	}
	return body
}

// InvoiceDetails carries the KSeF e-invoice data attached to a registration.
type InvoiceDetails struct {
	// PayerNIP is the payer's tax identification number.
	PayerNIP *string
	// PayerName is the payer's legal name.
	PayerName *string
	// InvoiceNumber is the merchant's invoice number.
	InvoiceNumber *string
	// PaymentDueDate is the due date in YYYY-MM-DD format.
	PaymentDueDate *string
	// VatAmount is the VAT amount, sent in minor units.
	VatAmount *Money
}

// Validate reports whether the due date, when present, is well formed.
func (i InvoiceDetails) Validate() error {
	if i.PaymentDueDate != nil && !validDate(*i.PaymentDueDate) {
		return newValidationError("Payment due date must be in YYYY-MM-DD format")
	}
	return nil
}

func (i InvoiceDetails) toBody() *wire.Body {
	body := wire.NewBody()
	body.SetIfNotNil("payer_nip", i.PayerNIP)
	body.SetIfNotNil("payer_name", i.PayerName)
	body.SetIfNotNil("invoice_number", i.InvoiceNumber)
	body.SetIfNotNil("payment_due_date", i.PaymentDueDate)
	if i.VatAmount != nil {
		body.Set("vat_amount", i.VatAmount.Minor())
	}
	return body
}

// PayoutPosition is a single 1:1 payout instruction line.
type PayoutPosition struct {
	// IBAN is the receiving account.
	IBAN string
	// Title is the transfer title, 1-255 characters.
	Title string
	// Amount is the payout amount, sent as a float.
	Amount Money
}

// Validate reports whether the IBAN and title are within range.
func (p PayoutPosition) Validate() error {
	if p.IBAN == "" {
		return newValidationError("Payout IBAN must not be empty")
	}
	if length := runeLen(p.Title); length == 0 || length > 255 {
		return newValidationError("Payout title must be 1-255 characters")
	}
	return nil
}

func (p PayoutPosition) toBody() *wire.Body {
	body := wire.NewBody()
	body.Set("iban", p.IBAN)
	body.Set("title", p.Title)
	body.Set("amount", moneyFloat(p.Amount))
	return body
}

// PayoutInstruction splits a payment into 1:1 payouts.
type PayoutInstruction struct {
	// Positions must hold at least one entry.
	Positions []PayoutPosition
	// FeeMode decides whether positions are net or gross of fees.
	FeeMode PayoutFeeMode
}

// Validate reports whether the instruction has positions, a known fee mode and valid lines.
func (p PayoutInstruction) Validate() error {
	if len(p.Positions) == 0 {
		return newValidationError("Payout instruction requires at least one position")
	}
	for _, position := range p.Positions {
		if err := position.Validate(); err != nil {
			return err
		}
	}
	if !p.FeeMode.Valid() {
		return newValidationError(fmt.Sprintf("Invalid payout fee mode %q", string(p.FeeMode)))
	}
	return nil
}

func (p PayoutInstruction) toBody() *wire.Body {
	positions := make([]any, 0, len(p.Positions))
	for _, position := range p.Positions {
		positions = append(positions, position.toBody())
	}
	body := wire.NewBody()
	body.Set("fee_mode", string(p.FeeMode))
	body.Set("positions", positions)
	return body
}

// BlikAliasRegistration asks dpay to register a BLIK alias during a payment.
type BlikAliasRegistration struct {
	// Label is shown to the payer in their banking app, 1-50 characters.
	Label string
	// Type selects the alias kind.
	Type BlikAliasType
}

// Validate reports whether the label length and alias type are acceptable.
func (b BlikAliasRegistration) Validate() error {
	if length := runeLen(b.Label); length == 0 || length > 50 {
		return newValidationError("Alias label must be 1-50 characters")
	}
	if !b.Type.Valid() {
		return newValidationError(fmt.Sprintf("Invalid BLIK alias type %q", string(b.Type)))
	}
	return nil
}

func (b BlikAliasRegistration) toBody() *wire.Body {
	body := wire.NewBody()
	body.Set("label", b.Label)
	body.Set("type", string(b.Type))
	return body
}

// BlikRecurringRegistration asks dpay to register a BLIK recurring mandate.
type BlikRecurringRegistration struct {
	// Label is shown to the payer, 1-50 characters.
	Label string
	// Model is the mandate model.
	Model BlikRecurringModel
	// Frequency matches ^[1-9][0-9]{0,2}[DWMQY]$, for example 1M.
	Frequency string
	// Value is the recurring amount, sent as a decimal string.
	Value *Money
	// LimitAmt is the per-charge limit in minor units.
	LimitAmt *int
	// TotLimitAmt is the total limit in minor units.
	TotLimitAmt *int
	// LimitAmtFixed marks the limit as fixed.
	LimitAmtFixed *bool
	// ExpirationDate is the mandate expiry in YYYY-MM-DD format.
	ExpirationDate *string
	// InitDate is the first charge date in YYYY-MM-DD format.
	InitDate *string
}

// Validate reports whether the label, model, frequency and dates are acceptable.
func (b BlikRecurringRegistration) Validate() error {
	if length := runeLen(b.Label); length == 0 || length > 50 {
		return newValidationError("Alias label must be 1-50 characters")
	}
	if !b.Model.Valid() {
		return newValidationError(fmt.Sprintf("Invalid recurring model %q", string(b.Model)))
	}
	if !blikFrequencyPattern.MatchString(b.Frequency) {
		return newValidationError(fmt.Sprintf("Invalid recurring frequency %q", b.Frequency))
	}
	if b.ExpirationDate != nil {
		if err := validateDate(*b.ExpirationDate); err != nil {
			return err
		}
	}
	if b.InitDate != nil {
		if err := validateDate(*b.InitDate); err != nil {
			return err
		}
	}
	return nil
}

func (b BlikRecurringRegistration) toBody() *wire.Body {
	body := wire.NewBody()
	body.Set("label", b.Label)
	body.Set("type", string(BlikAliasTypePayID))
	body.Set("model", string(b.Model))
	body.Set("frequency", b.Frequency)
	if b.Value != nil {
		body.Set("value", b.Value.String())
	}
	body.SetIfNotNil("limit_amt", b.LimitAmt)
	body.SetIfNotNil("tot_limit_amt", b.TotLimitAmt)
	body.SetIfNotNil("is_limit_amt_fixed", b.LimitAmtFixed)
	body.SetIfNotNil("expiration_date", b.ExpirationDate)
	body.SetIfNotNil("init_date", b.InitDate)
	return body
}

// CardRecurringRegistration asks dpay to register a card-on-file mandate.
type CardRecurringRegistration struct {
	// Label is shown to the payer, 1-50 characters.
	Label string
	// Frequency is the charge interval, empty when not declared.
	Frequency CardRecurringFrequency
	// LimitAmt is the per-charge limit, sent in minor units.
	LimitAmt *Money
	// TotLimitAmt is the total limit, sent in minor units.
	TotLimitAmt *Money
	// LimitAmtFixed marks the limit as fixed.
	LimitAmtFixed *bool
	// ExpirationDate is the mandate expiry in YYYY-MM-DD format.
	ExpirationDate *string
	// InitDate is the first charge date in YYYY-MM-DD format.
	InitDate *string
}

// Validate reports whether the label, frequency and dates are acceptable.
func (c CardRecurringRegistration) Validate() error {
	if length := runeLen(c.Label); length == 0 || length > 50 {
		return newValidationError("Mandate label must be 1-50 characters")
	}
	if c.Frequency != "" && !c.Frequency.Valid() {
		return newValidationError(fmt.Sprintf("Invalid card recurring frequency %q", string(c.Frequency)))
	}
	if c.ExpirationDate != nil {
		if err := validateDate(*c.ExpirationDate); err != nil {
			return err
		}
	}
	if c.InitDate != nil {
		if err := validateDate(*c.InitDate); err != nil {
			return err
		}
	}
	return nil
}

func (c CardRecurringRegistration) toBody() *wire.Body {
	body := wire.NewBody()
	body.Set("label", c.Label)
	if c.Frequency != "" {
		body.Set("frequency", string(c.Frequency))
	}
	if c.LimitAmt != nil {
		body.Set("limit_amt", c.LimitAmt.Minor())
	}
	if c.TotLimitAmt != nil {
		body.Set("tot_limit_amt", c.TotLimitAmt.Minor())
	}
	body.SetIfNotNil("is_limit_amt_fixed", c.LimitAmtFixed)
	body.SetIfNotNil("expiration_date", c.ExpirationDate)
	body.SetIfNotNil("init_date", c.InitDate)
	return body
}
