package dpay

import (
	"context"
	"fmt"

	"github.com/dpayglobal/dpay-go-sdk/internal/wire"
)

// BlikService manages BLIK aliases and recurring mandates.
type BlikService struct {
	client *Client
}

// UnregisterOption sets an optional field of an alias unregistration.
type UnregisterOption func(*unregisterOptions)

type unregisterOptions struct {
	reason *string
}

// WithUnregisterReason attaches a reason. It is sent in the body but, matching
// the API contract, it does not enter the checksum.
func WithUnregisterReason(reason string) UnregisterOption {
	return func(options *unregisterOptions) { options.reason = &reason }
}

// Alias registers a BLIK alias and returns its state.
func (s *BlikService) Alias(ctx context.Context, aliasValue string, aliasType BlikAliasType) (*BlikAlias, error) {
	if !aliasType.Valid() {
		return nil, newValidationError(fmt.Sprintf("Invalid BLIK alias type %q", string(aliasType)))
	}
	body := wire.NewBody()
	body.Set("service", s.client.service)
	body.Set("alias_value", aliasValue)
	body.Set("alias_type", string(aliasType))
	body.Set("checksum", s.client.checksum.SecretSecond(s.client.service, []any{aliasValue}))

	data, err := s.client.postJSONObject(ctx, hostAPIPayments, "/api/v1_0/payments/blik/aliases", body)
	if err != nil {
		return nil, err
	}
	return blikAliasFromAPI(objectField(data, "data")), nil
}

// UnregisterAlias removes a BLIK alias.
func (s *BlikService) UnregisterAlias(ctx context.Context, aliasValue string, aliasType BlikAliasType, opts ...UnregisterOption) error {
	if !aliasType.Valid() {
		return newValidationError(fmt.Sprintf("Invalid BLIK alias type %q", string(aliasType)))
	}
	options := &unregisterOptions{}
	for _, apply := range opts {
		apply(options)
	}

	body := wire.NewBody()
	body.Set("service", s.client.service)
	body.Set("alias_value", aliasValue)
	body.Set("alias_type", string(aliasType))
	if options.reason != nil {
		body.Set("reason", *options.reason)
	}
	body.Set("checksum", s.client.checksum.SecretSecond(s.client.service, []any{aliasValue}))

	_, err := s.client.postJSONObject(ctx, hostAPIPayments, "/api/v1_0/payments/blik/aliases/unregister", body)
	return err
}

// RecurringStatus reads the state of a BLIK recurring mandate.
func (s *BlikService) RecurringStatus(ctx context.Context, aliasValue string) (*BlikRecurringStatus, error) {
	body := wire.NewBody()
	body.Set("service", s.client.service)
	body.Set("alias_value", aliasValue)
	body.Set("checksum", s.client.checksum.SecretSecond(s.client.service, []any{aliasValue}))

	data, err := s.client.postJSONObject(ctx, hostAPIPayments, "/api/v1_0/payments/blik/recurring/status", body)
	if err != nil {
		return nil, err
	}
	return blikRecurringStatusFromAPI(objectField(data, "data")), nil
}

// BlikAlias is a registered BLIK alias.
type BlikAlias struct {
	raw            map[string]any
	value          string
	aliasType      BlikAliasType
	status         string
	expirationDate string
	apps           []*BlikApp
}

func blikAliasFromAPI(data map[string]any) *BlikAlias {
	aliasType := stringField(data, "alias_type")
	if aliasType == "" {
		aliasType = string(BlikAliasTypeUID)
	}
	alias := &BlikAlias{
		raw:            data,
		value:          stringField(data, "alias_value"),
		aliasType:      BlikAliasType(aliasType),
		status:         stringField(data, "status"),
		expirationDate: stringField(data, "expiration_date"),
	}
	for _, entry := range arrayField(data, "apps") {
		if app, ok := entry.(map[string]any); ok {
			alias.apps = append(alias.apps, &BlikApp{key: stringField(app, "key"), label: stringField(app, "label")})
		}
	}
	return alias
}

// Value returns the alias value.
func (a *BlikAlias) Value() string { return a.value }

// Type returns the alias type.
func (a *BlikAlias) Type() BlikAliasType { return a.aliasType }

