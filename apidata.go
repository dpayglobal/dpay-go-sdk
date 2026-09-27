package dpay

import (
	"math"
	"strconv"

	"github.com/dpayglobal/dpay-go-sdk/internal/php"
)

// phpArray reports whether value is what PHP json_decode turns into an array: a
// JSON object or a list. A list yields an empty object, because the SDK reads
// only string keys from it.
func phpArray(value any) (map[string]any, bool) {
	switch typed := value.(type) {
	case map[string]any:
		return typed, true
	case []any:
		return map[string]any{}, true
	}
	return nil, false
}

// phpObject returns value as an object, or an empty one when it is not an array.
func phpObject(value any) map[string]any {
	if object, ok := phpArray(value); ok {
		return object
	}
	return map[string]any{}
}

// strictString returns data[key] only when it is a JSON string (PHP is_string);
// numbers and booleans give an empty string, unlike stringField.
func strictString(data map[string]any, key string) string {
	value, _ := data[key].(string)
	return value
}

// jsonInt returns value as an integer when it is a JSON integer (PHP is_int).
// encoding/json decodes every number as float64, so an integral float such as
// 5.0 counts as well.
func jsonInt(value any) (int64, bool) {
	switch typed := value.(type) {
	case int:
		return int64(typed), true
	case int64:
		return typed, true
	case float64:
		if typed != math.Trunc(typed) || typed < -9.223372036854775808e18 || typed >= 9.223372036854775808e18 {
			return 0, false
		}
		return int64(typed), true
	}
	return 0, false
}

// digitsInt converts a string of ASCII digits to an integer (PHP ctype_digit
// followed by an int cast, which saturates on overflow).
func digitsInt(value any) (int64, bool) {
	text, ok := value.(string)
	if !ok || text == "" {
		return 0, false
	}
	for index := 0; index < len(text); index++ {
		if text[index] < '0' || text[index] > '9' {
			return 0, false
		}
	}
	parsed, err := strconv.ParseInt(text, 10, 64)
	if err != nil {
		return math.MaxInt64, true
	}
	return parsed, true
}

func optionalJSONInt(data map[string]any, key string) *int64 {
	if value, ok := jsonInt(data[key]); ok {
		return &value
	}
	return nil
}

func stringField(data map[string]any, key string) string {
	value, exists := data[key]
	if !exists || !php.IsScalar(value) {
		return ""
	}
	return php.Strval(value)
}

func boolField(data map[string]any, key string) bool {
	switch value := data[key].(type) {
	case bool:
		return value
	case float64:
		return value != 0
	case string:
		return value != "" && value != "0"
	}
	return false
}

func intField(data map[string]any, key string) int64 {
	value, exists := data[key]
	if !exists {
		return 0
	}
	return php.Int(value)
}

func objectField(data map[string]any, key string) map[string]any {
	if value, ok := data[key].(map[string]any); ok {
		return value
	}
	return map[string]any{}
}

func arrayField(data map[string]any, key string) []any {
	if value, ok := data[key].([]any); ok {
		return value
	}
	return nil
}

func moneyField(data map[string]any, key string, currency Currency) Money {
	value, exists := data[key]
	if !exists {
		value = float64(0)
	}
	if money, ok := moneyFromAPI(value, currency); ok {
		return money
	}
	zero, _ := NewMoney(0, currency)
	return zero
}

func optionalInt(data map[string]any, key string) *int64 {
	value, present := data[key]
	if !present || value == nil {
		return nil
	}
	converted := intField(data, key)
	return &converted
}

func marshalRaw(data map[string]any) string {
	encoded, err := php.Encode(data, false)
	if err != nil {
		return ""
	}
	return string(encoded)
}
