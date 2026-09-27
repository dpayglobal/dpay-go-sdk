package dpay

import (
	"context"
	"encoding/base64"
	"strings"

	"github.com/dpayglobal/dpay-go-sdk/internal/wire"
)

// CardService runs server-to-server card operations.
type CardService struct {
	client *Client
}

// PublicKey fetches the RSA key used to encrypt card data. dpay rotates it, so
// fetch it before every payment attempt rather than caching it.
func (s *CardService) PublicKey(ctx context.Context) (string, error) {
	key, err := s.client.getText(ctx, hostAPIPayments, "/api/v1_0/cards/public-key")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(key), nil
}

// CardPaymentRequest carries a card payment or pre-authorization.
type CardPaymentRequest struct {
	// DeviceInfo describes the payer browser and is always sent.
	DeviceInfo DeviceInfo
	// Email is the payer address.
	Email *string
	// ChannelID selects the acquiring channel.
	ChannelID *int
	// CardHolderFirstName is the name printed on the card.
	CardHolderFirstName *string
	// CardHolderLastName is the surname printed on the card.
	CardHolderLastName *string
	// EncryptedCardData is the result of EncryptCard.
	EncryptedCardData *string
	// ThreeDSConfirmed marks the 3-D Secure challenge as completed.
	ThreeDSConfirmed *bool
	// DCCDecision answers a dynamic currency conversion offer.
	DCCDecision DCCDecision
}

// Validate reports whether the device info and DCC decision are acceptable.
func (r *CardPaymentRequest) Validate() error {
	if err := r.DeviceInfo.Validate(); err != nil {
		return err
	}
	if r.DCCDecision != "" && !r.DCCDecision.Valid() {
		return newValidationError(`Invalid DCC decision "` + string(r.DCCDecision) + `"`)
	}
	return nil
}

func (r *CardPaymentRequest) toBody() *wire.Body {
	body := wire.NewBody()
	body.SetIfNotNil("email", r.Email)
	body.SetIfNotNil("channelId", r.ChannelID)
	body.SetIfNotNil("cardHolderFirstName", r.CardHolderFirstName)
	body.SetIfNotNil("cardHolderLastName", r.CardHolderLastName)
	body.SetIfNotNil("encryptedCardData", r.EncryptedCardData)
	body.Set("deviceInfo", r.DeviceInfo.toBody())
	body.SetIfNotNil("threeDsConfirmed", r.ThreeDSConfirmed)
	if r.DCCDecision != "" {
		body.Set("dccDecision", string(r.DCCDecision))
	}
	return body
}

// GooglePayRequest carries a Google Pay token.
type GooglePayRequest struct {
	// Token is the Google Pay payment token.
	Token string
	// DeviceInfo describes the payer browser.
	DeviceInfo DeviceInfo
	// Email is the payer address.
	Email *string
	// ChannelID selects the acquiring channel.
	ChannelID *int
}

// Validate reports whether the device info is acceptable.
func (r *GooglePayRequest) Validate() error {
	return r.DeviceInfo.Validate()
}

func (r *GooglePayRequest) toBody() *wire.Body {
	body := wire.NewBody()
	body.SetIfNotNil("email", r.Email)
	body.SetIfNotNil("channelId", r.ChannelID)
	body.Set("xPayType", "GOOGLE_PAY")
	body.Set("xPayToken", r.Token)
	body.Set("deviceInfo", r.DeviceInfo.toBody())
	return body
}

// ApplePayRequest starts an Apple Pay session or pays with its token.
type ApplePayRequest struct {
	// DeviceInfo describes the payer browser.
	DeviceInfo DeviceInfo
	// Token is the Apple Pay payment token, ignored when Init is set.
	Token *string
	// ChannelID selects the acquiring channel.
	ChannelID *int
	// Init requests a session instead of a payment.
	Init bool
}

// Validate reports whether the device info is acceptable.
func (r *ApplePayRequest) Validate() error {
	return r.DeviceInfo.Validate()
}

func (r *ApplePayRequest) toBody() *wire.Body {
	body := wire.NewBody()
	body.SetIfNotNil("channelId", r.ChannelID)
	if r.Init {
		body.Set("xPayType", "APPLE_PAY_INIT")
	} else {
		body.Set("xPayType", "APPLE_PAY")
		body.SetIfNotNil("xPayToken", r.Token)
	}
	body.Set("deviceInfo", r.DeviceInfo.toBody())
	return body
}

// PayOTP charges an encrypted card.
func (s *CardService) PayOTP(ctx context.Context, transactionID string, request *CardPaymentRequest) (*CardPaymentResult, error) {
	if err := request.Validate(); err != nil {
		return nil, err
	}
	return s.post(ctx, transactionID, "/pay/card-otp", request.toBody())
}

// PreAuth authorizes an encrypted card without capturing.
func (s *CardService) PreAuth(ctx context.Context, transactionID string, request *CardPaymentRequest) (*CardPaymentResult, error) {
	if err := request.Validate(); err != nil {
		return nil, err
	}
	return s.post(ctx, transactionID, "/pay/card-pre-auth", request.toBody())
}

