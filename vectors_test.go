package dpay

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"reflect"
	"testing"

	"github.com/dpayglobal/dpay-go-sdk/internal/wire"
)

// apiVectors mirrors testdata/api_vectors.json: checksum, webhook and IPN vectors
// shared by all dpay SDKs (synthetic data computed with the API code; identical
// to tests/Fixtures/api_vectors.json of the PHP SDK).
type apiVectors struct {
	Service       string `json:"service"`
	SecretHash    string `json:"secret_hash"`
	TransactionID string `json:"transaction_id"`
	SecretSecond  []struct {
		Name     string   `json:"name"`
		Fields   []string `json:"fields"`
		Checksum string   `json:"checksum"`
	} `json:"secret_second"`
	Operation []struct {
		Name      string  `json:"name"`
		Operation string  `json:"operation"`
		Amount    *string `json:"amount"`
		Checksum  string  `json:"checksum"`
	} `json:"operation"`
	OrderedBody []struct {
		Name     string          `json:"name"`
		Body     json.RawMessage `json:"body"`
		Checksum string          `json:"checksum"`
	} `json:"ordered_body"`
	Webhook webhookVector `json:"webhook"`
	IPN     []struct {
		Name    string          `json:"name"`
		Payload json.RawMessage `json:"payload"`
	} `json:"ipn"`
}

type webhookVector struct {
	Secret            string `json:"secret"`
	OldSecret         string `json:"old_secret"`
	ID                string `json:"id"`
	Timestamp         int64  `json:"timestamp"`
	Body              string `json:"body"`
	Signature         string `json:"signature"`
	RotationSignature string `json:"rotation_signature"`
}

func loadAPIVectors(t *testing.T) apiVectors {
	t.Helper()
	raw, err := os.ReadFile("testdata/api_vectors.json")
	if err != nil {
		t.Fatalf("testdata/api_vectors.json: %v", err)
	}
	var vectors apiVectors
	if err := json.Unmarshal(raw, &vectors); err != nil {
		t.Fatal(err)
	}
	if len(vectors.SecretSecond) == 0 || len(vectors.Operation) == 0 || len(vectors.OrderedBody) == 0 || vectors.Webhook.ID == "" || len(vectors.IPN) == 0 {
		t.Fatal("testdata/api_vectors.json is incomplete")
	}
	return vectors
}

// decodeOrdered decodes JSON keeping the key order of objects (as *wire.Body)
// and integers as integers, like PHP json_decode.
func decodeOrdered(t *testing.T, raw []byte) any {
	t.Helper()
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	value, err := readOrdered(decoder)
	if err != nil {
		t.Fatal(err)
	}
	return value
}

func readOrdered(decoder *json.Decoder) (any, error) {
	token, err := decoder.Token()
	if err != nil {
		return nil, err
	}
	switch typed := token.(type) {
	case json.Delim:
		if typed == '{' {
			body := wire.NewBody()
			for decoder.More() {
				key, err := decoder.Token()
				if err != nil {
					return nil, err
				}
				value, err := readOrdered(decoder)
				if err != nil {
					return nil, err
				}
				body.Set(key.(string), value)
			}
			_, err := decoder.Token()
			return body, err
		}
		list := []any{}
		for decoder.More() {
			value, err := readOrdered(decoder)
			if err != nil {
				return nil, err
			}
			list = append(list, value)
		}
		_, err := decoder.Token()
		return list, err
	case json.Number:
		if integer, err := typed.Int64(); err == nil {
			return integer, nil
		}
		return typed.Float64()
	}
	return token, nil
}

func TestAPIVectorsSecretSecond(t *testing.T) {
	vectors := loadAPIVectors(t)
	checksum := wire.NewChecksum(vectors.SecretHash)
	for _, vector := range vectors.SecretSecond {
		fields := make([]any, 0, len(vector.Fields))
		for _, field := range vector.Fields {
			fields = append(fields, field)
		}
		if got := checksum.SecretSecond(vectors.Service, fields); got != vector.Checksum {
			t.Errorf("%s = %s, want %s", vector.Name, got, vector.Checksum)
		}
	}
}