// Status returns the alias status as reported by the API.
func (a *BlikAlias) Status() string { return a.status }

// IsActive reports whether the alias status is ACTIVE.
func (a *BlikAlias) IsActive() bool { return a.status == "ACTIVE" }

// ExpirationDate returns when the alias expires.
func (a *BlikAlias) ExpirationDate() string { return a.expirationDate }

// Apps returns the banking apps the alias is registered in.
func (a *BlikAlias) Apps() []*BlikApp { return a.apps }

// Raw returns the decoded data object.
func (a *BlikAlias) Raw() map[string]any { return a.raw }

// BlikApp is a banking app an alias is registered in.
type BlikApp struct {
	key   string
	label string
}

// Key returns the app identifier.
func (a *BlikApp) Key() string { return a.key }

// Label returns the app display name.
func (a *BlikApp) Label() string { return a.label }

// BlikRecurringStatus is the state of a BLIK recurring mandate.
type BlikRecurringStatus struct {
	raw            map[string]any
	value          string
	aliasType      BlikAliasType
	status         string
	expirationDate string
	registration   *BlikRecurringInfo
}

func blikRecurringStatusFromAPI(data map[string]any) *BlikRecurringStatus {
	aliasType := stringField(data, "alias_type")
	if aliasType == "" {
		aliasType = string(BlikAliasTypePayID)
	}
	status := &BlikRecurringStatus{
		raw:            data,
		value:          stringField(data, "alias_value"),
		aliasType:      BlikAliasType(aliasType),
		status:         stringField(data, "status"),
		expirationDate: stringField(data, "expiration_date"),
	}
	if registration, ok := data["registration"].(map[string]any); ok {
		status.registration = &BlikRecurringInfo{raw: registration}
	}
	return status
}

// Value returns the alias value.
func (s *BlikRecurringStatus) Value() string { return s.value }

// Type returns the alias type.
func (s *BlikRecurringStatus) Type() BlikAliasType { return s.aliasType }

// Status returns the mandate status as reported by the API.
func (s *BlikRecurringStatus) Status() string { return s.status }

// IsActive reports whether the mandate status is ACTIVE.
func (s *BlikRecurringStatus) IsActive() bool { return s.status == "ACTIVE" }

// ExpirationDate returns when the mandate expires.
func (s *BlikRecurringStatus) ExpirationDate() string { return s.expirationDate }

// Registration returns the mandate terms, nil when the API sent none.
func (s *BlikRecurringStatus) Registration() *BlikRecurringInfo { return s.registration }

// Raw returns the decoded data object.
func (s *BlikRecurringStatus) Raw() map[string]any { return s.raw }

// BlikRecurringInfo carries the terms of a registered BLIK mandate.
type BlikRecurringInfo struct {
	raw map[string]any
}

// Model returns the mandate model.
func (i *BlikRecurringInfo) Model() string { return stringField(i.raw, "model") }

// Frequency returns the charge interval.
func (i *BlikRecurringInfo) Frequency() string { return stringField(i.raw, "frequency") }

// LimitAmt returns the per-charge limit in minor units, nil when absent.
func (i *BlikRecurringInfo) LimitAmt() *int64 { return optionalInt(i.raw, "limit_amt") }

// TotLimitAmt returns the total limit in minor units, nil when absent.
func (i *BlikRecurringInfo) TotLimitAmt() *int64 { return optionalInt(i.raw, "tot_limit_amt") }

// IsLimitAmtFixed reports whether the limit is fixed, nil when absent.
func (i *BlikRecurringInfo) IsLimitAmtFixed() *bool {
	if value, ok := i.raw["is_limit_amt_fixed"].(bool); ok {
		return &value
	}
	return nil
}

// InitDate returns the first charge date.
func (i *BlikRecurringInfo) InitDate() string { return stringField(i.raw, "init_date") }

// Label returns the label shown to the payer.
func (i *BlikRecurringInfo) Label() string { return stringField(i.raw, "label") }

// RegisteredAt returns when the mandate was registered.
func (i *BlikRecurringInfo) RegisteredAt() string { return stringField(i.raw, "registered_at") }

// Raw returns the decoded registration object.
func (i *BlikRecurringInfo) Raw() map[string]any { return i.raw }
