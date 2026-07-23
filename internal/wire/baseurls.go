package wire

import "strings"

// Host keys accepted by BaseURLs.Resolve.
const (
	HostAPIPayments = "api_payments"
	HostPanel       = "panel"
	HostGateway     = "gateway"
)

const (
	defaultAPIPayments = "https://api-payments.dpay.pl"
	defaultPanel       = "https://panel.dpay.pl"
	defaultGateway     = "https://secure.dpay.pl"
)

// BaseURLs resolves a host key to a base URL, falling back to production.
type BaseURLs struct {
	urls map[string]string
}

// NewBaseURLs returns BaseURLs where every empty override keeps the production
// default and every provided one is stripped of trailing slashes.
func NewBaseURLs(apiPayments, panel, gateway string) BaseURLs {
	return BaseURLs{urls: map[string]string{
		HostAPIPayments: override(apiPayments, defaultAPIPayments),
		HostPanel:       override(panel, defaultPanel),
		HostGateway:     override(gateway, defaultGateway),
	}}
}

func override(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return strings.TrimRight(value, "/")
}

// Resolve returns the base URL for a host key, or an empty string if the key is unknown.
func (b BaseURLs) Resolve(host string) string {
	return b.urls[host]
}
