package dpay

import (
	"context"
	"encoding/base64"
	"reflect"
	"testing"
)

func TestApplePayInitReturnsTheMerchantSession(t *testing.T) {
	session := `{"epochTimestamp":1790503500000,"merchantSessionIdentifier":"SSH-1","signature":"abc"}`
	client, _ := scriptedClient(t, okResponse(`{"success":true,"status":"success","message":{"redirectType":"FORM","redirectText":"`+
		base64.StdEncoding.EncodeToString([]byte(session))+`"}}`))
	result, err := client.Cards.ApplePay(context.Background(), "TX-1", &ApplePayRequest{DeviceInfo: cardDevice(), Init: true})
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]any{"epochTimestamp": float64(1790503500000), "merchantSessionIdentifier": "SSH-1", "signature": "abc"}
	if got := result.ApplePayMerchantSession(); !reflect.DeepEqual(got, want) {
		t.Fatalf("session = %#v", got)
	}
}

func TestApplePayMerchantSessionVectors(t *testing.T) {
	cases := []struct {
		redirectType string
		text         string
		want         map[string]any
	}{
		{"FORM", "eyJlcG9jaFRpbWVzdGFtcCI6MTc5MDUwMzUwMDAwMCwibWVyY2hhbnRTZXNzaW9uSWRlbnRpZmllciI6IlNTSC0xIn0=",
			map[string]any{"epochTimestamp": float64(1790503500000), "merchantSessionIdentifier": "SSH-1"}},
		// <form method="POST"></form> - a 3-D Secure form is no session
		{"FORM", "PGZvcm0gbWV0aG9kPSJQT1NUIj48L2Zvcm0+", nil},
		// JSON, but not an object
		{"FORM", base64.StdEncoding.EncodeToString([]byte(`[1,2]`)), nil},
		{"FORM", base64.StdEncoding.EncodeToString([]byte(`"text"`)), nil},
		{"FORM", base64.StdEncoding.EncodeToString([]byte(`{}`)), map[string]any{}},
		// only a FORM answer carries a session
		{"URL", base64.StdEncoding.EncodeToString([]byte(`{"a":1}`)), nil},
		{"FORM", "", nil},
	}
	for index, testCase := range cases {
		result := cardPaymentResultFromAPI(map[string]any{"success": true, "message": map[string]any{
			"redirectType": testCase.redirectType, "redirectText": testCase.text}})
		got := result.ApplePayMerchantSession()
		if !reflect.DeepEqual(got, testCase.want) {
			t.Errorf("case %d: session = %#v, want %#v", index, got, testCase.want)
		}
	}
	form := cardPaymentResultFromAPI(map[string]any{"message": map[string]any{"redirectType": "FORM", "redirectText": "PGZvcm0gbWV0aG9kPSJQT1NUIj48L2Zvcm0+"}})
	if form.ThreeDSFormHTML() != `<form method="POST"></form>` {
		t.Fatal("the 3-D Secure form must stay readable")
	}
}

func TestApplePaySendsTheEmail(t *testing.T) {
	client, doer := scriptedClient(t, okResponse(`{"success":true,"status":"success","message":{"redirectType":"SUCCESS"}}`))
	if _, err := client.Cards.ApplePay(context.Background(), "TX-1", &ApplePayRequest{
		DeviceInfo: cardDevice(), Token: String("APPLE-TOKEN"), Email: String("client@example.com"), ChannelID: Int(31),
	}); err != nil {
		t.Fatal(err)
	}
	if keys := keysInOrder(t, doer.bodies[0]); !reflect.DeepEqual(keys, []string{"email", "channelId", "xPayType", "xPayToken", "deviceInfo"}) {
		t.Fatalf("keys = %v", keys)
	}
	body := doer.lastBody(t)
	if body["email"] != "client@example.com" || body["xPayType"] != "APPLE_PAY" {
		t.Fatalf("body = %v", body)
	}
}
