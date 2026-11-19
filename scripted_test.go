package dpay

import (
	"io"
	"net/http"
	"strings"
	"testing"
)

// scriptedResponse is one queued answer of a scriptedDoer.
type scriptedResponse struct {
	status  int
	body    string
	headers map[string]string
}

// scriptedDoer answers with the queued responses in order and records every
// request with its body.
type scriptedDoer struct {
	responses []scriptedResponse
	requests  []*http.Request
	bodies    []string
}

func (d *scriptedDoer) Do(request *http.Request) (*http.Response, error) {
	body := ""
	if request.Body != nil {
		raw, err := io.ReadAll(request.Body)
		if err != nil {
			return nil, err
		}
		body = string(raw)
	}
	d.requests = append(d.requests, request)
	d.bodies = append(d.bodies, body)
	answer := scriptedResponse{status: 200, body: "{}"}
	if len(d.responses) > 0 {
		answer, d.responses = d.responses[0], d.responses[1:]
	}
	header := http.Header{"Content-Type": []string{"application/json"}}
	for name, value := range answer.headers {
		header.Set(name, value)
	}
	return &http.Response{StatusCode: answer.status, Header: header, Body: io.NopCloser(strings.NewReader(answer.body))}, nil
}

// lastBody decodes the body of the last request.
func (d *scriptedDoer) lastBody(t *testing.T) map[string]any {
	t.Helper()
	if len(d.bodies) == 0 {
		t.Fatal("no request was sent")
	}
	return decodeBody(t, d.bodies[len(d.bodies)-1])
}

// scriptedClient returns a client for service MyShop and hash secret123 (the
// values of the PHP SDK tests) answering with the given responses.
func scriptedClient(t *testing.T, responses ...scriptedResponse) (*Client, *scriptedDoer) {
	t.Helper()
	doer := &scriptedDoer{responses: responses}
	client, err := New("MyShop", "secret123", WithHTTPClient(doer))
	if err != nil {
		t.Fatal(err)
	}
	return client, doer
}

func okResponse(body string) scriptedResponse { return scriptedResponse{status: 200, body: body} }
