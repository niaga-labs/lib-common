// Package domain provides shared domain primitives for DDD/Clean Architecture.
// These value objects should be used across all microservices to ensure
// consistent handling of common domain concepts.
package domain

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"math"
)

// Supported currencies
const (
	CurrencyMYR = "MYR" // Malaysian Ringgit (default)
	CurrencyUSD = "USD" // US Dollar
	CurrencySGD = "SGD" // Singapore Dollar
)

// DefaultCurrency is the default currency for money operations
const DefaultCurrency = CurrencyMYR

// Money represents a monetary value with currency.
// Internally stores amount as cents (int64) to avoid floating-point precision issues.
type Money struct {
	amount   int64  // Amount in smallest currency unit (cents)
	currency string // ISO 4217 currency code
}

// moneyJSON is used for JSON marshaling/unmarshaling
type moneyJSON struct {
	Amount   float64 `json:"amount"`
	Currency string  `json:"currency"`
}

// NewMoney creates a new Money value from a float64 amount and currency.
// The amount is converted to cents internally.
func NewMoney(amount float64, currency string) Money {
	if currency == "" {
		currency = DefaultCurrency
	}
	// Round to 2 decimal places and convert to cents
	cents := int64(math.Round(amount * 100))
	return Money{
		amount:   cents,
		currency: currency,
	}
}

// MYR creates a Money value in Malaysian Ringgit.
func MYR(amount float64) Money {
	return NewMoney(amount, CurrencyMYR)
}

// USD creates a Money value in US Dollars.
func USD(amount float64) Money {
	return NewMoney(amount, CurrencyUSD)
}

// SGD creates a Money value in Singapore Dollars.
func SGD(amount float64) Money {
	return NewMoney(amount, CurrencySGD)
}

// Zero returns a zero Money value for the given currency.
func Zero(currency string) Money {
	if currency == "" {
		currency = DefaultCurrency
	}
	return Money{amount: 0, currency: currency}
}

// FromCents creates a Money value from cents.
func FromCents(cents int64, currency string) Money {
	if currency == "" {
		currency = DefaultCurrency
	}
	return Money{amount: cents, currency: currency}
}

// Amount returns the amount as a float64.
func (m Money) Amount() float64 {
	return float64(m.amount) / 100
}

// Cents returns the amount in cents.
func (m Money) Cents() int64 {
	return m.amount
}

// Currency returns the currency code.
func (m Money) Currency() string {
	return m.currency
}

// String returns a formatted string representation (e.g., "MYR 150.00").
func (m Money) String() string {
	return fmt.Sprintf("%s %.2f", m.currency, m.Amount())
}

// Float64 returns the amount as a float64 (alias for Amount).
func (m Money) Float64() float64 {
	return m.Amount()
}

// Add returns a new Money that is the sum of m and other.
// Returns an error if currencies don't match.
func (m Money) Add(other Money) (Money, error) {
	if m.currency != other.currency {
		return Money{}, fmt.Errorf("cannot add money with different currencies: %s and %s", m.currency, other.currency)
	}
	return Money{
		amount:   m.amount + other.amount,
		currency: m.currency,
	}, nil
}

// MustAdd returns a new Money that is the sum of m and other.
// Panics if currencies don't match.
func (m Money) MustAdd(other Money) Money {
	result, err := m.Add(other)
	if err != nil {
		panic(err)
	}
	return result
}

// Subtract returns a new Money that is m minus other.
// Returns an error if currencies don't match.
func (m Money) Subtract(other Money) (Money, error) {
	if m.currency != other.currency {
		return Money{}, fmt.Errorf("cannot subtract money with different currencies: %s and %s", m.currency, other.currency)
	}
	return Money{
		amount:   m.amount - other.amount,
		currency: m.currency,
	}, nil
}

// MustSubtract returns a new Money that is m minus other.
// Panics if currencies don't match.
func (m Money) MustSubtract(other Money) Money {
	result, err := m.Subtract(other)
	if err != nil {
		panic(err)
	}
	return result
}

// Multiply returns a new Money that is m multiplied by the factor.
func (m Money) Multiply(factor float64) Money {
	newAmount := int64(math.Round(float64(m.amount) * factor))
	return Money{
		amount:   newAmount,
		currency: m.currency,
	}
}

// Divide returns a new Money that is m divided by the divisor.
// Returns an error if divisor is zero.
func (m Money) Divide(divisor float64) (Money, error) {
	if divisor == 0 {
		return Money{}, errors.New("cannot divide money by zero")
	}
	newAmount := int64(math.Round(float64(m.amount) / divisor))
	return Money{
		amount:   newAmount,
		currency: m.currency,
	}, nil
}

