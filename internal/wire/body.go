// Package wire carries the dpay protocol mechanics: ordered request bodies,
// checksums and API host resolution.
package wire

import (
	"bytes"
	"reflect"

	"github.com/dpayglobal/dpay-go-sdk/internal/php"
)

// Body is an ordered JSON object. Insertion order is part of the protocol
// because the ordered-body checksum is computed from the values in that order.
type Body struct {
	keys   []string
	values map[string]any
}

// NewBody returns an empty Body.
func NewBody() *Body {
	return &Body{values: map[string]any{}}
}

// Set stores value under key, appending the key on first use and keeping its
// original position on overwrite.
func (b *Body) Set(key string, value any) {
	if _, exists := b.values[key]; !exists {
		b.keys = append(b.keys, key)
	}
	b.values[key] = value
}

// SetIfNotNil stores value under key unless value is nil or a nil pointer,
// dereferencing pointers so that the wire format carries the pointed-to value.
func (b *Body) SetIfNotNil(key string, value any) {
	if value == nil {
		return
	}
	reflected := reflect.ValueOf(value)
	if reflected.Kind() == reflect.Ptr {
		if reflected.IsNil() {
			return
		}
		b.Set(key, reflected.Elem().Interface())
		return
	}
	b.Set(key, value)
}

// Get returns the value stored under key.
func (b *Body) Get(key string) (any, bool) {
	value, exists := b.values[key]
	return value, exists
}

// Keys returns the keys in insertion order.
func (b *Body) Keys() []string {
	return append([]string(nil), b.keys...)
}

// Values returns the values in insertion order, which is the input to the
// ordered-body checksum.
func (b *Body) Values() []any {
	values := make([]any, 0, len(b.keys))
	for _, key := range b.keys {
		values = append(values, b.values[key])
	}
	return values
}

// Len returns the number of entries.
func (b *Body) Len() int {
	return len(b.keys)
}

// MarshalJSON serializes the body in insertion order using PHP encoding rules.
func (b *Body) MarshalJSON() ([]byte, error) {
	var buffer bytes.Buffer
	buffer.WriteByte('{')
	for index, key := range b.keys {
		if index > 0 {
			buffer.WriteByte(',')
		}
		encodedKey, err := php.Encode(key, false)
		if err != nil {
			return nil, err
		}
		buffer.Write(encodedKey)
		buffer.WriteByte(':')
		encodedValue, err := php.Encode(b.values[key], false)
		if err != nil {
			return nil, err
		}
		buffer.Write(encodedValue)
	}
	buffer.WriteByte('}')
	return buffer.Bytes(), nil
}