func TestAPIVectorsOperation(t *testing.T) {
	vectors := loadAPIVectors(t)
	checksum := wire.NewChecksum(vectors.SecretHash)
	for _, vector := range vectors.Operation {
		amount := ""
		if vector.Amount != nil {
			amount = *vector.Amount
		}
		if got := checksum.Operation(vector.Operation, vectors.Service, vectors.TransactionID, amount); got != vector.Checksum {
			t.Errorf("%s = %s, want %s", vector.Name, got, vector.Checksum)
		}
	}
}

func TestAPIVectorsOrderedBody(t *testing.T) {
	vectors := loadAPIVectors(t)
	checksum := wire.NewChecksum(vectors.SecretHash)
	for _, vector := range vectors.OrderedBody {
		body, ok := decodeOrdered(t, vector.Body).(*wire.Body)
		if !ok {
			t.Fatalf("%s: body is not an object", vector.Name)
		}
		if got := checksum.OrderedBody(body); got != vector.Checksum {
			t.Errorf("%s = %s, want %s", vector.Name, got, vector.Checksum)
		}
	}
}

// The SDK itself produces the vector checksums: registration without IPN, recurring charge, refund with a webhook.
func TestAPIVectorsMatchTheRequestsTheSDKSends(t *testing.T) {
	vectors := loadAPIVectors(t)
	want := map[string]string{}
	for _, vector := range vectors.SecretSecond {
		want[vector.Name] = vector.Checksum
	}
	for _, vector := range vectors.OrderedBody {
		want[vector.Name] = vector.Checksum
	}

	doer := &recordingDoer{responses: []string{
		`{"error":false,"msg":"Internal processing","status":true,"transactionId":"TX-REG"}`,
		`{"error":false,"msg":"Internal processing","status":true,"transactionId":"TX-CHG"}`,
		`{"status":"success","refund":true}`,
	}}
	client, err := New(vectors.Service, vectors.SecretHash, WithHTTPClient(doer))
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	urls := ReturnURLs{Success: "https://shop.example/ok", Fail: "https://shop.example/fail"}
	mustRegister(t, client, ctx, &RegisterPaymentRequest{
		Amount: PLN(0), TransactionType: TransactionTypeTransfers, URLs: urls,
		BlikCode: String("777123"), UserAgent: String("Mozilla/5.0"), UserIP: String("83.238.17.42"),
		RecurringRegistration: &RecurringRegistration{Label: "Abonament", Model: RecurringModelOnDemand, TermsURL: "https://shop.example/terms"},
	})
	mustRegister(t, client, ctx, &RegisterPaymentRequest{
		Amount: PLN(4999), TransactionType: TransactionTypeTransfers, URLs: urls, RecurringAlias: String("SUB-0001"),
	})
	if _, err := client.Refunds.Create(ctx, vectors.TransactionID, WithRefundAmount(PLN(1500)), WithRefundReason("Zwrot"),
		WithRefundWebhook(WebhookTarget{URL: "https://shop.example/webhooks/refunds",
			Events: []WebhookEventType{WebhookEventTypeRefundSucceeded, WebhookEventTypeRefundFailed}})); err != nil {
		t.Fatal(err)
	}

	for index, name := range []string{"register_recurring_value_0_without_ipn", "recurring_charge_without_ipn", "refund_with_reason_and_webhook"} {
		if got := decodeBody(t, doer.bodies[index])["checksum"]; got != want[name] {
			t.Errorf("%s: checksum = %v, want %s", name, got, want[name])
		}
	}
}

