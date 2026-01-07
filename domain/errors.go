package domain

import (
	"errors"
	"fmt"

	"github.com/google/uuid"
)

// Domain error codes - machine-readable identifiers
const (
	CodeNotFound               = "NOT_FOUND"
	CodeConflict               = "CONFLICT"
	CodeValidation             = "VALIDATION_ERROR"
	CodeConcurrentModification = "CONCURRENT_MODIFICATION"
	CodeInsufficientStock      = "INSUFFICIENT_STOCK"
	CodeInvalidTransition      = "INVALID_STATE_TRANSITION"
	CodeUnauthorized           = "UNAUTHORIZED"
	CodeForbidden              = "FORBIDDEN"
	CodeInternalError          = "INTERNAL_ERROR"
)

// DomainError represents a domain-level error with semantic meaning.
// These errors map cleanly to HTTP status codes and provide
// machine-readable codes for API consumers.
type DomainError struct {
	Code    string      // Machine-readable error code
	Message string      // Human-readable message
	Details interface{} // Optional additional details (validation errors, etc.)
	Err     error       // Wrapped underlying error
}

// Error implements the error interface.
func (e *DomainError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %s: %v", e.Code, e.Message, e.Err)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// Unwrap returns the wrapped error for errors.Is/As support.
func (e *DomainError) Unwrap() error {
	return e.Err
}

// WithDetails returns a new DomainError with additional details.
func (e *DomainError) WithDetails(details interface{}) *DomainError {
	return &DomainError{
		Code:    e.Code,
		Message: e.Message,
		Details: details,
		Err:     e.Err,
	}
}

// WithError returns a new DomainError wrapping the given error.
func (e *DomainError) WithError(err error) *DomainError {
	return &DomainError{
		Code:    e.Code,
		Message: e.Message,
		Details: e.Details,
		Err:     err,
	}
}

// Sentinel errors for type checking with errors.Is
var (
	ErrNotFound               = &DomainError{Code: CodeNotFound, Message: "resource not found"}
	ErrConflict               = &DomainError{Code: CodeConflict, Message: "conflict with existing data"}
	ErrValidation             = &DomainError{Code: CodeValidation, Message: "validation failed"}
	ErrConcurrentModification = &DomainError{Code: CodeConcurrentModification, Message: "concurrent modification detected"}
	ErrInsufficientStock      = &DomainError{Code: CodeInsufficientStock, Message: "insufficient stock"}
	ErrInvalidTransition      = &DomainError{Code: CodeInvalidTransition, Message: "invalid state transition"}
	ErrUnauthorized           = &DomainError{Code: CodeUnauthorized, Message: "unauthorized access"}
	ErrForbidden              = &DomainError{Code: CodeForbidden, Message: "permission denied"}
	ErrInternal               = &DomainError{Code: CodeInternalError, Message: "internal error"}
)

// Is implements errors.Is for DomainError.
func (e *DomainError) Is(target error) bool {
	t, ok := target.(*DomainError)
	if !ok {
		return false
	}
	return e.Code == t.Code
}

// ----- Constructor Functions -----

// NewNotFoundError creates a not found error for a specific entity.
func NewNotFoundError(entity string, id interface{}) *DomainError {
	return &DomainError{
		Code:    CodeNotFound,
		Message: fmt.Sprintf("%s with ID '%v' not found", entity, id),
	}
}

// NewConflictError creates a conflict error.
func NewConflictError(message string) *DomainError {
	return &DomainError{
		Code:    CodeConflict,
		Message: message,
	}
}

// NewDuplicateError creates a conflict error for duplicate entries.
func NewDuplicateError(entity, field, value string) *DomainError {
	return &DomainError{
		Code:    CodeConflict,
		Message: fmt.Sprintf("%s with %s '%s' already exists", entity, field, value),
	}
}

// NewValidationError creates a validation error.
func NewValidationError(message string, details interface{}) *DomainError {
	return &DomainError{
		Code:    CodeValidation,
		Message: message,
		Details: details,
	}
}

// NewFieldValidationError creates a validation error for a specific field.
func NewFieldValidationError(field, message string) *DomainError {
	return &DomainError{
		Code:    CodeValidation,
		Message: fmt.Sprintf("validation failed for field '%s': %s", field, message),
		Details: map[string]string{field: message},
	}
}

// NewConcurrentModificationError creates a concurrent modification error.
func NewConcurrentModificationError(entity string, id interface{}, expectedVersion, actualVersion int64) *DomainError {
	return &DomainError{
		Code:    CodeConcurrentModification,
		Message: fmt.Sprintf("%s with ID '%v' was modified by another process (expected version %d, found %d)", entity, id, expectedVersion, actualVersion),
		Details: map[string]interface{}{
			"expected_version": expectedVersion,
			"actual_version":   actualVersion,
		},
	}
}

// NewInsufficientStockError creates an insufficient stock error.
func NewInsufficientStockError(productID uuid.UUID, requested, available int) *DomainError {
	return &DomainError{
		Code:    CodeInsufficientStock,
		Message: fmt.Sprintf("insufficient stock for product '%s': requested %d, available %d", productID, requested, available),
		Details: map[string]interface{}{
			"product_id": productID,
			"requested":  requested,
			"available":  available,
		},
	}
}

// NewInvalidTransitionError creates an invalid state transition error.
func NewInvalidTransitionError(from, to string) *DomainError {
	return &DomainError{
		Code:    CodeInvalidTransition,
		Message: fmt.Sprintf("invalid state transition from '%s' to '%s'", from, to),
		Details: map[string]string{
			"from": from,
			"to":   to,
		},
	}
}

// NewUnauthorizedError creates an unauthorized error.
func NewUnauthorizedError(message string) *DomainError {
	if message == "" {
		message = "authentication required"
	}
	return &DomainError{
		Code:    CodeUnauthorized,
		Message: message,
	}
}

// NewForbiddenError creates a forbidden error.
func NewForbiddenError(message string) *DomainError {
	if message == "" {
		message = "you do not have permission to perform this action"
	}
	return &DomainError{
		Code:    CodeForbidden,
		Message: message,
	}
}

// NewInternalError creates an internal error, wrapping the original error.
func NewInternalError(message string, err error) *DomainError {
	return &DomainError{
		Code:    CodeInternalError,
		Message: message,
		Err:     err,
	}
}

// ----- Type Checking Functions -----

// IsNotFoundError returns true if the error is a not found error.
func IsNotFoundError(err error) bool {
	return errors.Is(err, ErrNotFound)
}

// IsConflictError returns true if the error is a conflict error.
func IsConflictError(err error) bool {
	return errors.Is(err, ErrConflict)
}

// IsValidationError returns true if the error is a validation error.
func IsValidationError(err error) bool {
	return errors.Is(err, ErrValidation)
}

// IsConcurrentModificationError returns true if the error is a concurrent modification error.
func IsConcurrentModificationError(err error) bool {
	return errors.Is(err, ErrConcurrentModification)
}

// IsInsufficientStockError returns true if the error is an insufficient stock error.
func IsInsufficientStockError(err error) bool {
	return errors.Is(err, ErrInsufficientStock)
}

// IsInvalidTransitionError returns true if the error is an invalid transition error.
func IsInvalidTransitionError(err error) bool {
	return errors.Is(err, ErrInvalidTransition)
}

// IsUnauthorizedError returns true if the error is an unauthorized error.
func IsUnauthorizedError(err error) bool {
	return errors.Is(err, ErrUnauthorized)
}

// IsForbiddenError returns true if the error is a forbidden error.
func IsForbiddenError(err error) bool {
	return errors.Is(err, ErrForbidden)
}

// IsInternalError returns true if the error is an internal error.
func IsInternalError(err error) bool {
	return errors.Is(err, ErrInternal)
}

// ----- Utility Functions -----

// GetDomainError extracts a DomainError from an error chain.
// Returns nil if no DomainError is found.
func GetDomainError(err error) *DomainError {
	var domainErr *DomainError
	if errors.As(err, &domainErr) {
		return domainErr
	}
	return nil
}

// GetErrorCode returns the error code from a DomainError, or empty string if not a DomainError.
func GetErrorCode(err error) string {
	domainErr := GetDomainError(err)
	if domainErr != nil {
		return domainErr.Code
	}
	return ""
}

// GetHTTPStatus returns the appropriate HTTP status code for a domain error.
func GetHTTPStatus(err error) int {
	code := GetErrorCode(err)
	switch code {
	case CodeNotFound:
		return 404
	case CodeConflict, CodeConcurrentModification:
		return 409
	case CodeValidation, CodeInsufficientStock, CodeInvalidTransition:
		return 422
	case CodeUnauthorized:
		return 401
	case CodeForbidden:
		return 403
	default:
		return 500
	}
}

// WrapError wraps an error with domain context.
// If err is already a DomainError, it returns a new DomainError with the original error wrapped.
// Otherwise, it creates an internal error wrapping the original.
func WrapError(message string, err error) *DomainError {
	if err == nil {
		return nil
	}
	domainErr := GetDomainError(err)
	if domainErr != nil {
		return &DomainError{
			Code:    domainErr.Code,
			Message: message,
			Details: domainErr.Details,
			Err:     err,
		}
	}
	return NewInternalError(message, err)
}
