package dpay

import (
	"strconv"

	"github.com/dpayglobal/dpay-go-sdk/internal/php"
)

func mapAPIError(response apiResponse) error {
	data, _ := response.decodeJSONObject()

	message := "Unexpected API error"
	if value, ok := data["message"].(string); ok {
		message = value
	} else if value, ok := data["msg"].(string); ok {
		message = value
	}

	// Cards API and webhook errors carry "code", the older payments errors "errorcode"
	errorCode := ""
	if value, ok := data["code"].(string); ok {
		errorCode = value
	} else if value, ok := data["errorcode"].(string); ok {
		errorCode = value
	}
	reason, _ := data["reason"].(string)

	base := &APIError{
		message:     message,
		HTTPStatus:  response.status,
		ErrorCode:   errorCode,
		Reason:      reason,
		FieldErrors: normalizeFieldErrors(data["errors"]),
		RawBody:     string(response.body),
	}

	if response.status == 429 {
		base.kind = ErrRateLimit
		base.ErrorCode = ""
		base.Reason = ""
		base.FieldErrors = map[string][]string{}
		return &RateLimitError{
			APIError:   base,
			RetryAfter: intHeader(response, "Retry-After"),
			Limit:      intHeader(response, "X-RateLimit-Limit"),
			Remaining:  intHeader(response, "X-RateLimit-Remaining"),
		}
	}

	switch {
	case response.status == 401:
		base.kind = ErrAuthentication
	case response.status == 403:
		base.kind = ErrAccessDenied
	case response.status == 404:
		base.kind = ErrNotFound
	case response.status == 400 || response.status == 422:
		base.kind = ErrInvalidRequest
	case response.status >= 500:
		base.kind = ErrServer
	default:
		base.kind = ErrAPI
	}
	return base
}

func normalizeFieldErrors(value any) map[string][]string {
	normalized := map[string][]string{}
	fields, ok := value.(map[string]any)
	if !ok {
		return normalized
	}
	for field, messages := range fields {
		switch typed := messages.(type) {
		case string:
			normalized[field] = []string{typed}
		case []any:
			collected := []string{}
			for _, single := range typed {
				if php.IsScalar(single) {
					collected = append(collected, php.Strval(single))
				}
			}
			normalized[field] = collected
		}
	}
	return normalized
}

func intHeader(response apiResponse, name string) *int {
	raw := response.header(name)
	if raw == "" {
		return nil
	}
	parsed, err := strconv.Atoi(raw)
	if err != nil {
		return nil
	}
	return &parsed
}
