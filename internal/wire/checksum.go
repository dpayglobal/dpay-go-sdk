package wire

import (
	"crypto/sha256"
	"encoding/hex"
	"reflect"
	"sort"
	"strings"

	"github.com/dpayglobal/dpay-go-sdk/internal/php"
)

// Checksum computes the checksum variants the dpay API accepts.
type Checksum struct {
	secret string
}

// NewChecksum returns a Checksum bound to a secret hash.
func NewChecksum(secret string) Checksum {
	return Checksum{secret: secret}
}

// SecretSecond computes sha256 over service, the secret and the given fields,
// joined with a pipe: payment registration, BLIK aliases, recurring payments
// and the Events API.
func (c Checksum) SecretSecond(service string, fields []any) string {
	parts := make([]string, 0, len(fields)+2)
	parts = append(parts, service, c.secret)
	for _, field := range fields {
		parts = append(parts, php.Strval(field))
	}
	return sum(strings.Join(parts, "|"))
}

// OrderedBody computes sha256 over the body values in the order they are sent,
// followed by the secret, joined with a pipe (PBL API: refunds, transaction
// details, banks, payouts). The top-level "checksum" key is skipped, nested
// objects and lists (e.g. webhook) contribute their leaf values in order, and
// every leaf is cast to a string the way the API casts JSON values: nil and
// false give an empty segment, true gives 1.
func (c Checksum) OrderedBody(body *Body) string {
	parts := []string{}
	for _, key := range body.keys {
		if key == "checksum" {
			continue
		}
		parts = appendLeaves(parts, body.values[key])
	}
	return sum(strings.Join(parts, "|") + "|" + c.secret)
}

// Operation computes sha256 over operation|service|transactionID|amount|secret,
// the Cards API capture and cancellation checksum. The operation name keeps a
// capture checksum from authorising a cancellation; an empty amount leaves its
// segment empty.
func (c Checksum) Operation(operation, service, transactionID, amount string) string {
	return sum(strings.Join([]string{operation, service, transactionID, amount, c.secret}, "|"))
}

func appendLeaves(parts []string, value any) []string {
	switch typed := value.(type) {
	case *Body:
		if typed == nil {
			// encoded as null, which the API casts to an empty segment
			return append(parts, "")
		}
		for _, key := range typed.keys {
			parts = appendLeaves(parts, typed.values[key])
		}
		return parts
	case []any:
		for _, item := range typed {
			parts = appendLeaves(parts, item)
		}
		return parts
	case []string:
		return append(parts, typed...)
	case map[string]any:
		// encoding/json sends map keys sorted, so the sorted order is the order sent
		keys := make([]string, 0, len(typed))
		for key := range typed {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			parts = appendLeaves(parts, typed[key])
		}
		return parts
	}
	if value == nil || php.IsScalar(value) {
		return append(parts, php.Strval(value))
	}
	reflected := reflect.ValueOf(value)
	switch reflected.Kind() {
	case reflect.Slice, reflect.Array:
		for index := 0; index < reflected.Len(); index++ {
			parts = appendLeaves(parts, reflected.Index(index).Interface())
		}
		return parts
	case reflect.String:
		return append(parts, reflected.String())
	}
	return append(parts, "")
}

func sum(payload string) string {
	digest := sha256.Sum256([]byte(payload))
	return hex.EncodeToString(digest[:])
}
