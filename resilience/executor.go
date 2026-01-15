package resilience

import (
	"context"
	"time"

	"go.uber.org/zap"
)

// ExecutorConfig holds configuration for resilient executor
type ExecutorConfig struct {
	Name string

	// Circuit Breaker settings
	CircuitBreakerEnabled bool
	MaxFailures           int
	CircuitTimeout        time.Duration

	// Bulkhead settings
	BulkheadEnabled bool
	MaxConcurrent   int
	MaxWaitDuration time.Duration

	// Retry settings
	RetryEnabled bool
	MaxAttempts  int
	InitialDelay time.Duration
	MaxDelay     time.Duration
}

// DefaultExecutorConfig returns default configuration
func DefaultExecutorConfig(name string) ExecutorConfig {
	return ExecutorConfig{
		Name:                  name,
		CircuitBreakerEnabled: true,
		MaxFailures:           5,
		CircuitTimeout:        30 * time.Second,
		BulkheadEnabled:       true,
		MaxConcurrent:         10,
		MaxWaitDuration:       0,
		RetryEnabled:          true,
		MaxAttempts:           3,
		InitialDelay:          100 * time.Millisecond,
		MaxDelay:              5 * time.Second,
	}
}

// Executor combines circuit breaker, bulkhead, and retry patterns
type Executor struct {
	config         ExecutorConfig
	circuitBreaker *CircuitBreaker
	bulkhead       *Bulkhead
	retry          *Retry
	logger         *zap.Logger
}

// NewExecutor creates a new resilient executor
func NewExecutor(config ExecutorConfig, logger *zap.Logger) *Executor {
	exec := &Executor{
		config: config,
		logger: logger,
	}

	// Initialize circuit breaker
	if config.CircuitBreakerEnabled {
		cbConfig := CircuitBreakerConfig{
			Name:         config.Name + "-cb",
			MaxFailures:  config.MaxFailures,
			Timeout:      config.CircuitTimeout,
			ResetTimeout: 10 * time.Second,
		}
		exec.circuitBreaker = NewCircuitBreaker(cbConfig, logger)
	}

	// Initialize bulkhead
	if config.BulkheadEnabled {
		bhConfig := BulkheadConfig{
			Name:            config.Name + "-bh",
			MaxConcurrent:   config.MaxConcurrent,
			MaxWaitDuration: config.MaxWaitDuration,
		}
		exec.bulkhead = NewBulkhead(bhConfig, logger)
	}

	// Initialize retry
	if config.RetryEnabled {
		retryConfig := RetryConfig{
			MaxAttempts:  config.MaxAttempts,
			InitialDelay: config.InitialDelay,
			MaxDelay:     config.MaxDelay,
			Multiplier:   2.0,
			JitterFactor: 0.1,
		}
		exec.retry = NewRetry(retryConfig, logger)
	}

	return exec
}

// Execute runs the function with all enabled resilience patterns
// Order: Bulkhead -> Circuit Breaker -> Retry -> Function
func (e *Executor) Execute(fn func() error) error {
	return e.ExecuteWithContext(context.Background(), func(ctx context.Context) error {
		return fn()
	})
}

// ExecuteWithContext runs the function with context and all enabled resilience patterns
func (e *Executor) ExecuteWithContext(ctx context.Context, fn func(context.Context) error) error {
	// Build the execution chain from inside out
	// The innermost is the actual function
	// Then wrapped by retry, circuit breaker, and bulkhead

	execution := fn

	// Wrap with retry (innermost after function)
	if e.retry != nil {
		innerFn := execution
		execution = func(ctx context.Context) error {
			return e.retry.ExecuteWithContext(ctx, innerFn)
		}
	}

	// Wrap with circuit breaker
	if e.circuitBreaker != nil {
		innerFn := execution
		execution = func(ctx context.Context) error {
			return e.circuitBreaker.ExecuteWithContext(ctx, innerFn)
		}
	}

	// Wrap with bulkhead (outermost)
	if e.bulkhead != nil {
		innerFn := execution
		execution = func(ctx context.Context) error {
			return e.bulkhead.ExecuteWithContext(ctx, innerFn)
		}
	}

	return execution(ctx)
}

// CircuitBreaker returns the underlying circuit breaker
func (e *Executor) CircuitBreaker() *CircuitBreaker {
	return e.circuitBreaker
}

// Bulkhead returns the underlying bulkhead
func (e *Executor) Bulkhead() *Bulkhead {
	return e.bulkhead
}

// Stats returns statistics for the executor
func (e *Executor) Stats() ExecutorStats {
	stats := ExecutorStats{
		Name: e.config.Name,
	}

	if e.circuitBreaker != nil {
		stats.CircuitBreakerState = e.circuitBreaker.StateName()
		stats.CircuitBreakerFailures = e.circuitBreaker.Failures()
	}

	if e.bulkhead != nil {
		bhStats := e.bulkhead.Stats()
		stats.BulkheadActive = bhStats.Active
		stats.BulkheadAvailable = bhStats.Available
		stats.BulkheadRejected = bhStats.Rejected
	}

	return stats
}

// ExecutorStats holds statistics for an executor
type ExecutorStats struct {
	Name                   string `json:"name"`
	CircuitBreakerState    string `json:"circuit_breaker_state,omitempty"`
	CircuitBreakerFailures int    `json:"circuit_breaker_failures,omitempty"`
	BulkheadActive         int    `json:"bulkhead_active,omitempty"`
	BulkheadAvailable      int    `json:"bulkhead_available,omitempty"`
	BulkheadRejected       int64  `json:"bulkhead_rejected,omitempty"`
}

// ExecutorGroup manages multiple executors for different resources
type ExecutorGroup struct {
	executors map[string]*Executor
	logger    *zap.Logger
}

// NewExecutorGroup creates a new executor group
func NewExecutorGroup(logger *zap.Logger) *ExecutorGroup {
	return &ExecutorGroup{
		executors: make(map[string]*Executor),
		logger:    logger,
	}
}

// Get returns the executor for the given name, creating with defaults if needed
func (eg *ExecutorGroup) Get(name string) *Executor {
	if exec, ok := eg.executors[name]; ok {
		return exec
	}

	config := DefaultExecutorConfig(name)
	exec := NewExecutor(config, eg.logger)
	eg.executors[name] = exec
	return exec
}

// GetOrCreate returns the executor for the given name, creating with config if needed
func (eg *ExecutorGroup) GetOrCreate(name string, config ExecutorConfig) *Executor {
	if exec, ok := eg.executors[name]; ok {
		return exec
	}

	exec := NewExecutor(config, eg.logger)
	eg.executors[name] = exec
	return exec
}

// AllStats returns statistics for all executors
func (eg *ExecutorGroup) AllStats() []ExecutorStats {
	stats := make([]ExecutorStats, 0, len(eg.executors))
	for _, exec := range eg.executors {
		stats = append(stats, exec.Stats())
	}
	return stats
}
