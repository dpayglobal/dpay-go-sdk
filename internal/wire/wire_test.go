package wire

import (
	"testing"

	"github.com/dpayglobal/dpay-go-sdk/internal/php"
)

func encode(t *testing.T, body *Body) string {
	t.Helper()
	encoded, err := php.Encode(body, false)
	if err != nil {
		t.Fatal(err)
	}
	return string(encoded)
}

func TestBodyPreservesInsertionOrder(t *testing.T) {
	body := NewBody()
	body.Set("zzz", 1)
	body.Set("aaa", 2)
	body.Set("mmm", 3)

	encoded := encode(t, body)
	if encoded != `{"zzz":1,"aaa":2,"mmm":3}` {
		t.Fatalf("Marshal = %s", encoded)
	}
	values := body.Values()
	if len(values) != 3 || values[0] != 1 || values[2] != 3 {
		t.Fatalf("Values = %v", values)
	}
}

func TestBodyOverwriteKeepsPosition(t *testing.T) {
	body := NewBody()
	body.Set("a", 1)
	body.Set("b", 2)
	body.Set("a", 9)

	encoded := encode(t, body)
	if encoded != `{"a":9,"b":2}` {
		t.Fatalf("Marshal = %s", encoded)
	}
	if body.Len() != 2 {
		t.Fatalf("Len = %d", body.Len())
	}
}

func TestBodyEmptyIsObject(t *testing.T) {
	encoded := encode(t, NewBody())
	if encoded != `{}` {
		t.Fatalf("Marshal = %s, want {}", encoded)
	}
}

func TestBodyNested(t *testing.T) {
	inner := NewBody()
	inner.Set("second", 2)
	inner.Set("first", 1)

	outer := NewBody()
	outer.Set("nested", inner)
	outer.Set("list", []any{1, "two"})

	encoded := encode(t, outer)
	if encoded != `{"nested":{"second":2,"first":1},"list":[1,"two"]}` {
		t.Fatalf("Marshal = %s", encoded)
	}
}

func TestBodyDoesNotEscapeHTML(t *testing.T) {
	body := NewBody()
	body.Set("description", "Zamówienie <b>&</b>")
	body.Set("url", "https://a/b")

	encoded := encode(t, body)
	want := `{"description":"Zamówienie <b>&</b>","url":"https://a/b"}`
	if encoded != want {
		t.Fatalf("Marshal = %s, want %s", encoded, want)
	}
}

func TestBodyFalseIsKept(t *testing.T) {
	body := NewBody()
	body.Set("accept_tos", false)
	body.Set("creditcard", 0)

	encoded := encode(t, body)
	if encoded != `{"accept_tos":false,"creditcard":0}` {
		t.Fatalf("Marshal = %s", encoded)
	}
}

func TestBodySetIfNotNil(t *testing.T) {
	body := NewBody()
	var missing *string
	present := "x"
	body.SetIfNotNil("skipped", missing)
	body.SetIfNotNil("kept", &present)
	body.SetIfNotNil("also_skipped", nil)

	encoded := encode(t, body)
	if encoded != `{"kept":"x"}` {
		t.Fatalf("Marshal = %s", encoded)
	}
}

func TestSecretSecond(t *testing.T) {
	checksum := NewChecksum("secret_hash")
	got := checksum.SecretSecond("test_service", []any{"29.99", "https://a", "https://b", "https://c"})
	if len(got) != 64 {
		t.Fatalf("length = %d", len(got))
	}
	same := checksum.SecretSecond("test_service", []any{"29.99", "https://a", "https://b", "https://c"})
	if got != same {
		t.Fatal("not deterministic")
	}
	different := checksum.SecretSecond("test_service", []any{"29.99", "https://b", "https://a", "https://c"})
	if got == different {
		t.Fatal("field order must change the checksum")
	}
}

func TestSecretSecondMatchesPHP(t *testing.T) {
	got := NewChecksum("secret_hash").SecretSecond("test_service", []any{"29.99", "https://a", "https://b", "https://c"})
	const wantFromPHP = "81f765df63ef8af0f4a3ba3d63aa05cf78d1c776a7893fbd50a41144e189ff19"
	if got != wantFromPHP {
		t.Fatalf("got %s, want %s", got, wantFromPHP)
	}
}

func TestSecretSecondUsesPHPStringConversion(t *testing.T) {
	checksum := NewChecksum("s")
	if checksum.SecretSecond("svc", []any{true}) != checksum.SecretSecond("svc", []any{"1"}) {
		t.Fatal("true must serialize as \"1\"")
	}
	if checksum.SecretSecond("svc", []any{10.0}) != checksum.SecretSecond("svc", []any{"10"}) {
		t.Fatal("10.0 must serialize as \"10\"")
	}
}

func TestOrderedBody(t *testing.T) {
	checksum := NewChecksum("secret_hash")
	got := checksum.OrderedBody([]any{"test_service", "tx-1"})
	reordered := checksum.OrderedBody([]any{"tx-1", "test_service"})
	if got == reordered {
		t.Fatal("value order must change the checksum")
	}
	if len(got) != 64 {
		t.Fatalf("length = %d", len(got))
	}
}

func TestBaseURLsDefaults(t *testing.T) {
	urls := NewBaseURLs("", "", "")
	cases := map[string]string{
		HostAPIPayments: "https://api-payments.dpay.pl",
		HostPanel:       "https://panel.dpay.pl",
		HostGateway:     "https://secure.dpay.pl",
	}
	for host, want := range cases {
		if got := urls.Resolve(host); got != want {
			t.Errorf("Resolve(%q) = %q, want %q", host, got, want)
		}
	}
}

func TestBaseURLsOverrideTrimsTrailingSlash(t *testing.T) {
	urls := NewBaseURLs("", "https://panel.test///", "")
	if got := urls.Resolve(HostPanel); got != "https://panel.test" {
		t.Fatalf("Resolve = %q", got)
	}
	if got := urls.Resolve(HostAPIPayments); got != "https://api-payments.dpay.pl" {
		t.Fatalf("untouched host changed: %q", got)
	}
}

func TestBaseURLsUnknownHost(t *testing.T) {
	if got := NewBaseURLs("", "", "").Resolve("nope"); got != "" {
		t.Fatalf("Resolve(unknown) = %q, want empty", got)
	}
}
