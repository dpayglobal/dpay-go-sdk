package dpay

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/dpayglobal/dpay-go-sdk/internal/php"
	"github.com/dpayglobal/dpay-go-sdk/internal/wire"
)

// HTTPDoer is the transport the client sends requests through. *http.Client
// satisfies it, which is the point: retries, proxies and instrumentation are
// configured with a custom http.Client or RoundTripper.
type HTTPDoer interface {
	Do(req *http.Request) (*http.Response, error)
}

type apiResponse struct {
	status  int
	headers map[string]string
	body    []byte
}

func (r apiResponse) header(name string) string {
	return r.headers[lowercase(name)]
}

func (r apiResponse) decodeJSONObject() (map[string]any, bool) {
	var decoded any
	if err := json.Unmarshal(r.body, &decoded); err != nil {
		return nil, false
	}
	object, ok := decoded.(map[string]any)
	return object, ok
}

func (r apiResponse) decodeJSONArray() ([]any, bool) {
	var decoded any
	if err := json.Unmarshal(r.body, &decoded); err != nil {
		return nil, false
	}
	array, ok := decoded.([]any)
	return array, ok
}

func lowercase(value string) string {
	return strings.ToLower(value)
}

func (c *Client) send(ctx context.Context, method, host, path string, body []byte) (apiResponse, error) {
	base := c.baseURLs.Resolve(host)
	if base == "" {
		return apiResponse{}, newValidationError(`Unknown API host "` + host + `"`)
	}

	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}
	request, err := http.NewRequestWithContext(ctx, method, base+path, reader)
	if err != nil {
		return apiResponse{}, newTransportError("unable to build request", err)
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("User-Agent", userAgent())
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}

	response, err := c.httpClient.Do(request)
	if err != nil {
		return apiResponse{}, newTransportError("request failed", err)
	}
	defer response.Body.Close()

	payload, err := io.ReadAll(response.Body)
	if err != nil {
		return apiResponse{}, newTransportError("unable to read response body", err)
	}

	headers := make(map[string]string, len(response.Header))
	for name, values := range response.Header {
		if len(values) > 0 {
			headers[lowercase(name)] = values[0]
		}
	}
	return apiResponse{status: response.StatusCode, headers: headers, body: payload}, nil
}

func (c *Client) sendBody(ctx context.Context, method, host, path string, body *wire.Body) (apiResponse, error) {
	encoded, err := php.Encode(body, false)
	if err != nil {
		return apiResponse{}, newTransportError("Unable to encode request body as JSON", err)
	}
	return c.send(ctx, method, host, path, encoded)
}

func (c *Client) postJSONObject(ctx context.Context, host, path string, body *wire.Body) (map[string]any, error) {
	response, err := c.sendBody(ctx, "POST", host, path, body)
	if err != nil {
		return nil, err
	}
	if response.status >= 400 {
		return nil, mapAPIError(response)
	}
	return decodeObjectOrFail(response)
}

func (c *Client) postJSONArray(ctx context.Context, host, path string, body *wire.Body) ([]any, error) {
	response, err := c.sendBody(ctx, "POST", host, path, body)
	if err != nil {
		return nil, err
	}
	if response.status >= 400 {
		return nil, mapAPIError(response)
	}
	return decodeArrayOrFail(response)
}

func (c *Client) getJSONArray(ctx context.Context, host, path string) ([]any, error) {
	response, err := c.send(ctx, "GET", host, path, nil)
	if err != nil {
		return nil, err
	}
	if response.status >= 400 {
		return nil, mapAPIError(response)
	}
	return decodeArrayOrFail(response)
}

func (c *Client) getText(ctx context.Context, host, path string) (string, error) {
	response, err := c.send(ctx, "GET", host, path, nil)
	if err != nil {
		return "", err
	}
	if response.status >= 400 {
		return "", mapAPIError(response)
	}
	return string(response.body), nil
}

func decodeObjectOrFail(response apiResponse) (map[string]any, error) {
	data, ok := response.decodeJSONObject()
	if !ok {
		return nil, invalidJSONError(response)
	}
	return data, nil
}

func decodeArrayOrFail(response apiResponse) ([]any, error) {
	data, ok := response.decodeJSONArray()
	if !ok {
		return nil, invalidJSONError(response)
	}
	return data, nil
}

func invalidJSONError(response apiResponse) *APIError {
	return &APIError{
		kind:        ErrServer,
		message:     "Invalid JSON in API response",
		HTTPStatus:  response.status,
		FieldErrors: map[string][]string{},
		RawBody:     string(response.body),
	}
}
