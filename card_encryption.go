package dpay

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"strings"
	"time"

	"github.com/dpayglobal/dpay-go-sdk/internal/php"
	"github.com/dpayglobal/dpay-go-sdk/internal/wire"
)

// CardData is the raw card the payer entered. It never leaves the process
// unencrypted: pass it to EncryptCard and send only the result.
type CardData struct {
	// PAN is the card number, 12-19 digits. Spaces are stripped.
	PAN string
	// CVV is the security code, 3-4 digits.
	CVV string
	// Expiry is the expiry date in MM/YY format.
	Expiry string
}

// Normalized returns the card with spaces stripped from the PAN.
func (c CardData) Normalized() CardData {
	return CardData{PAN: strings.ReplaceAll(c.PAN, " ", ""), CVV: c.CVV, Expiry: c.Expiry}
}

// Validate reports whether the card number, security code and expiry are well formed.
func (c CardData) Validate() error {
	normalized := c.Normalized()
	if !panPattern.MatchString(normalized.PAN) {
		return newValidationError("Card number must be 12-19 digits")
	}
	if !cvvPattern.MatchString(normalized.CVV) {
		return newValidationError("CVV must be 3-4 digits")
	}
	if !expiryPattern.MatchString(normalized.Expiry) {
		return newValidationError("Expiry must be in MM/YY format")
	}
	return nil
}

// EncryptCard encrypts card data for a transaction with the public key fetched
// from Cards.PublicKey. dpay rotates that key, so fetch it before every attempt.
func EncryptCard(card CardData, transactionID, publicKeyPEM string) (string, error) {
	if err := card.Validate(); err != nil {
		return "", err
	}
	key, err := parseRSAPublicKey(publicKeyPEM)
	if err != nil {
		return "", err
	}
	payload, err := cardPayload(card, transactionID, time.Now().Unix())
	if err != nil {
		return "", &CardEncryptionError{message: "Unable to encode card payload", cause: err}
	}
	ciphertext, err := rsa.EncryptPKCS1v15(rand.Reader, key, payload)
	if err != nil {
		return "", &CardEncryptionError{message: "Card data encryption failed", cause: err}
	}
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

func cardPayload(card CardData, transactionID string, now int64) ([]byte, error) {
	normalized := card.Normalized()
	body := wire.NewBody()
	body.Set("PN", normalized.PAN)
	body.Set("SC", normalized.CVV)
	body.Set("DT", normalized.Expiry)
	body.Set("ID", transactionID)
	body.Set("TX", now)

	encoded := make([]byte, 0, 128)
	encoded = append(encoded, '{')
	for index, key := range body.Keys() {
		if index > 0 {
			encoded = append(encoded, ',')
		}
		encodedKey, err := php.Encode(key, true)
		if err != nil {
			return nil, err
		}
		value, _ := body.Get(key)
		encodedValue, err := php.Encode(value, true)
		if err != nil {
			return nil, err
		}
		encoded = append(encoded, encodedKey...)
		encoded = append(encoded, ':')
		encoded = append(encoded, encodedValue...)
	}
	return append(encoded, '}'), nil
}

func parseRSAPublicKey(publicKeyPEM string) (*rsa.PublicKey, error) {
	block, _ := pem.Decode([]byte(strings.TrimSpace(publicKeyPEM)))
	if block == nil {
		return nil, &CardEncryptionError{message: "Invalid RSA public key"}
	}
	if parsed, err := x509.ParsePKIXPublicKey(block.Bytes); err == nil {
		if key, ok := parsed.(*rsa.PublicKey); ok {
			return key, nil
		}
		return nil, &CardEncryptionError{message: "Invalid RSA public key"}
	}
	key, err := x509.ParsePKCS1PublicKey(block.Bytes)
	if err != nil {
		return nil, &CardEncryptionError{message: "Invalid RSA public key"}
	}
	return key, nil
}