// CaptureOption sets an optional field of a card capture.
type CaptureOption func(*captureOptions)

type captureOptions struct {
	webhook *WebhookTarget
}

// WithCaptureWebhook sends the payment.captured event of this capture also to
// target, signed with the webhook secret of the service. It does not enter the
// checksum and allows only WebhookEventTypePaymentCaptured.
func WithCaptureWebhook(target WebhookTarget) CaptureOption {
	return func(options *captureOptions) { options.webhook = &target }
}

// Capture settles a pre-authorization; partial captures are allowed up to the
// authorized amount. The request is signed with
// sha256(capture|service|transaction_id|amount|hash).
func (s *CardService) Capture(ctx context.Context, transactionID string, amount Money, opts ...CaptureOption) (*CardPaymentResult, error) {
	options := &captureOptions{}
	for _, apply := range opts {
		apply(options)
	}

	body := wire.NewBody()
	body.Set("service", s.client.service)
	body.Set("amount", moneyFloat(amount))
	if options.webhook != nil {
		if err := options.webhook.validateFor(CaptureEventTypes(), "a card capture"); err != nil {
			return nil, err
		}
		body.Set("webhook", options.webhook.toBody())
	}
	body.Set("checksum", s.client.checksum.Operation("capture", s.client.service, transactionID, amount.String()))
	return s.post(ctx, transactionID, "/capture", body)
}

// Cancel voids a pre-authorization; a nil amount cancels the whole uncaptured
// remainder. The request is signed with
// sha256(cancellation|service|transaction_id|amount|hash), with an empty amount
// segment for a nil amount.
func (s *CardService) Cancel(ctx context.Context, transactionID string, amount *Money) (*CardPaymentResult, error) {
	body := wire.NewBody()
	body.Set("service", s.client.service)
	decimal := ""
	if amount != nil {
		body.Set("amount", moneyFloat(*amount))
		decimal = amount.String()
	}
	body.Set("checksum", s.client.checksum.Operation("cancellation", s.client.service, transactionID, decimal))
	return s.post(ctx, transactionID, "/cancellation", body)
}

// GooglePay charges a Google Pay token.
func (s *CardService) GooglePay(ctx context.Context, transactionID string, request *GooglePayRequest) (*CardPaymentResult, error) {
	if err := request.Validate(); err != nil {
		return nil, err
	}
	return s.post(ctx, transactionID, "/pay/google-pay", request.toBody())
}

// ApplePay starts an Apple Pay session or charges its token.
func (s *CardService) ApplePay(ctx context.Context, transactionID string, request *ApplePayRequest) (*CardPaymentResult, error) {
	if err := request.Validate(); err != nil {
		return nil, err
	}
	return s.post(ctx, transactionID, "/pay/apple-pay", request.toBody())
}

func (s *CardService) post(ctx context.Context, transactionID, suffix string, body *wire.Body) (*CardPaymentResult, error) {
	path := "/api/v1_0/cards/payment/" + escapePathSegment(transactionID) + suffix
	data, err := s.client.postJSONObject(ctx, hostAPIPayments, path, body)
	if err != nil {
		return nil, err
	}
	if success, ok := data["success"].(bool); !ok || !success {
		return nil, cardPaymentErrorFromAPI(data)
	}
	return cardPaymentResultFromAPI(data), nil
}

func cardPaymentErrorFromAPI(data map[string]any) *CardPaymentError {
	message := "Card payment failed"
	if value, ok := data["message"].(string); ok {
		message = value
	}
	return &CardPaymentError{APIError: &APIError{
		kind:        ErrCardPayment,
		message:     message,
		HTTPStatus:  200,
		ErrorCode:   message,
		FieldErrors: map[string][]string{},
		RawBody:     marshalRaw(data),
	}}
}

func escapePathSegment(value string) string {
	const unreserved = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789-_.~"
	const hexDigits = "0123456789ABCDEF"
	var escaped strings.Builder
	for index := 0; index < len(value); index++ {
		character := value[index]
		if strings.IndexByte(unreserved, character) >= 0 {
			escaped.WriteByte(character)
			continue
		}
		escaped.WriteByte('%')
		escaped.WriteByte(hexDigits[character>>4])
		escaped.WriteByte(hexDigits[character&0x0F])
	}
	return escaped.String()
}

// CardPaymentResult is the outcome of a card operation.
type CardPaymentResult struct {
	raw          map[string]any
	redirectType RedirectType
	redirectText string
	dccOffer     *DCCOffer
}

func cardPaymentResultFromAPI(data map[string]any) *CardPaymentResult {
	message := objectField(data, "message")
	result := &CardPaymentResult{
		raw:          data,
		redirectType: RedirectType(stringField(message, "redirectType")),
		redirectText: stringField(message, "redirectText"),
	}
	if offer, ok := message["dccOffer"].(map[string]any); ok {
		result.dccOffer = dccOfferFromAPI(offer)
	}
	return result
}

