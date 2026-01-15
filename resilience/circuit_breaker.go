package resilience

import (
	"context"
	"errors"
	"sync"
	"time"

	"go.uber.org/zap"
)

// CircuitState represents the state of the circuit breaker
type CircuitState int

const (
	StateClosed   CircuitState = iota // Normal operation
	StateOpen                         // Circuit is open, requests fail fast
	StateHalfOpen                     // Testing if service recovered
)

var (
	// ErrCircuitOpen is returned when circuit is open
	ErrCircuitOpen = errors.New("circuit breaker is open")

	// ErrTooManyRequests is returned when too many requests in half-open state
	ErrTooManyRequests = errors.New("too many requests in half-open state")
)

// CircuitBreakerConfig holds configuration for circuit breaker
type CircuitBreakerConfig struct {
	Name         string        // Name of the circuit breaker (for logging)
	MaxFailures  int           // Max failures before opening circuit (default: 5)
	Timeout      time.Duration // How long to wait before trying again (default: 30s)
	ResetTimeout time.Duration // How long to wait in half-open before closing (default: 10s)
}

// DefaultCircuitBreakerConfig returns default configuration
func DefaultCircuitBreakerConfig(name string) CircuitBreakerConfig {
	return CircuitBreakerConfig{
		Name:         name,
		MaxFailures:  5,
		Timeout:      30 * time.Second,
		ResetTimeout: 10 * time.Second,
	}
}

// CircuitBreaker implements the circuit breaker pattern
type CircuitBreaker struct {
	config CircuitBreakerConfig

	mu           sync.RWMutex
	state        CircuitState
	failures     int
	successes    int // Successes needed to close from half-open
	lastFailTime time.Time
	lastAttempt  time.Time
	logger       *zap.Logger
}

// NewCircuitBreaker creates a new circuit breaker
func NewCircuitBreaker(config CircuitBreakerConfig, logger *zap.Logger) *CircuitBreaker {
	if config.MaxFailures <= 0 {
		config.MaxFailures = 5
	}
	if config.Timeout <= 0 {
		config.Timeout = 30 * time.Second
	}
	if config.ResetTimeout <= 0 {
		config.ResetTimeout = 10 * time.Second
	}

	return &CircuitBreaker{
		config: config,
		state:  StateClosed,
		logger: logger,
	}
}

// Execute runs the given function with circuit breaker protection
func (cb *CircuitBreaker) Execute(fn func() error) error {
	if err := cb.beforeRequest(); err != nil {
		return err
	}

	// Execute the function
	err := fn()

	// Handle the result
	cb.afterRequest(err)

	return err
}

// ExecuteWithContext runs the given function with context and circuit breaker protection
func (cb *CircuitBreaker) ExecuteWithContext(ctx context.Context, fn func(context.Context) error) error {
	if err := cb.beforeRequest(); err != nil {
		return err
	}

	// Execute the function
	err := fn(ctx)

	// Handle the result
	cb.afterRequest(err)

	return err
}

// beforeRequest checks if the request should be allowed
func (cb *CircuitBreaker) beforeRequest() error {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	now := time.Now()

	switch cb.state {
	case StateClosed:
		return nil

	case StateOpen:
		if now.Sub(cb.lastFailTime) > cb.config.Timeout {
			cb.state = StateHalfOpen
			cb.successes = 0
			cb.lastAttempt = now
			cb.logger.Info("Circuit breaker moving to half-open state",
				zap.String("name", cb.config.Name))
			return nil
		}
		return ErrCircuitOpen

	case StateHalfOpen:
		if now.Sub(cb.lastAttempt) < time.Second {
			return ErrTooManyRequests
		}
		cb.lastAttempt = now
		return nil

	default:
		return nil
	}
}

// afterRequest records the result of the request
func (cb *CircuitBreaker) afterRequest(err error) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	if err != nil {
		cb.onFailure()
	} else {
		cb.onSuccess()
	}
}

// onFailure handles a failed request
func (cb *CircuitBreaker) onFailure() {
	cb.failures++
	cb.lastFailTime = time.Now()

	switch cb.state {
	case StateClosed:
		if cb.failures >= cb.config.MaxFailures {
			cb.state = StateOpen
			cb.logger.Warn("Circuit breaker opened",
				zap.String("name", cb.config.Name),
				zap.Int("failures", cb.failures),
				zap.Int("max_failures", cb.config.MaxFailures))
		}

	case StateHalfOpen:
		cb.state = StateOpen
		cb.logger.Warn("Circuit breaker reopened after half-open failure",
			zap.String("name", cb.config.Name))
	}
}

// onSuccess handles a successful request
func (cb *CircuitBreaker) onSuccess() {
	switch cb.state {
	case StateClosed:
		cb.failures = 0

	case StateHalfOpen:
		cb.successes++
		// Need 3 consecutive successes to close
		if cb.successes >= 3 {
			cb.state = StateClosed
			cb.failures = 0
			cb.logger.Info("Circuit breaker closed after successful recovery",
				zap.String("name", cb.config.Name))
		}
	}
}

// State returns the current state
func (cb *CircuitBreaker) State() CircuitState {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.state
}

// StateName returns the string name of the current state
func (cb *CircuitBreaker) StateName() string {
	switch cb.State() {
	case StateClosed:
		return "closed"
	case StateOpen:
		return "open"
	case StateHalfOpen:
		return "half-open"
	default:
		return "unknown"
	}
}

// Failures returns the current failure count
func (cb *CircuitBreaker) Failures() int {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.failures
}

// Reset manually resets the circuit breaker
func (cb *CircuitBreaker) Reset() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.state = StateClosed
	cb.failures = 0
	cb.successes = 0
	cb.logger.Info("Circuit breaker manually reset",
		zap.String("name", cb.config.Name))
}

// IsOpen returns true if the circuit is open
func (cb *CircuitBreaker) IsOpen() bool {
	return cb.State() == StateOpen
}
