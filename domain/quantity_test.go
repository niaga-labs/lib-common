package domain

import (
	"encoding/json"
	"testing"
)

func TestNewQuantity(t *testing.T) {
	q, err := NewQuantity(10)
	if err != nil {
		t.Fatalf("NewQuantity(10) error = %v", err)
	}
	if q.Int() != 10 {
		t.Errorf("Int() = %v, want 10", q.Int())
	}

	// Negative should error
	_, err = NewQuantity(-5)
	if err != ErrNegativeQuantity {
		t.Errorf("NewQuantity(-5) should return ErrNegativeQuantity")
	}
}

func TestMustQuantity(t *testing.T) {
	q := MustQuantity(5)
	if q.Int() != 5 {
		t.Errorf("MustQuantity(5) = %v, want 5", q.Int())
	}

	// Should panic for negative
	defer func() {
		if r := recover(); r == nil {
			t.Error("MustQuantity(-1) should panic")
		}
	}()
	MustQuantity(-1)
}

func TestQuantityWithUnit(t *testing.T) {
	q, err := QuantityWithUnit(100, "kg")
	if err != nil {
		t.Fatalf("QuantityWithUnit() error = %v", err)
	}
	if q.Int() != 100 || q.Unit() != "kg" {
		t.Errorf("QuantityWithUnit() = %v, want 100 kg", q)
	}
}

func TestQuantityAdd(t *testing.T) {
	a, _ := NewQuantity(10)
	b, _ := NewQuantity(5)

	result, err := a.Add(b)
	if err != nil {
		t.Fatalf("Add() error = %v", err)
	}
	if result.Int() != 15 {
		t.Errorf("Add() = %v, want 15", result.Int())
	}
}

func TestQuantitySubtract(t *testing.T) {
	a, _ := NewQuantity(10)
	b, _ := NewQuantity(3)

	result, err := a.Subtract(b)
	if err != nil {
		t.Fatalf("Subtract() error = %v", err)
	}
	if result.Int() != 7 {
		t.Errorf("Subtract() = %v, want 7", result.Int())
	}

	// Subtracting more than available should error
	c, _ := NewQuantity(20)
	_, err = a.Subtract(c)
	if err != ErrNegativeQuantity {
		t.Errorf("Subtract() should return ErrNegativeQuantity when result would be negative")
	}
}

func TestQuantityCanSubtract(t *testing.T) {
	a, _ := NewQuantity(10)
	b, _ := NewQuantity(5)
	c, _ := NewQuantity(15)

	if !a.CanSubtract(b) {
		t.Error("CanSubtract() should return true when subtraction is valid")
	}
	if a.CanSubtract(c) {
		t.Error("CanSubtract() should return false when subtraction would go negative")
	}
}

func TestQuantityMultiply(t *testing.T) {
	q, _ := NewQuantity(10)

	result, err := q.Multiply(3)
	if err != nil {
		t.Fatalf("Multiply() error = %v", err)
	}
	if result.Int() != 30 {
		t.Errorf("Multiply() = %v, want 30", result.Int())
	}

	// Negative factor should error
	_, err = q.Multiply(-2)
	if err != ErrNegativeQuantity {
		t.Error("Multiply() with negative factor should return ErrNegativeQuantity")
	}
}

func TestQuantityComparison(t *testing.T) {
	a, _ := NewQuantity(10)
	b, _ := NewQuantity(5)
	c, _ := NewQuantity(10)

	if !a.Equals(c) {
		t.Error("Equals() should return true for same value")
	}
	if a.Equals(b) {
		t.Error("Equals() should return false for different values")
	}
	if !a.GreaterThan(b) {
		t.Error("GreaterThan() should return true")
	}
	if !b.LessThan(a) {
		t.Error("LessThan() should return true")
	}
}

func TestQuantityIsZero(t *testing.T) {
	zero := ZeroQuantity()
	if !zero.IsZero() {
		t.Error("ZeroQuantity() should be zero")
	}

	nonZero, _ := NewQuantity(1)
	if nonZero.IsZero() {
		t.Error("NewQuantity(1) should not be zero")
	}
}

func TestQuantityJSON(t *testing.T) {
	q, _ := NewQuantity(25)

	data, err := json.Marshal(q)
	if err != nil {
		t.Fatalf("MarshalJSON() error = %v", err)
	}

	var parsed Quantity
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("UnmarshalJSON() error = %v", err)
	}

	if !q.Equals(parsed) {
		t.Errorf("JSON round trip failed: got %v, want %v", parsed, q)
	}

	// Test with unit
	qUnit, _ := QuantityWithUnit(50, "pcs")
	dataUnit, _ := json.Marshal(qUnit)

	var parsedUnit Quantity
	if err := json.Unmarshal(dataUnit, &parsedUnit); err != nil {
		t.Fatalf("UnmarshalJSON(with unit) error = %v", err)
	}

	if !qUnit.Equals(parsedUnit) {
		t.Errorf("JSON round trip with unit failed: got %v, want %v", parsedUnit, qUnit)
	}
}

func TestQuantityString(t *testing.T) {
	q, _ := NewQuantity(10)
	if q.String() != "10" {
		t.Errorf("String() = %v, want '10'", q.String())
	}

	qUnit, _ := QuantityWithUnit(50, "kg")
	if qUnit.String() != "50 kg" {
		t.Errorf("String() = %v, want '50 kg'", qUnit.String())
	}
}

func TestSumQuantities(t *testing.T) {
	q1, _ := NewQuantity(10)
	q2, _ := NewQuantity(20)
	q3, _ := NewQuantity(30)

	sum, err := SumQuantities(q1, q2, q3)
	if err != nil {
		t.Fatalf("SumQuantities() error = %v", err)
	}
	if sum.Int() != 60 {
		t.Errorf("SumQuantities() = %v, want 60", sum.Int())
	}

	// Empty should return zero
	empty, _ := SumQuantities()
	if !empty.IsZero() {
		t.Error("SumQuantities() with no args should return zero")
	}
}
