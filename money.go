package dpay

import (
	"fmt"
	"regexp"
	"strconv"

	"github.com/dpayglobal/dpay-go-sdk/internal/php"
)

// Currency is an ISO 4217 alphabetic currency code.
type Currency string

// Currencies used by the dpay API.
const (
	CurrencyPLN Currency = "PLN"
	CurrencyEUR Currency = "EUR"
	CurrencyCZK Currency = "CZK"
)

var currencyPattern = regexp.MustCompile(`^[A-Z]{3}$`)

// Valid reports whether the code consists of exactly three uppercase letters.
func (c Currency) Valid() bool {
	return currencyPattern.MatchString(string(c))
}

var decimalPattern = regexp.MustCompile(`^(-?)(\d+)(?:\.(\d{1,2}))?$`)

// Money is an amount in minor units together with its currency.
type Money struct {
	minor    int64
	currency Currency
}

// PLN returns an amount in Polish grosz.
func PLN(minor int64) Money {
	return Money{minor: minor, currency: CurrencyPLN}
}

// NewMoney returns an amount in minor units of the given currency.
func NewMoney(minor int64, currency Currency) (Money, error) {
	if !currency.Valid() {
		return Money{}, newValidationError(fmt.Sprintf("Invalid currency code %q", string(currency)))
	}
	return Money{minor: minor, currency: currency}, nil
}

// ParseMoney parses a decimal string with at most two fraction digits.
func ParseMoney(decimal string, currency Currency) (Money, error) {
	if !currency.Valid() {
		return Money{}, newValidationError(fmt.Sprintf("Invalid currency code %q", string(currency)))
	}
	minor, ok := parseDecimal(decimal)
	if !ok {
		return Money{}, newValidationError(fmt.Sprintf("Invalid money amount %q", decimal))
	}
	return Money{minor: minor, currency: currency}, nil
}

func parseDecimal(decimal string) (int64, bool) {
	matches := decimalPattern.FindStringSubmatch(decimal)
	if matches == nil {
		return 0, false
	}
	fraction := matches[3]
	for len(fraction) < 2 {
		fraction += "0"
	}
	units, err := strconv.ParseInt(matches[2], 10, 64)
	if err != nil {
		return 0, false
	}
	cents, err := strconv.ParseInt(fraction, 10, 64)
	if err != nil {
		return 0, false
	}
	minor := units*100 + cents
	if matches[1] == "-" {
		minor = -minor
	}
	return minor, true
}

// Minor returns the amount in minor units.
func (m Money) Minor() int64 {
	return m.minor
}

// Currency returns the currency code.
func (m Money) Currency() Currency {
	return m.currency
}

// String returns the amount as a decimal with exactly two fraction digits.
func (m Money) String() string {
	minor := m.minor
	sign := ""
	if minor < 0 {
		sign = "-"
		minor = -minor
	}
	return fmt.Sprintf("%s%d.%02d", sign, minor/100, minor%100)
}

// IsNegative reports whether the amount is below zero.
func (m Money) IsNegative() bool {
	return m.minor < 0
}

// IsZero reports whether the amount is zero.
func (m Money) IsZero() bool {
	return m.minor == 0
}

// Equal reports whether both the amount and the currency match.
func (m Money) Equal(other Money) bool {
	return m.minor == other.minor && m.currency == other.currency
}

func moneyFromAPI(value any, currency Currency) (Money, bool) {
	if !currency.Valid() {
		return Money{}, false
	}
	switch typed := value.(type) {
	case int:
		return Money{minor: int64(typed) * 100, currency: currency}, true
	case int64:
		return Money{minor: typed * 100, currency: currency}, true
	case float64:
		return Money{minor: php.Round(typed * 100), currency: currency}, true
	case string:
		minor, ok := parseDecimal(typed)
		if !ok {
			return Money{}, false
		}
		return Money{minor: minor, currency: currency}, true
	}
	return Money{}, false
}

func moneyFloat(m Money) float64 {
	parsed, err := strconv.ParseFloat(m.String(), 64)
	if err != nil {
		return 0
	}
	return parsed
}
