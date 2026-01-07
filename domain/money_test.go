package domain

import (
	"encoding/json"
	"testing"
)

func TestNewMoney(t *testing.T) {
	tests := []struct {
		name     string
		amount   float64
		currency string
		wantAmt  float64
		wantCur  string
	}{
		{"basic MYR", 100.50, "MYR", 100.50, "MYR"},
		{"USD", 50.99, "USD", 50.99, "USD"},
		{"default currency", 25.00, "", 25.00, "MYR"},
		{"zero", 0, "MYR", 0, "MYR"},
		{"negative", -50.00, "MYR", -50.00, "MYR"},
		{"rounding", 10.555, "MYR", 10.56, "MYR"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := NewMoney(tt.amount, tt.currency)
			if m.Amount() != tt.wantAmt {
				t.Errorf("Amount() = %v, want %v", m.Amount(), tt.wantAmt)
			}
			if m.Currency() != tt.wantCur {
				t.Errorf("Currency() = %v, want %v", m.Currency(), tt.wantCur)
			}
		})
	}
}

func TestMoneyConvenienceConstructors(t *testing.T) {
	myr := MYR(100)
	if myr.Currency() != CurrencyMYR || myr.Amount() != 100 {
		t.Errorf("MYR() = %v, want MYR 100.00", myr)
	}

	usd := USD(50)
	if usd.Currency() != CurrencyUSD || usd.Amount() != 50 {
		t.Errorf("USD() = %v, want USD 50.00", usd)
	}

	sgd := SGD(75)
	if sgd.Currency() != CurrencySGD || sgd.Amount() != 75 {
		t.Errorf("SGD() = %v, want SGD 75.00", sgd)
	}

	zero := Zero("MYR")
	if !zero.IsZero() {
		t.Errorf("Zero() should be zero")
	}

	cents := FromCents(15050, "MYR")
	if cents.Amount() != 150.50 {
		t.Errorf("FromCents(15050) = %v, want 150.50", cents.Amount())
	}
}

func TestMoneyAdd(t *testing.T) {
	a := MYR(100)
	b := MYR(50.50)

	result, err := a.Add(b)
	if err != nil {
		t.Fatalf("Add() error = %v", err)
	}
	if result.Amount() != 150.50 {
		t.Errorf("Add() = %v, want 150.50", result.Amount())
	}

	// Different currencies should error
	c := USD(25)
	_, err = a.Add(c)
	if err == nil {
		t.Error("Add() with different currencies should error")
	}
}

func TestMoneySubtract(t *testing.T) {
	a := MYR(100)
	b := MYR(30.50)

	result, err := a.Subtract(b)
	if err != nil {
		t.Fatalf("Subtract() error = %v", err)
	}
	if result.Amount() != 69.50 {
		t.Errorf("Subtract() = %v, want 69.50", result.Amount())
	}
}

func TestMoneyMultiply(t *testing.T) {
	m := MYR(100)

	result := m.Multiply(1.5)
	if result.Amount() != 150 {
		t.Errorf("Multiply(1.5) = %v, want 150", result.Amount())
	}

	result = m.Multiply(0.1)
	if result.Amount() != 10 {
		t.Errorf("Multiply(0.1) = %v, want 10", result.Amount())
	}
}

func TestMoneyPercentage(t *testing.T) {
	m := MYR(200)

	result := m.Percentage(10)
	if result.Amount() != 20 {
		t.Errorf("Percentage(10) = %v, want 20", result.Amount())
	}

	result = m.Percentage(50)
	if result.Amount() != 100 {
		t.Errorf("Percentage(50) = %v, want 100", result.Amount())
	}
}

func TestMoneyComparison(t *testing.T) {
	a := MYR(100)
	b := MYR(50)
	c := MYR(100)

	if !a.Equals(c) {
		t.Error("Equals() should return true for same amount")
	}
	if a.Equals(b) {
		t.Error("Equals() should return false for different amounts")
	}
	if !a.GreaterThan(b) {
		t.Error("GreaterThan() should return true")
	}
	if !b.LessThan(a) {
		t.Error("LessThan() should return true")
	}
}

func TestMoneyJSON(t *testing.T) {
	m := MYR(150.50)

	data, err := json.Marshal(m)
	if err != nil {
		t.Fatalf("MarshalJSON() error = %v", err)
	}

	var parsed Money
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("UnmarshalJSON() error = %v", err)
	}

	if !m.Equals(parsed) {
		t.Errorf("JSON round trip failed: got %v, want %v", parsed, m)
	}

	// Test unmarshaling plain number
	var fromNumber Money
	if err := json.Unmarshal([]byte("99.99"), &fromNumber); err != nil {
		t.Fatalf("UnmarshalJSON(number) error = %v", err)
	}
	if fromNumber.Amount() != 99.99 {
		t.Errorf("UnmarshalJSON(number) = %v, want 99.99", fromNumber.Amount())
	}
}

func TestMoneyString(t *testing.T) {
	m := MYR(150.50)
	if m.String() != "MYR 150.50" {
		t.Errorf("String() = %v, want 'MYR 150.50'", m.String())
	}
}

func TestSumMoney(t *testing.T) {
	values := []Money{MYR(10), MYR(20), MYR(30)}
	sum, err := SumMoney(values...)
	if err != nil {
		t.Fatalf("SumMoney() error = %v", err)
	}
	if sum.Amount() != 60 {
		t.Errorf("SumMoney() = %v, want 60", sum.Amount())
	}

	// Empty should return zero
	empty, _ := SumMoney()
	if !empty.IsZero() {
		t.Error("SumMoney() with no args should return zero")
	}
}
