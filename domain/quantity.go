package domain

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
)

// Common quantity errors
var (
	ErrNegativeQuantity = errors.New("quantity cannot be negative")
	ErrQuantityOverflow = errors.New("quantity operation would cause overflow")
)

// Quantity represents a non-negative quantity value with an optional unit.
type Quantity struct {
	value int
	unit  string // Optional unit (e.g., "pcs", "kg", "m")
}

// quantityJSON is used for JSON marshaling/unmarshaling
type quantityJSON struct {
	Value int    `json:"value"`
	Unit  string `json:"unit,omitempty"`
}

// NewQuantity creates a new Quantity value.
// Returns an error if the value is negative.
func NewQuantity(value int) (Quantity, error) {
	if value < 0 {
		return Quantity{}, ErrNegativeQuantity
	}
	return Quantity{value: value}, nil
}

// MustQuantity creates a new Quantity value.
// Panics if the value is negative.
func MustQuantity(value int) Quantity {
	q, err := NewQuantity(value)
	if err != nil {
		panic(err)
	}
	return q
}

// QuantityWithUnit creates a new Quantity with a unit.
// Returns an error if the value is negative.
func QuantityWithUnit(value int, unit string) (Quantity, error) {
	if value < 0 {
		return Quantity{}, ErrNegativeQuantity
	}
	return Quantity{value: value, unit: unit}, nil
}

// MustQuantityWithUnit creates a new Quantity with a unit.
// Panics if the value is negative.
func MustQuantityWithUnit(value int, unit string) Quantity {
	q, err := QuantityWithUnit(value, unit)
	if err != nil {
		panic(err)
	}
	return q
}

// ZeroQuantity returns a zero Quantity.
func ZeroQuantity() Quantity {
	return Quantity{value: 0}
}

// RawValue returns the quantity value as an int.
func (q Quantity) RawValue() int {
	return q.value
}

// Unit returns the quantity unit.
func (q Quantity) Unit() string {
	return q.unit
}

// String returns a string representation of the quantity.
func (q Quantity) String() string {
	if q.unit != "" {
		return fmt.Sprintf("%d %s", q.value, q.unit)
	}
	return fmt.Sprintf("%d", q.value)
}

// Int returns the quantity value as an int (alias for RawValue).
func (q Quantity) Int() int {
	return q.RawValue()
}

// Add returns a new Quantity that is the sum of q and other.
// Units must match if both are specified.
func (q Quantity) Add(other Quantity) (Quantity, error) {
	if q.unit != "" && other.unit != "" && q.unit != other.unit {
		return Quantity{}, fmt.Errorf("cannot add quantities with different units: %s and %s", q.unit, other.unit)
	}
	unit := q.unit
	if unit == "" {
		unit = other.unit
	}
	newValue := q.value + other.value
	if newValue < q.value { // Overflow check
		return Quantity{}, ErrQuantityOverflow
	}
	return Quantity{value: newValue, unit: unit}, nil
}

// MustAdd returns a new Quantity that is the sum of q and other.
// Panics on error.
func (q Quantity) MustAdd(other Quantity) Quantity {
	result, err := q.Add(other)
	if err != nil {
		panic(err)
	}
	return result
}

// Subtract returns a new Quantity that is q minus other.
// Returns an error if the result would be negative.
func (q Quantity) Subtract(other Quantity) (Quantity, error) {
	if q.unit != "" && other.unit != "" && q.unit != other.unit {
		return Quantity{}, fmt.Errorf("cannot subtract quantities with different units: %s and %s", q.unit, other.unit)
	}
	if other.value > q.value {
		return Quantity{}, ErrNegativeQuantity
	}
	unit := q.unit
	if unit == "" {
		unit = other.unit
	}
	return Quantity{value: q.value - other.value, unit: unit}, nil
}

// MustSubtract returns a new Quantity that is q minus other.
// Panics on error.
func (q Quantity) MustSubtract(other Quantity) Quantity {
	result, err := q.Subtract(other)
	if err != nil {
		panic(err)
	}
	return result
}

// CanSubtract returns true if other can be subtracted from q without going negative.
func (q Quantity) CanSubtract(other Quantity) bool {
	if q.unit != "" && other.unit != "" && q.unit != other.unit {
		return false
	}
	return q.value >= other.value
}

// Multiply returns a new Quantity that is q multiplied by the factor.
// Returns an error if the result would be negative.
func (q Quantity) Multiply(factor int) (Quantity, error) {
	if factor < 0 {
		return Quantity{}, ErrNegativeQuantity
	}
	return Quantity{value: q.value * factor, unit: q.unit}, nil
}

