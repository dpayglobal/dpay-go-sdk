// Package php reproduces the PHP semantics that the dpay wire protocol depends on.
package php

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"math"
	"strconv"
	"strings"
	"unicode/utf16"
	"unicode/utf8"
)

// Trim strips the characters PHP trim removes by default: space, tab, line feed,
// carriage return, NUL and vertical tab. strings.TrimSpace differs on NUL and on
// Unicode spaces.
func Trim(value string) string {
	return strings.Trim(value, " \t\n\r\x00\x0B")
}

// Base64DecodeStrict decodes value the way PHP base64_decode($value, true) does:
// spaces, tabs and line breaks are skipped, missing padding is accepted, while
// characters outside the alphabet, data after padding, a truncated last group and
// wrong padding are rejected.
func Base64DecodeStrict(value string) ([]byte, bool) {
	data := make([]byte, 0, len(value))
	padding := 0
	for index := 0; index < len(value); index++ {
		character := value[index]
		switch {
		case character == '=':
			padding++
		case character == ' ' || character == '\t' || character == '\n' || character == '\r':
		case isBase64Character(character):
			if padding > 0 {
				return nil, false
			}
			data = append(data, character)
		default:
			return nil, false
		}
	}
	if len(data)%4 == 1 {
		return nil, false
	}
	if padding > 0 && (padding > 2 || (len(data)+padding)%4 != 0) {
		return nil, false
	}
	decoded, err := base64.RawStdEncoding.DecodeString(string(data))
	if err != nil {
		return nil, false
	}
	return decoded, true
}

func isBase64Character(character byte) bool {
	return character >= 'A' && character <= 'Z' || character >= 'a' && character <= 'z' ||
		character >= '0' && character <= '9' || character == '+' || character == '/'
}

// IsScalar reports whether v is an int, float, bool or string, matching PHP is_scalar.
func IsScalar(v any) bool {
	switch v.(type) {
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64,
		float32, float64, bool, string:
		return true
	}
	return false
}

// IsNumeric reports whether v is a number or a numeric string, matching PHP is_numeric.
func IsNumeric(v any) bool {
	switch value := v.(type) {
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64, float32, float64:
		return true
	case string:
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			return false
		}
		_, err := strconv.ParseFloat(trimmed, 64)
		return err == nil
	}
	return false
}

// Strval converts v to a string exactly the way PHP casts a value to string.
func Strval(v any) string {
	switch value := v.(type) {
	case nil:
		return ""
	case bool:
		if value {
			return "1"
		}
		return ""
	case string:
		return value
	case int:
		return strconv.FormatInt(int64(value), 10)
	case int8:
		return strconv.FormatInt(int64(value), 10)
	case int16:
		return strconv.FormatInt(int64(value), 10)
	case int32:
		return strconv.FormatInt(int64(value), 10)
	case int64:
		return strconv.FormatInt(value, 10)
	case uint:
		return strconv.FormatUint(uint64(value), 10)
	case uint8:
		return strconv.FormatUint(uint64(value), 10)
	case uint16:
		return strconv.FormatUint(uint64(value), 10)
	case uint32:
		return strconv.FormatUint(uint64(value), 10)
	case uint64:
		return strconv.FormatUint(value, 10)
	case float32:
		return formatFloat(float64(value))
	case float64:
		return formatFloat(value)
	}
	return ""
}

func formatFloat(f float64) string {
	switch {
	case math.IsNaN(f):
		return "NAN"
	case math.IsInf(f, 1):
		return "INF"
	case math.IsInf(f, -1):
		return "-INF"
	}
	formatted := strconv.FormatFloat(f, 'G', 14, 64)
	index := strings.IndexByte(formatted, 'E')
	if index < 0 {
		return formatted
	}
	mantissa, exponent := formatted[:index], formatted[index+1:]
	if !strings.Contains(mantissa, ".") {
		mantissa += ".0"
	}
	sign, digits := exponent[:1], strings.TrimLeft(exponent[1:], "0")
	if digits == "" {
		digits = "0"
	}
	return mantissa + "E" + sign + digits
}

// Round rounds half away from zero, matching PHP round.
func Round(v float64) int64 {
	if v >= 0 {
		return int64(math.Floor(v + 0.5))
	}
	return int64(math.Ceil(v - 0.5))
}

// Int converts v to an integer the way PHP casts a value to int.
func Int(v any) int64 {
	switch value := v.(type) {
	case nil:
		return 0
	case bool:
		if value {
			return 1
		}
		return 0
	case int:
		return int64(value)
	case int64:
		return value
	case float64:
		if math.IsNaN(value) || math.IsInf(value, 0) {
			return 0
		}
		return int64(math.Trunc(value))
	case string:
		return leadingInt(value)
	}
	if IsNumeric(v) {
		parsed, err := strconv.ParseFloat(Strval(v), 64)
		if err != nil {
			return 0
		}
		return int64(math.Trunc(parsed))
	}
	return 0
}

func leadingInt(text string) int64 {
	trimmed := strings.TrimSpace(text)
	index := 0
	if index < len(trimmed) && (trimmed[index] == '+' || trimmed[index] == '-') {
		index++
	}
	start := index
	for index < len(trimmed) && trimmed[index] >= '0' && trimmed[index] <= '9' {
		index++
	}
	if index == start {
		return 0
	}
	parsed, err := strconv.ParseInt(trimmed[:index], 10, 64)
	if err != nil {
		return 0
	}
	return parsed
}

// Encode serializes v to JSON the way PHP json_encode does. With escapeSlashes
// it reproduces json_encode called without flags: slashes and non-ASCII runes
// are escaped. Without it, it reproduces JSON_UNESCAPED_SLASHES|JSON_UNESCAPED_UNICODE.
func Encode(v any, escapeSlashes bool) ([]byte, error) {
	var buffer bytes.Buffer
	encoder := json.NewEncoder(&buffer)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(v); err != nil {
		return nil, err
	}
	encoded := bytes.TrimRight(buffer.Bytes(), "\n")
	if !escapeSlashes {
		return encoded, nil
	}
	return escapeASCII(bytes.ReplaceAll(encoded, []byte("/"), []byte(`\/`))), nil
}

func escapeASCII(encoded []byte) []byte {
	if isASCII(encoded) {
		return encoded
	}
	var out bytes.Buffer
	for index := 0; index < len(encoded); {
		if encoded[index] < utf8.RuneSelf {
			out.WriteByte(encoded[index])
			index++
			continue
		}
		decoded, size := utf8.DecodeRune(encoded[index:])
		if decoded > 0xFFFF {
			high, low := utf16.EncodeRune(decoded)
			out.WriteString(`\u` + hex4(uint16(high)) + `\u` + hex4(uint16(low)))
		} else {
			out.WriteString(`\u` + hex4(uint16(decoded)))
		}
		index += size
	}
	return out.Bytes()
}

func isASCII(encoded []byte) bool {
	for _, character := range encoded {
		if character >= utf8.RuneSelf {
			return false
		}
	}
	return true
}

func hex4(value uint16) string {
	const digits = "0123456789abcdef"
	return string([]byte{
		digits[value>>12&0xF],
		digits[value>>8&0xF],
		digits[value>>4&0xF],
		digits[value&0xF],
	})
}
