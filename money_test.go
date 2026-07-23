package dpay

import "testing"

func TestMoneyString(t *testing.T) {
	cases := map[int64]string{
		1050:      "10.50",
		1000:      "10.00",
		5:         "0.05",
		0:         "0.00",
		-2000:     "-20.00",
		-5:        "-0.05",
		123456789: "1234567.89",
	}
	for minor, want := range cases {
		if got := PLN(minor).String(); got != want {
			t.Errorf("PLN(%d).String() = %q, want %q", minor, got, want)
		}
	}
}

func TestParseMoney(t *testing.T) {
	cases := map[string]int64{
		"10.50": 1050,
		"10":    1000,
		"10.5":  1050,
		"0.05":  5,
		"-20":   -2000,
		"-0.05": -5,
	}
	for decimal, want := range cases {
		money, err := ParseMoney(decimal, CurrencyPLN)
		if err != nil {
			t.Fatalf("ParseMoney(%q): %v", decimal, err)
		}
		if money.Minor() != want {
			t.Errorf("ParseMoney(%q).Minor() = %d, want %d", decimal, money.Minor(), want)
		}
	}
	for _, bad := range []string{"", "abc", "10.123", "1,5", " 10", "+10"} {
		if _, err := ParseMoney(bad, CurrencyPLN); err == nil {
			t.Errorf("ParseMoney(%q) must fail", bad)
		}
	}
}

func TestNewMoneyRejectsInvalidCurrency(t *testing.T) {
	if _, err := NewMoney(100, "pln"); err == nil {
		t.Fatal("lowercase currency must fail")
	}
	if _, err := NewMoney(100, "PLNX"); err == nil {
		t.Fatal("four-letter currency must fail")
	}
	if _, err := NewMoney(100, CurrencyEUR); err != nil {
		t.Fatalf("EUR must pass: %v", err)
	}
}

func TestMoneyFromAPI(t *testing.T) {
	cases := []struct {
		in    any
		want  int64
		valid bool
	}{
		{30, 3000, true},
		{0, 0, true},
		{-7, -700, true},
		{10.5, 1050, true},
		{8.285, 829, true},
		{0.1, 10, true},
		{"30", 3000, true},
		{"10.00", 1000, true},
		{"10.5", 1050, true},
		{"10.50", 1050, true},
		{"0.05", 5, true},
		{"-0.05", -5, true},
		{"abc", 0, false},
		{nil, 0, false},
		{[]any{}, 0, false},
	}
	for _, c := range cases {
		money, ok := moneyFromAPI(c.in, CurrencyPLN)
		if ok != c.valid {
			t.Errorf("moneyFromAPI(%v) ok = %v, want %v", c.in, ok, c.valid)
			continue
		}
		if ok && money.Minor() != c.want {
			t.Errorf("moneyFromAPI(%v) = %d, want %d", c.in, money.Minor(), c.want)
		}
	}
}

func TestMoneyFloat(t *testing.T) {
	cases := map[int64]float64{
		2999:      29.99,
		1000:      10,
		5:         0.05,
		-2000:     -20,
		123456789: 1234567.89,
	}
	for minor, want := range cases {
		if got := moneyFloat(PLN(minor)); got != want {
			t.Errorf("moneyFloat(PLN(%d)) = %v, want %v", minor, got, want)
		}
	}
}

func TestMoneyEqualAndSigns(t *testing.T) {
	if !PLN(100).Equal(PLN(100)) {
		t.Fatal("equal amounts must compare equal")
	}
	eur, _ := NewMoney(100, CurrencyEUR)
	if PLN(100).Equal(eur) {
		t.Fatal("different currencies must not compare equal")
	}
	if !PLN(-1).IsNegative() || PLN(1).IsNegative() {
		t.Fatal("IsNegative broken")
	}
	if !PLN(0).IsZero() {
		t.Fatal("IsZero broken")
	}
}

func TestFieldReaders(t *testing.T) {
	data := map[string]any{
		"s": "text", "n": float64(42), "b": true, "f": 10.5,
		"obj": map[string]any{"k": "v"}, "arr": []any{1, 2},
		"numericString": "7", "wrongType": []any{},
	}
	if stringField(data, "s") != "text" || stringField(data, "missing") != "" {
		t.Fatal("stringField broken")
	}
	if stringField(data, "n") != "42" {
		t.Fatalf("stringField on number = %q, want \"42\"", stringField(data, "n"))
	}
	if stringField(data, "wrongType") != "" {
		t.Fatal("non-scalar must read as empty")
	}
	if !boolField(data, "b") || boolField(data, "missing") {
		t.Fatal("boolField broken")
	}
	if intField(data, "n") != 42 || intField(data, "numericString") != 7 || intField(data, "missing") != 0 {
		t.Fatal("intField broken")
	}
	if objectField(data, "obj")["k"] != "v" || objectField(data, "missing") == nil {
		t.Fatal("objectField must return an empty map for a missing key")
	}
	if len(arrayField(data, "arr")) != 2 || len(arrayField(data, "missing")) != 0 {
		t.Fatal("arrayField broken")
	}
	if got := moneyField(data, "f", CurrencyPLN); got.Minor() != 1050 {
		t.Fatalf("moneyField = %d", got.Minor())
	}
	if got := moneyField(data, "wrongType", CurrencyPLN); got.Minor() != 0 || got.Currency() != CurrencyPLN {
		t.Fatal("unparsable amount must fall back to zero PLN")
	}
}
