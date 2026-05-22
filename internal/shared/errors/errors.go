// Package errors defines typed domain errors shared across all packages.
package errors

import "errors"

// Sentinel errors used throughout the application.
// Controllers map these to HTTP status codes via httpx.WriteError.
var (
	ErrValidation  = errors.New("validation error")
	ErrNotFound    = errors.New("not found")
	ErrSKUConflict = errors.New("sku conflict")
	ErrRateLimited = errors.New("rate limited")
	ErrInvalidJSON = errors.New("invalid json")
)

// ValidationError wraps ErrValidation with a human-readable message.
type ValidationError struct {
	Message string
}

func (e *ValidationError) Error() string { return e.Message }
func (e *ValidationError) Unwrap() error { return ErrValidation }

// NewValidation returns a *ValidationError that unwraps to ErrValidation.
func NewValidation(msg string) error {
	return &ValidationError{Message: msg}
}

// RateLimitedError carries retry metadata alongside the sentinel.
type RateLimitedError struct {
	Message           string
	RetryAfterSeconds int
}

func (e *RateLimitedError) Error() string { return e.Message }
func (e *RateLimitedError) Unwrap() error { return ErrRateLimited }

// NewRateLimited returns a *RateLimitedError that unwraps to ErrRateLimited.
func NewRateLimited(retryAfter int) error {
	return &RateLimitedError{
		Message:           "user has exceeded 5 requests per minute",
		RetryAfterSeconds: retryAfter,
	}
}
