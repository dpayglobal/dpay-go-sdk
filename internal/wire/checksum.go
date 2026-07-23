package wire

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"

	"github.com/dpayglobal/dpay-go-sdk/internal/php"
)

// Checksum computes the two checksum variants the dpay API accepts.
type Checksum struct {
	secret string
}

// NewChecksum returns a Checksum bound to a secret hash.
func NewChecksum(secret string) Checksum {
	return Checksum{secret: secret}
}

// SecretSecond computes sha256 over service, the secret and the given fields,
// joined with a pipe.
func (c Checksum) SecretSecond(service string, fields []any) string {
	parts := make([]string, 0, len(fields)+2)
	parts = append(parts, service, c.secret)
	for _, field := range fields {
		parts = append(parts, php.Strval(field))
	}
	return sum(strings.Join(parts, "|"))
}

// OrderedBody computes sha256 over the body values in insertion order followed
// by the secret, joined with a pipe.
func (c Checksum) OrderedBody(values []any) string {
	parts := make([]string, 0, len(values))
	for _, value := range values {
		parts = append(parts, php.Strval(value))
	}
	return sum(strings.Join(parts, "|") + "|" + c.secret)
}

func sum(payload string) string {
	digest := sha256.Sum256([]byte(payload))
	return hex.EncodeToString(digest[:])
}
