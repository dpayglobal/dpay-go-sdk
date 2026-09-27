package dpay

import (
	"fmt"

	"github.com/dpayglobal/dpay-go-sdk/internal/wire"
)

// RecurringRegistration is the recurring_registration object: it registers a
// recurring payment (today BLIK) together with a payment that carries the
// customer's BLIK code (RegisterPaymentRequest.BlikCode) and
// TransactionTypeTransfers. The amount may be 0 (consent only) or an initial fee.
//
// Model rules: RecurringModelOnDemand forbids Frequency, LimitAmt, TotLimitAmt
// and LimitAmtFixed; RecurringModelAutomatic requires Frequency, LimitAmt,
// TotLimitAmt, ExpirationDate and InitDate and does not allow LimitAmtFixed=false;
// in RecurringModelManual they are optional.
type RecurringRegistration struct {
	// Label is shown to the customer, 1-50 characters.
	Label string
	// Model decides who approves later charges.
	Model RecurringModel
	// TermsURL is the merchant's terms the customer accepted (consent
	// evidence), a URL of at most 2048 characters.
	TermsURL string

	// Alias is your own alias of the recurring payment, 1-128 characters; nil
	// lets dpay assign one (RegisteredPayment.RecurringAlias returns it).
	Alias *string
	// TermsVersion is the version of the terms, 1-64 characters.
	TermsVersion *string
	// Methods lists the payment methods, today only RecurringMethodBlik. Nil
	// omits the field; a non-nil list must not be empty.
	Methods []RecurringMethod
	// Frequency is 1-999 days, weeks, months or years, e.g. 1M, 2W, 14D or 1Y.
	Frequency *string
	// LimitAmt is the single charge limit in minor units (grosz), at least 1.
	LimitAmt *int
	// TotLimitAmt is the total limit of all charges in minor units, at least 1.
	TotLimitAmt *int
	// LimitAmtFixed makes every charge equal LimitAmt.
	LimitAmtFixed *bool
	// ExpirationDate is YYYY-MM-DD, after today and at most 10 years ahead.
	ExpirationDate *string
	// InitDate is the date of the first charge, YYYY-MM-DD, today or later.
	InitDate *string
}

// Validate reports the first problem with the registration, including the rules
// of its model, using the same messages as the PHP SDK.
func (r RecurringRegistration) Validate() error {
	if err := r.validateFields(); err != nil {
		return err
	}
	return r.validateModel()
}

func (r RecurringRegistration) validateFields() error {
	if length := runeLen(r.Label); length == 0 || length > 50 {
		return newValidationError("Recurring payment label must be 1-50 characters")
	}
	if !r.Model.Valid() {
		return newValidationError(fmt.Sprintf("Invalid recurring model %q", string(r.Model)))
	}
	if len(r.TermsURL) > 2048 || !validURL(r.TermsURL) {
		return newValidationError(fmt.Sprintf("Invalid terms URL %q", r.TermsURL))
	}
	if r.Alias != nil {
		if err := validateRecurringAlias(*r.Alias); err != nil {
			return err
		}
	}
	if r.TermsVersion != nil {
		if length := runeLen(*r.TermsVersion); length == 0 || length > 64 {
			return newValidationError("Terms version must be 1-64 characters")
		}
	}
	if r.Methods != nil {
		if len(r.Methods) == 0 || hasDuplicates(r.Methods) {
			return newValidationError("Methods must be a non-empty list of distinct methods")
		}
		for _, method := range r.Methods {
			if !method.Valid() {
				return newValidationError(fmt.Sprintf("Unsupported recurring method %q", string(method)))
			}
		}
	}
	if r.Frequency != nil && !recurringFrequencyPattern.MatchString(*r.Frequency) {
		return newValidationError(fmt.Sprintf("Invalid recurring frequency %q", *r.Frequency))
	}
	if r.LimitAmt != nil && *r.LimitAmt < 1 {
		return newValidationError("limit_amt must be at least 1 (minor units)")
	}
	if r.TotLimitAmt != nil && *r.TotLimitAmt < 1 {
		return newValidationError("tot_limit_amt must be at least 1 (minor units)")
	}
	if r.ExpirationDate != nil {
		if err := validateDate(*r.ExpirationDate); err != nil {
			return err
		}
	}
	if r.InitDate != nil {
		if err := validateDate(*r.InitDate); err != nil {
			return err
		}
	}
	return nil
}

func (r RecurringRegistration) validateModel() error {
	switch r.Model {
	case RecurringModelOnDemand:
		for _, field := range []presentField{
			{"frequency", r.Frequency != nil},
			{"limit_amt", r.LimitAmt != nil},
			{"tot_limit_amt", r.TotLimitAmt != nil},
			{"is_limit_amt_fixed", r.LimitAmtFixed != nil},
		} {
			if field.present {
				return newValidationError(field.name + " is not allowed in recurring model O")
			}
		}
	case RecurringModelAutomatic:
		for _, field := range []presentField{
			{"frequency", r.Frequency != nil},
			{"limit_amt", r.LimitAmt != nil},
			{"tot_limit_amt", r.TotLimitAmt != nil},
			{"expiration_date", r.ExpirationDate != nil},
			{"init_date", r.InitDate != nil},
		} {
			if !field.present {
				return newValidationError(field.name + " is required in recurring model A")
			}
		}
		if r.LimitAmtFixed != nil && !*r.LimitAmtFixed {
			return newValidationError("Recurring model A requires a fixed amount (is_limit_amt_fixed = true)")
		}
	}
	return nil
}

// toBody keeps the order of the PHP SDK: label, alias, model, frequency,
// limit_amt, tot_limit_amt, is_limit_amt_fixed, expiration_date, init_date,
// methods, terms_url, terms_version.
func (r RecurringRegistration) toBody() *wire.Body {
	body := wire.NewBody()
	body.Set("label", r.Label)
	body.SetIfNotNil("alias", r.Alias)
	body.Set("model", string(r.Model))
	body.SetIfNotNil("frequency", r.Frequency)
	body.SetIfNotNil("limit_amt", r.LimitAmt)
	body.SetIfNotNil("tot_limit_amt", r.TotLimitAmt)
	body.SetIfNotNil("is_limit_amt_fixed", r.LimitAmtFixed)
	body.SetIfNotNil("expiration_date", r.ExpirationDate)
	body.SetIfNotNil("init_date", r.InitDate)
	if r.Methods != nil {
		methods := make([]string, 0, len(r.Methods))
		for _, method := range r.Methods {
			methods = append(methods, string(method))
		}
		body.Set("methods", methods)
	}
	body.Set("terms_url", r.TermsURL)
	body.SetIfNotNil("terms_version", r.TermsVersion)
	return body
}

func validateRecurringAlias(alias string) error {
	if alias == "" || len(alias) > 128 {
		return newValidationError("Recurring alias must be 1-128 characters")
	}
	return nil
}

func hasDuplicates[T comparable](values []T) bool {
	seen := make(map[T]bool, len(values))
	for _, value := range values {
		if seen[value] {
			return true
		}
		seen[value] = true
	}
	return false
}
