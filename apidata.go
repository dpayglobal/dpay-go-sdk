package dpay

import "github.com/dpayglobal/dpay-go-sdk/internal/php"

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
