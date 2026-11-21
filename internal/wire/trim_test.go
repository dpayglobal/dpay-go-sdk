package wire

import (
	"reflect"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestServerTrimCodePointsAreTheSixtyFiveOfTheAPI(t *testing.T) {
	if len(ServerTrimCodePoints) != 65 || len(serverTrimSet) != 65 {
		t.Fatalf("%d code points, %d distinct, want 65", len(ServerTrimCodePoints), len(serverTrimSet))
	}
	for index := 1; index < len(ServerTrimCodePoints); index++ {
		if ServerTrimCodePoints[index-1] >= ServerTrimCodePoints[index] {
			t.Fatalf("code points must be sorted and distinct at %d", index)
		}
	}
}

// Every scalar value is either trimmed from both ends (and only then) or kept,
// never touched inside the string.
func TestServerTrimOverEveryCodePoint(t *testing.T) {
	trimmedCount := 0
	for codePoint := rune(0); codePoint <= utf8.MaxRune; codePoint++ {
		if codePoint >= 0xD800 && codePoint <= 0xDFFF {
			continue
		}
		character := string(codePoint)
		input := character + "x" + character + "y" + character
		got := ServerTrim(input)
		if isServerTrimmed(codePoint) {
			trimmedCount++
			if want := "x" + character + "y"; got != want {
				t.Fatalf("U+%04X: %q, want %q", codePoint, got, want)
			}
		} else if got != input {
			t.Fatalf("U+%04X must not be trimmed: %q", codePoint, got)
		}
	}
	if trimmedCount != 65 {
		t.Fatalf("%d code points trimmed, want 65", trimmedCount)
	}
}

// strings.TrimSpace (unicode.IsSpace) is not what the API does.
func TestServerTrimIsNotTrimSpace(t *testing.T) {
	cases := map[string]string{
		"\x00T\u200b":            "T",
		"\ufeffT\u00ad":          "T",
		"\u2060T\U000E0020":      "T",
		"\x1cT\x1f":              "\x1cT\x1f",
		"\u0008T":                "\u0008T",
		"  T  \n":                "T",
		"\u3000\u00a0\u2028":     "",
		"":                       "",
		"\u00a0a \u00a0 b\u00a0": "a \u00a0 b",
	}
	for input, want := range cases {
		if got := ServerTrim(input); got != want {
			t.Errorf("ServerTrim(%q) = %q, want %q", input, got, want)
		}
	}
	if strings.TrimSpace("\x00T\u200b") == "T" {
		t.Fatal("the premise of this test changed")
	}
}

func TestServerTrimKeepsInvalidUTF8(t *testing.T) {
	for _, input := range []string{" \xff ", "\xc3", " a\xe2\x80"} {
		if got := ServerTrim(input); got != input {
			t.Errorf("ServerTrim(%q) = %q, want it unchanged", input, got)
		}
	}
}

func TestTrimStringsCopiesNestedValuesKeepingOrderAndTypes(t *testing.T) {
	webhook := NewBody()
	webhook.Set("url", " https://shop.example/hook\n")
	webhook.Set("events", []string{"\u200brefund.succeeded"})
	body := NewBody()
	body.Set("service", "\ufeffMyShop ")
	body.Set("withdraw_id", int64(7))
	body.Set("flag", true)
	body.Set("none", nil)
	body.Set("webhook", webhook)
	body.Set("list", []any{" a ", 1, map[string]any{"k": "\tb\t"}})

	trimmed := TrimBody(body)
	if got := encode(t, trimmed); got != `{"service":"MyShop","withdraw_id":7,"flag":true,"none":null,`+
		`"webhook":{"url":"https://shop.example/hook","events":["refund.succeeded"]},"list":["a",1,{"k":"b"}]}` {
		t.Fatalf("trimmed = %s", got)
	}
	if url, _ := webhook.Get("url"); url != " https://shop.example/hook\n" {
		t.Fatalf("the nested original was modified: %q", url)
	}
	if !reflect.DeepEqual(body.Keys(), trimmed.Keys()) {
		t.Fatalf("keys = %v", trimmed.Keys())
	}
	if value, _ := body.Get("service"); value != "\ufeffMyShop " {
		t.Fatalf("the original body was modified: %q", value)
	}
	if TrimStrings(42) != 42 || TrimStrings(nil) != nil {
		t.Fatal("non-strings must be kept")
	}
}
