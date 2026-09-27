package dpay

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestNewRequiresServiceAndSecret(t *testing.T) {
	if _, err := New("", "hash"); err == nil || !errors.Is(err, ErrInvalidArgument) {
		t.Fatalf("empty service: %v", err)
	} else if err.Error() != `dpay: Option "service" is required and must be a non-empty string` {
		t.Fatalf("message = %q", err.Error())
	}
	if _, err := New("svc", ""); err == nil {
		t.Fatal("empty secret must fail")
	} else if err.Error() != `dpay: Option "secret_hash" is required and must be a non-empty string` {
		t.Fatalf("message = %q", err.Error())
	}
}

func TestNewRejectsNonPositiveTimeout(t *testing.T) {
	_, err := New("svc", "hash", WithTimeout(0))
	if err == nil || err.Error() != `dpay: Option "timeout" must be a positive integer` {
		t.Fatalf("err = %v", err)
	}
	if _, err := New("svc", "hash", WithTimeout(-time.Second)); err == nil {
		t.Fatal("negative timeout must fail")
	}
}

func TestNewWiresServices(t *testing.T) {
	client, err := New("svc", "hash")
	if err != nil {
		t.Fatal(err)
	}
	if client.Payments == nil || client.Refunds == nil || client.Banks == nil ||
		client.Blik == nil || client.Cards == nil || client.Payouts == nil ||
		client.Recurring == nil || client.Events == nil {
		t.Fatal("all eight services must be wired")
	}
	if client.Service() != "svc" {
		t.Fatalf("Service() = %q", client.Service())
	}
}

func TestBaseURLDefaultsAndOverrides(t *testing.T) {
	client, _ := New("svc", "hash")
	if got := client.baseURLs.Resolve(hostPanel); got != "https://panel.dpay.pl" {
		t.Fatalf("default panel = %q", got)
	}
	if got := client.baseURLs.Resolve(hostAPIPayments); got != "https://api-payments.dpay.pl" {
		t.Fatalf("default api = %q", got)
	}
	if got := client.baseURLs.Resolve(hostGateway); got != "https://secure.dpay.pl" {
		t.Fatalf("default gateway = %q", got)
	}

	overridden, _ := New("svc", "hash", WithBaseURLs(BaseURLs{Panel: "https://panel.test/"}))
	if got := overridden.baseURLs.Resolve(hostPanel); got != "https://panel.test" {
		t.Fatalf("override = %q", got)
	}
	if got := overridden.baseURLs.Resolve(hostAPIPayments); got != "https://api-payments.dpay.pl" {
		t.Fatalf("untouched host = %q", got)
	}
}

func TestWithHTTPClient(t *testing.T) {
	custom := &http.Client{Timeout: time.Second}
	client, err := New("svc", "hash", WithHTTPClient(custom))
	if err != nil {
		t.Fatal(err)
	}
	if client.httpClient != HTTPDoer(custom) {
		t.Fatal("custom transport not used")
	}
}

func TestDefaultTimeout(t *testing.T) {
	client, _ := New("svc", "hash")
	standard, ok := client.httpClient.(*http.Client)
	if !ok {
		t.Fatal("default transport must be *http.Client")
	}
	if standard.Timeout != 30*time.Second {
		t.Fatalf("Timeout = %v, want 30s", standard.Timeout)
	}
}

func TestPointerHelpers(t *testing.T) {
	if *String("x") != "x" || *Bool(false) != false || *Int(3) != 3 || *Int64(4) != 4 {
		t.Fatal("pointer helpers broken")
	}
}

func TestTransportSendsHeadersAndBody(t *testing.T) {
	var gotMethod, gotPath, gotAccept, gotContentType, gotUserAgent, gotBody string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod, gotPath = r.Method, r.URL.Path
		gotAccept = r.Header.Get("Accept")
		gotContentType = r.Header.Get("Content-Type")
		gotUserAgent = r.Header.Get("User-Agent")
		buffer := make([]byte, r.ContentLength)
		r.Body.Read(buffer)
		gotBody = string(buffer)
		w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	client, err := New("svc", "hash", WithBaseURLs(BaseURLs{Panel: server.URL}))
	if err != nil {
		t.Fatal(err)
	}
	got, err := client.send(context.Background(), "POST", hostPanel, "/echo", []byte(`{"a":1}`))
	if err != nil {
		t.Fatal(err)
	}
	if got.status != 200 {
		t.Fatalf("status = %d", got.status)
	}
	if gotMethod != "POST" || gotPath != "/echo" {
		t.Fatalf("method/path = %s %s", gotMethod, gotPath)
	}
	if gotAccept != "application/json" || gotContentType != "application/json" {
		t.Fatalf("headers = %q %q", gotAccept, gotContentType)
	}
	if gotUserAgent != userAgent() {
		t.Fatalf("User-Agent = %q", gotUserAgent)
	}
	if gotBody != `{"a":1}` {
		t.Fatalf("body = %q", gotBody)
	}
}

