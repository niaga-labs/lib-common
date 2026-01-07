package domain

import (
	"errors"
	"testing"

	"github.com/google/uuid"
)

func TestDomainErrorError(t *testing.T) {
	err := &DomainError{
		Code:    CodeNotFound,
		Message: "order not found",
	}

	expected := "NOT_FOUND: order not found"
	if err.Error() != expected {
		t.Errorf("Error() = %v, want %v", err.Error(), expected)
	}

	// With wrapped error
	wrapped := &DomainError{
		Code:    CodeNotFound,
		Message: "order not found",
		Err:     errors.New("db error"),
	}
	if wrapped.Error() != "NOT_FOUND: order not found: db error" {
		t.Errorf("Error() with wrapped = %v", wrapped.Error())
	}
}

func TestDomainErrorIs(t *testing.T) {
	err := NewNotFoundError("Order", "123")

	if !errors.Is(err, ErrNotFound) {
		t.Error("errors.Is() should match ErrNotFound")
	}

	if errors.Is(err, ErrConflict) {
		t.Error("errors.Is() should not match ErrConflict")
	}
}

func TestNewNotFoundError(t *testing.T) {
	err := NewNotFoundError("Product", uuid.New())

	if err.Code != CodeNotFound {
		t.Errorf("Code = %v, want %v", err.Code, CodeNotFound)
	}

	if !IsNotFoundError(err) {
		t.Error("IsNotFoundError() should return true")
	}
}

func TestNewConflictError(t *testing.T) {
	err := NewConflictError("email already exists")

	if err.Code != CodeConflict {
		t.Errorf("Code = %v, want %v", err.Code, CodeConflict)
	}

	if !IsConflictError(err) {
		t.Error("IsConflictError() should return true")
	}
}

func TestNewDuplicateError(t *testing.T) {
	err := NewDuplicateError("Customer", "email", "test@example.com")

	if err.Code != CodeConflict {
		t.Errorf("Code = %v, want %v", err.Code, CodeConflict)
	}

	expected := "Customer with email 'test@example.com' already exists"
	if err.Message != expected {
		t.Errorf("Message = %v, want %v", err.Message, expected)
	}
}

func TestNewValidationError(t *testing.T) {
	details := map[string]string{
		"email": "invalid email format",
		"phone": "invalid phone number",
	}
	err := NewValidationError("validation failed", details)

	if err.Code != CodeValidation {
		t.Errorf("Code = %v, want %v", err.Code, CodeValidation)
	}

	if !IsValidationError(err) {
		t.Error("IsValidationError() should return true")
	}
}

func TestNewInsufficientStockError(t *testing.T) {
	productID := uuid.New()
	err := NewInsufficientStockError(productID, 10, 5)

	if err.Code != CodeInsufficientStock {
		t.Errorf("Code = %v, want %v", err.Code, CodeInsufficientStock)
	}

	if !IsInsufficientStockError(err) {
		t.Error("IsInsufficientStockError() should return true")
	}

	details := err.Details.(map[string]interface{})
	if details["requested"] != 10 {
		t.Errorf("Details.requested = %v, want 10", details["requested"])
	}
	if details["available"] != 5 {
		t.Errorf("Details.available = %v, want 5", details["available"])
	}
}

func TestNewInvalidTransitionError(t *testing.T) {
	err := NewInvalidTransitionError("pending", "shipped")

	if err.Code != CodeInvalidTransition {
		t.Errorf("Code = %v, want %v", err.Code, CodeInvalidTransition)
	}

	if !IsInvalidTransitionError(err) {
		t.Error("IsInvalidTransitionError() should return true")
	}
}

func TestNewConcurrentModificationError(t *testing.T) {
	err := NewConcurrentModificationError("Order", "123", 1, 2)

	if err.Code != CodeConcurrentModification {
		t.Errorf("Code = %v, want %v", err.Code, CodeConcurrentModification)
	}

	if !IsConcurrentModificationError(err) {
		t.Error("IsConcurrentModificationError() should return true")
	}
}

func TestGetHTTPStatus(t *testing.T) {
	tests := []struct {
		err    error
		status int
	}{
		{NewNotFoundError("Order", "123"), 404},
		{NewConflictError("duplicate"), 409},
		{NewValidationError("invalid", nil), 422},
		{NewInsufficientStockError(uuid.New(), 1, 0), 422},
		{NewInvalidTransitionError("a", "b"), 422},
		{NewUnauthorizedError(""), 401},
		{NewForbiddenError(""), 403},
		{NewInternalError("error", nil), 500},
		{errors.New("unknown error"), 500},
	}

	for _, tt := range tests {
		t.Run(tt.err.Error(), func(t *testing.T) {
			status := GetHTTPStatus(tt.err)
			if status != tt.status {
				t.Errorf("GetHTTPStatus() = %v, want %v", status, tt.status)
			}
		})
	}
}

func TestGetDomainError(t *testing.T) {
	domainErr := NewNotFoundError("Order", "123")
	extracted := GetDomainError(domainErr)
	if extracted == nil {
		t.Error("GetDomainError() should extract DomainError")
	}

	regularErr := errors.New("regular error")
	extracted = GetDomainError(regularErr)
	if extracted != nil {
		t.Error("GetDomainError() should return nil for non-DomainError")
	}
}

func TestWrapError(t *testing.T) {
	// Wrap domain error
	original := NewNotFoundError("Order", "123")
	wrapped := WrapError("failed to get order", original)

	if wrapped.Code != CodeNotFound {
		t.Errorf("Code = %v, want %v", wrapped.Code, CodeNotFound)
	}

	// Wrap regular error
	regularErr := errors.New("db connection failed")
	wrapped = WrapError("database error", regularErr)

	if wrapped.Code != CodeInternalError {
		t.Errorf("Code = %v, want %v", wrapped.Code, CodeInternalError)
	}

	// Wrap nil
	result := WrapError("no error", nil)
	if result != nil {
		t.Error("WrapError(nil) should return nil")
	}
}

func TestDomainErrorWithDetails(t *testing.T) {
	err := NewValidationError("validation failed", nil)
	details := map[string]string{"field": "error"}

	errWithDetails := err.WithDetails(details)

	if errWithDetails.Details == nil {
		t.Error("WithDetails() should set details")
	}
}

func TestDomainErrorWithError(t *testing.T) {
	err := NewNotFoundError("Order", "123")
	wrapped := errors.New("underlying error")

	errWithWrapped := err.WithError(wrapped)

	if errWithWrapped.Err == nil {
		t.Error("WithError() should set wrapped error")
	}

	if !errors.Is(errWithWrapped.Err, wrapped) {
		t.Error("Unwrap() should return wrapped error")
	}
}