// Percentage returns a new Money that is the given percentage of m.
// e.g., m.Percentage(10) returns 10% of m.
func (m Money) Percentage(percent float64) Money {
	return m.Multiply(percent / 100)
}

// Equals returns true if m and other have the same amount and currency.
func (m Money) Equals(other Money) bool {
	return m.amount == other.amount && m.currency == other.currency
}

// GreaterThan returns true if m is greater than other.
// Returns false if currencies don't match.
func (m Money) GreaterThan(other Money) bool {
	if m.currency != other.currency {
		return false
	}
	return m.amount > other.amount
}

// GreaterThanOrEqual returns true if m is greater than or equal to other.
// Returns false if currencies don't match.
func (m Money) GreaterThanOrEqual(other Money) bool {
	if m.currency != other.currency {
		return false
	}
	return m.amount >= other.amount
}

// LessThan returns true if m is less than other.
// Returns false if currencies don't match.
func (m Money) LessThan(other Money) bool {
	if m.currency != other.currency {
		return false
	}
	return m.amount < other.amount
}

// LessThanOrEqual returns true if m is less than or equal to other.
// Returns false if currencies don't match.
func (m Money) LessThanOrEqual(other Money) bool {
	if m.currency != other.currency {
		return false
	}
	return m.amount <= other.amount
}

// IsZero returns true if the amount is zero.
func (m Money) IsZero() bool {
	return m.amount == 0
}

// IsPositive returns true if the amount is positive.
func (m Money) IsPositive() bool {
	return m.amount > 0
}

// IsNegative returns true if the amount is negative.
func (m Money) IsNegative() bool {
	return m.amount < 0
}

// Negate returns a new Money with the negated amount.
func (m Money) Negate() Money {
	return Money{
		amount:   -m.amount,
		currency: m.currency,
	}
}

// Abs returns a new Money with the absolute value of the amount.
func (m Money) Abs() Money {
	if m.amount < 0 {
		return m.Negate()
	}
	return m
}

// MarshalJSON implements json.Marshaler.
func (m Money) MarshalJSON() ([]byte, error) {
	return json.Marshal(moneyJSON{
		Amount:   m.Amount(),
		Currency: m.currency,
	})
}

// UnmarshalJSON implements json.Unmarshaler.
func (m *Money) UnmarshalJSON(data []byte) error {
	var mj moneyJSON
	if err := json.Unmarshal(data, &mj); err != nil {
		// Try unmarshaling as a plain number
		var amount float64
		if err := json.Unmarshal(data, &amount); err != nil {
			return err
		}
		*m = NewMoney(amount, DefaultCurrency)
		return nil
	}
	*m = NewMoney(mj.Amount, mj.Currency)
	return nil
}

// Value implements driver.Valuer for database storage.
// Stores the amount as float64 for compatibility with existing GORM decimal columns.
func (m Money) Value() (driver.Value, error) {
	return m.Amount(), nil
}

// Scan implements sql.Scanner for database retrieval.
func (m *Money) Scan(value interface{}) error {
	if value == nil {
		*m = Zero(DefaultCurrency)
		return nil
	}

	switch v := value.(type) {
	case float64:
		*m = NewMoney(v, DefaultCurrency)
	case int64:
		*m = FromCents(v*100, DefaultCurrency)
	case []byte:
		var amount float64
		if _, err := fmt.Sscanf(string(v), "%f", &amount); err != nil {
			return err
		}
		*m = NewMoney(amount, DefaultCurrency)
	case string:
		var amount float64
		if _, err := fmt.Sscanf(v, "%f", &amount); err != nil {
			return err
		}
		*m = NewMoney(amount, DefaultCurrency)
	default:
		return fmt.Errorf("cannot scan %T into Money", value)
	}
	return nil
}

// Max returns the larger of m and other.
// Returns m if currencies don't match.
func (m Money) Max(other Money) Money {
	if m.currency != other.currency {
		return m
	}
	if other.amount > m.amount {
		return other
	}
	return m
}

// Min returns the smaller of m and other.
// Returns m if currencies don't match.
func (m Money) Min(other Money) Money {
	if m.currency != other.currency {
		return m
	}
	if other.amount < m.amount {
		return other
	}
	return m
}

// SumMoney returns the sum of all money values.
// All values must have the same currency as the first value.
// Returns zero if the slice is empty.
func SumMoney(values ...Money) (Money, error) {
	if len(values) == 0 {
		return Zero(DefaultCurrency), nil
	}

	result := values[0]
	for i := 1; i < len(values); i++ {
		sum, err := result.Add(values[i])
		if err != nil {
			return Money{}, err
		}
		result = sum
	}
	return result, nil
}