// MustMultiply returns a new Quantity that is q multiplied by the factor.
// Panics on error.
func (q Quantity) MustMultiply(factor int) Quantity {
	result, err := q.Multiply(factor)
	if err != nil {
		panic(err)
	}
	return result
}

// Equals returns true if q and other have the same value and unit.
func (q Quantity) Equals(other Quantity) bool {
	return q.value == other.value && q.unit == other.unit
}

// GreaterThan returns true if q is greater than other.
// Returns false if units don't match.
func (q Quantity) GreaterThan(other Quantity) bool {
	if q.unit != "" && other.unit != "" && q.unit != other.unit {
		return false
	}
	return q.value > other.value
}

// GreaterThanOrEqual returns true if q is greater than or equal to other.
// Returns false if units don't match.
func (q Quantity) GreaterThanOrEqual(other Quantity) bool {
	if q.unit != "" && other.unit != "" && q.unit != other.unit {
		return false
	}
	return q.value >= other.value
}

// LessThan returns true if q is less than other.
// Returns false if units don't match.
func (q Quantity) LessThan(other Quantity) bool {
	if q.unit != "" && other.unit != "" && q.unit != other.unit {
		return false
	}
	return q.value < other.value
}

// LessThanOrEqual returns true if q is less than or equal to other.
// Returns false if units don't match.
func (q Quantity) LessThanOrEqual(other Quantity) bool {
	if q.unit != "" && other.unit != "" && q.unit != other.unit {
		return false
	}
	return q.value <= other.value
}

// IsZero returns true if the quantity is zero.
func (q Quantity) IsZero() bool {
	return q.value == 0
}

// WithUnit returns a new Quantity with the specified unit.
func (q Quantity) WithUnit(unit string) Quantity {
	return Quantity{value: q.value, unit: unit}
}

// MarshalJSON implements json.Marshaler.
func (q Quantity) MarshalJSON() ([]byte, error) {
	if q.unit == "" {
		return json.Marshal(q.value)
	}
	return json.Marshal(quantityJSON{
		Value: q.value,
		Unit:  q.unit,
	})
}

// UnmarshalJSON implements json.Unmarshaler.
func (q *Quantity) UnmarshalJSON(data []byte) error {
	// Try unmarshaling as a plain number first
	var value int
	if err := json.Unmarshal(data, &value); err == nil {
		if value < 0 {
			return ErrNegativeQuantity
		}
		*q = Quantity{value: value}
		return nil
	}

	// Try unmarshaling as an object
	var qj quantityJSON
	if err := json.Unmarshal(data, &qj); err != nil {
		return err
	}
	if qj.Value < 0 {
		return ErrNegativeQuantity
	}
	*q = Quantity{value: qj.Value, unit: qj.Unit}
	return nil
}

// Value implements driver.Valuer for database storage.
func (q Quantity) Value() (driver.Value, error) {
	return int64(q.value), nil
}

// Scan implements sql.Scanner for database retrieval.
func (q *Quantity) Scan(value interface{}) error {
	if value == nil {
		*q = ZeroQuantity()
		return nil
	}

	switch v := value.(type) {
	case int64:
		if v < 0 {
			return ErrNegativeQuantity
		}
		*q = Quantity{value: int(v)}
	case int:
		if v < 0 {
			return ErrNegativeQuantity
		}
		*q = Quantity{value: v}
	case float64:
		intVal := int(v)
		if intVal < 0 {
			return ErrNegativeQuantity
		}
		*q = Quantity{value: intVal}
	default:
		return fmt.Errorf("cannot scan %T into Quantity", value)
	}
	return nil
}

// Max returns the larger of q and other.
// Returns q if units don't match.
func (q Quantity) Max(other Quantity) Quantity {
	if q.unit != "" && other.unit != "" && q.unit != other.unit {
		return q
	}
	if other.value > q.value {
		return other
	}
	return q
}

// Min returns the smaller of q and other.
// Returns q if units don't match.
func (q Quantity) Min(other Quantity) Quantity {
	if q.unit != "" && other.unit != "" && q.unit != other.unit {
		return q
	}
	if other.value < q.value {
		return other
	}
	return q
}

// SumQuantities returns the sum of all quantity values.
// All values must have the same unit as the first value.
// Returns zero if the slice is empty.
func SumQuantities(values ...Quantity) (Quantity, error) {
	if len(values) == 0 {
		return ZeroQuantity(), nil
	}

	result := values[0]
	for i := 1; i < len(values); i++ {
		sum, err := result.Add(values[i])
		if err != nil {
			return Quantity{}, err
		}
		result = sum
	}
	return result, nil
}
