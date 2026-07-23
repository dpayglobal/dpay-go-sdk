package php

import (
	"math"
	"testing"
)

func TestStrval(t *testing.T) {
	cases := []struct {
		in   any
		want string
	}{
		{10, "10"},
		{int64(42), "42"},
		{10.0, "10"},
		{10.5, "10.5"},
		{0.1, "0.1"},
		{1.0 / 3.0, "0.33333333333333"},
		{1e25, "1.0E+25"},
		{1e-7, "1.0E-7"},
		{math.Copysign(0, -1), "-0"},
		{100.0, "100"},
		{828.5, "828.5"},
		{true, "1"},
		{false, ""},
		{nil, ""},
		{"abc", "abc"},
		{"", ""},
	}
	for _, c := range cases {
		if got := Strval(c.in); got != c.want {
			t.Errorf("Strval(%v) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestRoundHalfAwayFromZero(t *testing.T) {
	cases := []struct {
		in   float64
		want int64
	}{
		{828.5, 829},
		{-828.5, -829},
		{0.5, 1},
		{1.5, 2},
		{2.5, 3},
		{-0.5, -1},
		{2998.5, 2999},
	}
	for _, c := range cases {
		if got := Round(c.in); got != c.want {
			t.Errorf("Round(%v) = %d, want %d", c.in, got, c.want)
		}
	}
}

func TestInt(t *testing.T) {
	cases := []struct {
		in   any
		want int64
	}{
		{nil, 0},
		{true, 1},
		{false, 0},
		{42, 42},
		{10.9, 10},
		{-10.9, -10},
		{"12abc", 12},
		{"  -7x", -7},
		{"abc", 0},
		{math.NaN(), 0},
	}
	for _, c := range cases {
		if got := Int(c.in); got != c.want {
			t.Errorf("Int(%v) = %d, want %d", c.in, got, c.want)
		}
	}
}

func TestEncodeWireMode(t *testing.T) {
	got, err := Encode(map[string]string{"x": "ą<>&"}, false)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != `{"x":"ą<>&"}` {
		t.Fatalf("Encode = %s", got)
	}
	got, _ = Encode("https://a/b", false)
	if string(got) != `"https://a/b"` {
		t.Fatalf("Encode slashes = %s", got)
	}
}

func TestEncodeCardMode(t *testing.T) {
	got, err := Encode(map[string]string{"u": "https://a/b"}, true)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != `{"u":"https:\/\/a\/b"}` {
		t.Fatalf("Encode = %s", got)
	}
	got, _ = Encode(map[string]string{"x": "ą<>&"}, true)
	if string(got) != "{\"x\":\"\\u0105<>&\"}" {
		t.Fatalf("Encode non-ascii = %s", got)
	}
	got, _ = Encode("😀", true)
	if string(got) != "\"\\ud83d\\ude00\"" {
		t.Fatalf("Encode surrogate pair = %s", got)
	}
}

func TestEncodeFloatsMatchPHP(t *testing.T) {
	cases := []struct {
		in   float64
		want string
	}{
		{10.0, "10"},
		{10.5, "10.5"},
		{29.99, "29.99"},
		{-20.0, "-20"},
		{0.05, "0.05"},
		{1234567.89, "1234567.89"},
	}
	for _, c := range cases {
		got, err := Encode(c.in, false)
		if err != nil {
			t.Fatal(err)
		}
		if string(got) != c.want {
			t.Errorf("Encode(%v) = %s, want %s", c.in, got, c.want)
		}
	}
}

func TestIsNumeric(t *testing.T) {
	for _, v := range []any{1, 1.5, "1", " 2.5 ", "-3"} {
		if !IsNumeric(v) {
			t.Errorf("IsNumeric(%v) = false", v)
		}
	}
	for _, v := range []any{nil, true, "", "abc", "1abc"} {
		if IsNumeric(v) {
			t.Errorf("IsNumeric(%v) = true", v)
		}
	}
}

func TestIsScalar(t *testing.T) {
	for _, v := range []any{1, 1.5, "x", true} {
		if !IsScalar(v) {
			t.Errorf("IsScalar(%v) = false", v)
		}
	}
	for _, v := range []any{nil, map[string]any{}, []any{}} {
		if IsScalar(v) {
			t.Errorf("IsScalar(%v) = true", v)
		}
	}
}