func TestTransportOmitsContentTypeWithoutBody(t *testing.T) {
	var gotContentType string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotContentType = r.Header.Get("Content-Type")
		w.Write([]byte(`[]`))
	}))
	defer server.Close()

	client, _ := New("svc", "hash", WithBaseURLs(BaseURLs{Panel: server.URL}))
	if _, err := client.send(context.Background(), "GET", hostPanel, "/x", nil); err != nil {
		t.Fatal(err)
	}
	if gotContentType != "" {
		t.Fatalf("Content-Type = %q, want empty", gotContentType)
	}
}

func TestTransportNetworkFailureIsTransportError(t *testing.T) {
	client, _ := New("svc", "hash", WithBaseURLs(BaseURLs{Panel: "http://127.0.0.1:1"}))
	_, err := client.send(context.Background(), "GET", hostPanel, "/x", nil)
	if !errors.Is(err, ErrTransport) {
		t.Fatalf("err = %v, want ErrTransport", err)
	}
	if errors.Is(err, ErrAPI) {
		t.Fatal("network failure must not look like an API error")
	}
}

func TestTransportContextCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
	}))
	defer server.Close()

	client, _ := New("svc", "hash", WithBaseURLs(BaseURLs{Panel: server.URL}))
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := client.send(ctx, "GET", hostPanel, "/x", nil)
	if !errors.Is(err, ErrTransport) || !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want ErrTransport wrapping context.Canceled", err)
	}
}

func TestTransportReturnsErrorStatusesToCaller(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
		w.Write([]byte(`{"message":"gone"}`))
	}))
	defer server.Close()

	client, _ := New("svc", "hash", WithBaseURLs(BaseURLs{Panel: server.URL}))
	got, err := client.send(context.Background(), "GET", hostPanel, "/x", nil)
	if err != nil {
		t.Fatalf("send must not raise on 4xx: %v", err)
	}
	if got.status != 404 {
		t.Fatalf("status = %d", got.status)
	}
}

func TestUnknownHostIsValidationError(t *testing.T) {
	client, _ := New("svc", "hash")
	_, err := client.send(context.Background(), "GET", "nope", "/x", nil)
	if !errors.Is(err, ErrInvalidArgument) {
		t.Fatalf("err = %v", err)
	}
}

func TestDefaultTransportDoesNotFollowRedirects(t *testing.T) {
	var hits int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		if r.URL.Path == "/redirect-me" {
			http.Redirect(w, r, "/landed", http.StatusFound)
			return
		}
		w.Write([]byte(`<!DOCTYPE html><html>login page</html>`))
	}))
	defer server.Close()

	client, err := New("svc", "hash", WithBaseURLs(BaseURLs{Panel: server.URL}))
	if err != nil {
		t.Fatal(err)
	}
	got, err := client.send(context.Background(), "GET", hostPanel, "/redirect-me", nil)
	if err != nil {
		t.Fatal(err)
	}
	if got.status != http.StatusFound {
		t.Fatalf("status = %d, want 302 - the PHP SDK does not follow redirects either", got.status)
	}
	if hits != 1 {
		t.Fatalf("server hit %d times, want 1 - the redirect must not be followed", hits)
	}
	if location := got.header("Location"); location != "/landed" {
		t.Fatalf("Location = %q, want it preserved for diagnostics", location)
	}
}

func TestRedirectedResponseIsNotMistakenForSuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/login", http.StatusFound)
	}))
	defer server.Close()

	client, _ := New("svc", "hash", WithBaseURLs(BaseURLs{Panel: server.URL}))
	_, err := client.Payments.Details(context.Background(), "tx-1")
	if err == nil {
		t.Fatal("a redirect must not be reported as a successful transaction lookup")
	}
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("err = %v, want an *APIError", err)
	}
	if apiErr.HTTPStatus != http.StatusFound {
		t.Fatalf("HTTPStatus = %d, want 302 so the caller can diagnose it", apiErr.HTTPStatus)
	}
}