// RedirectType returns what the merchant should do next.
func (r *CardPaymentResult) RedirectType() RedirectType { return r.redirectType }

// IsSuccess reports whether the payment finished without further steps.
func (r *CardPaymentResult) IsSuccess() bool { return r.redirectType == RedirectTypeSuccess }

// RequiresThreeDSForm reports whether a 3-D Secure form must be rendered.
func (r *CardPaymentResult) RequiresThreeDSForm() bool { return r.redirectType == RedirectTypeForm }

// RequiresRedirect reports whether the payer must be redirected.
func (r *CardPaymentResult) RequiresRedirect() bool { return r.redirectType == RedirectTypeURL }

// HasDCCOffer reports whether the payer must answer a currency conversion offer.
func (r *CardPaymentResult) HasDCCOffer() bool { return r.redirectType == RedirectTypeDCCOffer }

// ThreeDSFormHTML returns the decoded 3-D Secure form, empty when there is none.
func (r *CardPaymentResult) ThreeDSFormHTML() string {
	if !r.RequiresThreeDSForm() {
		return ""
	}
	return decodeBase64(r.redirectText)
}

// RedirectURL returns the decoded redirect target, empty when there is none.
func (r *CardPaymentResult) RedirectURL() string {
	if !r.RequiresRedirect() {
		return ""
	}
	return decodeBase64(r.redirectText)
}

// DCCOffer returns the currency conversion offer, nil when there is none.
func (r *CardPaymentResult) DCCOffer() *DCCOffer { return r.dccOffer }

// Raw returns the decoded response body.
func (r *CardPaymentResult) Raw() map[string]any { return r.raw }

func decodeBase64(value string) string {
	if value == "" {
		return ""
	}
	decoded, err := base64.StdEncoding.DecodeString(value)
	if err != nil {
		return ""
	}
	return string(decoded)
}

// DCCOffer is a dynamic currency conversion offer the payer must accept or reject.
type DCCOffer struct {
	raw                  map[string]any
	currencyConversionID string
	originalAmount       Money
	convertedAmount      Money
	exchangeRate         float64
	validUntil           string
	declarationText      string
	markup               []*DCCMarkup
	europeanEconomicArea bool
}

func dccOfferFromAPI(data map[string]any) *DCCOffer {
	offer := &DCCOffer{
		raw:                  data,
		currencyConversionID: stringField(data, "currencyConversionId"),
		originalAmount:       moneyField(data, "originalAmount", currencyOr(data, "originalCurrency")),
		convertedAmount:      moneyField(data, "convertedAmount", currencyOr(data, "convertedCurrency")),
		validUntil:           stringField(data, "validUntil"),
		declarationText:      stringField(data, "declarationText"),
		europeanEconomicArea: boolField(data, "europeanEconomicArea"),
	}
	if rate, ok := data["exchangeRate"].(float64); ok {
		offer.exchangeRate = rate
	}
	for _, entry := range arrayField(data, "markup") {
		if markup, ok := entry.(map[string]any); ok {
			single := &DCCMarkup{additionalInfo: stringField(markup, "additionalInfo")}
			if rate, ok := markup["rate"].(float64); ok {
				single.rate = rate
			}
			offer.markup = append(offer.markup, single)
		}
	}
	return offer
}

func currencyOr(data map[string]any, key string) Currency {
	if value, ok := data[key].(string); ok && Currency(value).Valid() {
		return Currency(value)
	}
	return CurrencyPLN
}

// CurrencyConversionID identifies the offer.
func (o *DCCOffer) CurrencyConversionID() string { return o.currencyConversionID }

// OriginalAmount returns the amount in the transaction currency.
func (o *DCCOffer) OriginalAmount() Money { return o.originalAmount }

// ConvertedAmount returns the amount in the cardholder currency.
func (o *DCCOffer) ConvertedAmount() Money { return o.convertedAmount }

// ExchangeRate returns the offered rate.
func (o *DCCOffer) ExchangeRate() float64 { return o.exchangeRate }

// ValidUntil returns when the offer expires.
func (o *DCCOffer) ValidUntil() string { return o.validUntil }

// DeclarationText returns the PSD2 disclosure that must be shown to the payer.
func (o *DCCOffer) DeclarationText() string { return o.declarationText }

// Markup returns the margin components of the rate.
func (o *DCCOffer) Markup() []*DCCMarkup { return o.markup }

// IsEuropeanEconomicArea reports whether the card was issued in the EEA.
func (o *DCCOffer) IsEuropeanEconomicArea() bool { return o.europeanEconomicArea }

// Raw returns the decoded offer object.
func (o *DCCOffer) Raw() map[string]any { return o.raw }

// DCCMarkup is a single margin component of a conversion rate.
type DCCMarkup struct {
	rate           float64
	additionalInfo string
}

// Rate returns the margin rate.
func (m *DCCMarkup) Rate() float64 { return m.rate }

// AdditionalInfo returns the explanation of this margin component.
func (m *DCCMarkup) AdditionalInfo() string { return m.additionalInfo }
