package resilience

import (
	"context"
	"math"
	"math/rand"
	"time"

	"go.uber.org/zap"
)

// RetryConfig holds configuration for retry
type RetryConfig struct {
	MaxAttempts     int           // Maximum number of attempts (default: 3)
	InitialDelay    time.Duration // Initial delay before first retry (default: 100ms)
	MaxDelay        time.Duration // Maximum delay between retries (default: 10s)
	Multiplier      float64       // Multiplier for exponential backoff (default: 2.0)
	JitterFactor    float64       // Jitter factor 0-1 (default: 0.1 = 10%)
	RetryableErrors []error       // Errors that should trigger retry (nil = all errors)
}

// DefaultRetryConfig returns default configuration
func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxAttempts:  3,
		InitialDelay: 100 * time.Millisecond,
		MaxDelay:     10 * time.Second,
		Multiplier:   2.0,
		JitterFactor: 0.1,
	}
}

// Retry implements retry with exponential backoff and jitter
type Retry struct {
	config RetryConfig
	logger *zap.Logger
}

// NewRetry creates a new retry handler
func NewRetry(config RetryConfig, logger *zap.Logger) *Retry {
	if config.MaxAttempts <= 0 {
		config.MaxAttempts = 3
	}
	if config.InitialDelay <= 0 {
		config.InitialDelay = 100 * time.Millisecond
	}
	if config.MaxDelay <= 0 {
		config.MaxDelay = 10 * time.Second
	}
	if config.Multiplier <= 0 {
		config.Multiplier = 2.0
	}
	if config.JitterFactor < 0 || config.JitterFactor > 1 {
		config.JitterFactor = 0.1
	}

	return &Retry{
		config: config,
		logger: logger,
	}
}

// Execute runs the function with retry logic
func (r *Retry) Execute(fn func() error) error {
	return r.ExecuteWithContext(context.Background(), func(ctx context.Context) error {
		return fn()
	})
}

// ExecuteWithContext runs the function with context and retry logic
func (r *Retry) ExecuteWithContext(ctx context.Context, fn func(context.Context) error) error {
	var lastErr error

	for attempt := 1; attempt <= r.config.MaxAttempts; attempt++ {
		// Check context before attempt
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// Execute the function
		err := fn(ctx)
		if err == nil {
			return nil
		}

		lastErr = err

		// Check if error is retryable
		if !r.isRetryable(err) {
			return err
		}

		// Don't sleep after last attempt
		if attempt == r.config.MaxAttempts {
			break
		}

		// Calculate delay with exponential backoff and jitter
		delay := r.calculateDelay(attempt)

		r.logger.Debug("Retrying after error",
			zap.Int("attempt", attempt),
			zap.Int("max_attempts", r.config.MaxAttempts),
			zap.Duration("delay", delay),
			zap.Error(err))

		// Wait before retry
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(delay):
		}
	}

	return lastErr
}

// isRetryable checks if the error should trigger a retry
func (r *Retry) isRetryable(err error) bool {
	// If no specific errors defined, retry all errors
	if len(r.config.RetryableErrors) == 0 {
		return true
	}

	for _, retryableErr := range r.config.RetryableErrors {
		if err == retryableErr {
			return true
		}
	}
	return false
}

// calculateDelay calculates the delay with exponential backoff and jitter
func (r *Retry) calculateDelay(attempt int) time.Duration {
	// Exponential backoff: initialDelay * multiplier^(attempt-1)
	delay := float64(r.config.InitialDelay) * math.Pow(r.config.Multiplier, float64(attempt-1))

	// Apply jitter: delay * (1 + random(-jitter, +jitter))
	jitter := (rand.Float64()*2 - 1) * r.config.JitterFactor
	delay = delay * (1 + jitter)

	// Cap at max delay
	if delay > float64(r.config.MaxDelay) {
		delay = float64(r.config.MaxDelay)
	}

	return time.Duration(delay)
}

// RetryWithBackoff is a helper function for simple retry scenarios
func RetryWithBackoff(ctx context.Context, maxAttempts int, fn func() error, logger *zap.Logger) error {
	config := DefaultRetryConfig()
	config.MaxAttempts = maxAttempts
	retry := NewRetry(config, logger)
	return retry.ExecuteWithContext(ctx, func(ctx context.Context) error {
		return fn()
	})
}