func TestOrderedBodySkipsChecksumAndCastsLikeTheAPI(t *testing.T) {
	body, ok := decodeOrdered(t, []byte(`{"x":"a","checksum":"ignored","y":true,"z":null,"w":{"v":"b"}}`)).(*wire.Body)
	if !ok {
		t.Fatal("not an object")
	}
	digest := sha256.Sum256([]byte("a|1||b|h"))
	if got, want := wire.NewChecksum("h").OrderedBody(body), hex.EncodeToString(digest[:]); got != want {
		t.Fatalf("got %s, want %s", got, want)
	}
}

// serverTrimVectors mirrors the server_trim section of testdata/api_vectors.json:
// how the panel API trims request strings before it verifies the checksum.
type serverTrimVectors struct {
	SecretHash string `json:"secret_hash"`
	ServerTrim struct {
		Strings []struct {
			Name    string `json:"name"`
			Input   string `json:"input"`
			Trimmed string `json:"trimmed"`
		} `json:"strings"`
		Bodies []struct {
			Name     string          `json:"name"`
			Input    json.RawMessage `json:"input"`
			Body     json.RawMessage `json:"body"`
			Checksum string          `json:"checksum"`
		} `json:"bodies"`
	} `json:"server_trim"`
}

func loadServerTrimVectors(t *testing.T) serverTrimVectors {
	t.Helper()
	raw, err := os.ReadFile("testdata/api_vectors.json")
	if err != nil {
		t.Fatal(err)
	}
	var vectors serverTrimVectors
	if err := json.Unmarshal(raw, &vectors); err != nil {
		t.Fatal(err)
	}
	return vectors
}

func TestAPIVectorsFileIsTheSharedFixture(t *testing.T) {
	raw, err := os.ReadFile("testdata/api_vectors.json")
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(raw)
	if got := hex.EncodeToString(digest[:]); got != "614b0cac500b8b610cf8a527f0227a0819fc3f5f07ca973338c050cc98933334" {
		t.Fatalf("sha256 = %s - copy tests/Fixtures/api_vectors.json of the PHP SDK byte for byte", got)
	}
}

func TestAPIVectorsServerTrimStrings(t *testing.T) {
	vectors := loadServerTrimVectors(t)
	if len(vectors.ServerTrim.Strings) != 8 {
		t.Fatalf("%d string vectors, want 8", len(vectors.ServerTrim.Strings))
	}
	for _, vector := range vectors.ServerTrim.Strings {
		if got := wire.ServerTrim(vector.Input); got != vector.Trimmed {
			t.Errorf("%s: ServerTrim(%q) = %q, want %q", vector.Name, vector.Input, got, vector.Trimmed)
		}
	}
}

func TestAPIVectorsServerTrimBodies(t *testing.T) {
	vectors := loadServerTrimVectors(t)
	if len(vectors.ServerTrim.Bodies) != 4 {
		t.Fatalf("%d body vectors, want 4", len(vectors.ServerTrim.Bodies))
	}
	checksum := wire.NewChecksum(vectors.SecretHash)
	for _, vector := range vectors.ServerTrim.Bodies {
		input, ok := decodeOrdered(t, vector.Input).(*wire.Body)
		if !ok {
			t.Fatalf("%s: input is not an object", vector.Name)
		}
		want, ok := decodeOrdered(t, vector.Body).(*wire.Body)
		if !ok {
			t.Fatalf("%s: body is not an object", vector.Name)
		}
		trimmed := wire.TrimBody(input)
		if !reflect.DeepEqual(trimmed, want) {
			got, _ := trimmed.MarshalJSON()
			t.Errorf("%s: trimmed body = %s", vector.Name, got)
		}
		if got := checksum.OrderedBody(trimmed); got != vector.Checksum {
			t.Errorf("%s: checksum = %s, want %s", vector.Name, got, vector.Checksum)
		}
		if checksum.OrderedBody(input) == vector.Checksum {
			t.Errorf("%s: the untrimmed input must not give the checksum", vector.Name)
		}
	}
}
