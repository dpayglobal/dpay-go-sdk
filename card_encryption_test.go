package dpay

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"strings"
	"testing"
)

func testKeyPEM(t *testing.T, pkcs1 bool) (*rsa.PrivateKey, string) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	if pkcs1 {
		encoded := pem.EncodeToMemory(&pem.Block{
			Type:  "RSA PUBLIC KEY",
			Bytes: x509.MarshalPKCS1PublicKey(&key.PublicKey),
		})
		return key, string(encoded)
	}
	der, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	if err != nil {
		t.Fatal(err)
	}
	return key, string(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: der}))
}

func TestCardDataValidate(t *testing.T) {
	valid := CardData{PAN: "4111 1111 1111 1111", CVV: "123", Expiry: "12/25"}
	if err := valid.Validate(); err != nil {
		t.Fatalf("valid card rejected: %v", err)
	}
	if valid.Normalized().PAN != "4111111111111111" {
		t.Fatal("spaces must be stripped from the PAN")
	}
	short := CardData{PAN: "411111111", CVV: "123", Expiry: "12/25"}
	if err := short.Validate(); err == nil || err.Error() != "dpay: Card number must be 12-19 digits" {
		t.Fatalf("message = %v", err)
	}
	long := CardData{PAN: "41111111111111111111", CVV: "123", Expiry: "12/25"}
	if err := long.Validate(); err == nil {
		t.Fatal("20 digits must fail")
	}
	badCVV := CardData{PAN: "4111111111111111", CVV: "12", Expiry: "12/25"}
	if err := badCVV.Validate(); err == nil || err.Error() != "dpay: CVV must be 3-4 digits" {
		t.Fatalf("message = %v", err)
	}
	for _, expiry := range []string{"13/25", "00/25", "1/25", "12/2025", "12-25"} {
		bad := CardData{PAN: "4111111111111111", CVV: "123", Expiry: expiry}
		if err := bad.Validate(); err == nil || err.Error() != "dpay: Expiry must be in MM/YY format" {
			t.Errorf("expiry %q: %v", expiry, err)
		}
	}
}

func TestCardPayloadMatchesPHP(t *testing.T) {
	payload, err := cardPayload(CardData{PAN: "4111111111111111", CVV: "123", Expiry: "12/25"}, "tx-1", 1784700000)
	if err != nil {
		t.Fatal(err)
	}
	want := `{"PN":"4111111111111111","SC":"123","DT":"12\/25","ID":"tx-1","TX":1784700000}`
	if string(payload) != want {
		t.Fatalf("cardPayload = %s\nwant       = %s", payload, want)
	}
}

func TestCardPayloadEscapesNonASCII(t *testing.T) {
	payload, err := cardPayload(CardData{PAN: "4111111111111111", CVV: "123", Expiry: "12/25"}, "tx-ą", 1)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(payload), "\"ID\":\"tx-\\u0105\"") {
		t.Fatalf("cardPayload = %s", payload)
	}
}

func TestEncryptCardRoundTrip(t *testing.T) {
	for _, pkcs1 := range []bool{false, true} {
		key, publicPEM := testKeyPEM(t, pkcs1)
		encrypted, err := EncryptCard(CardData{PAN: "4111 1111 1111 1111", CVV: "123", Expiry: "12/25"}, "tx-1", publicPEM)
		if err != nil {
			t.Fatalf("pkcs1=%v: %v", pkcs1, err)
		}
		ciphertext, err := base64.StdEncoding.DecodeString(encrypted)
		if err != nil {
			t.Fatalf("result must be standard base64: %v", err)
		}
		decrypted, err := rsa.DecryptPKCS1v15(rand.Reader, key, ciphertext)
		if err != nil {
			t.Fatalf("must be PKCS#1 v1.5: %v", err)
		}
		text := string(decrypted)
		for _, fragment := range []string{`"PN":"4111111111111111"`, `"SC":"123"`, `"DT":"12\/25"`, `"ID":"tx-1"`, `"TX":`} {
			if !strings.Contains(text, fragment) {
				t.Fatalf("decrypted payload missing %s: %s", fragment, text)
			}
		}
	}
}

func TestEncryptCardRejectsBadKey(t *testing.T) {
	_, err := EncryptCard(CardData{PAN: "4111111111111111", CVV: "123", Expiry: "12/25"}, "tx-1", "not a key")
	if err == nil || err.Error() != "dpay: Invalid RSA public key" {
		t.Fatalf("err = %v", err)
	}
}

func TestEncryptCardValidatesCard(t *testing.T) {
	_, publicPEM := testKeyPEM(t, false)
	_, err := EncryptCard(CardData{PAN: "1", CVV: "123", Expiry: "12/25"}, "tx-1", publicPEM)
	if err == nil || err.Error() != "dpay: Card number must be 12-19 digits" {
		t.Fatalf("err = %v", err)
	}
}
