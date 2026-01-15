package resilience

import (
	"context"
	"errors"
	"sync"
	"time"

	"go.uber.org/zap"
)

var (
	// ErrBulkheadFull is returned when the bulkhead is at capacity
	ErrBulkheadFull = errors.New("bulkhead is full, request rejected")

	// ErrBulkheadTimeout is returned when waiting for bulkhead times out
	ErrBulkheadTimeout = errors.New("bulkhead acquire timeout")
)

// BulkheadConfig holds configuration for bulkhead
type BulkheadConfig struct {
	Name            string        // Name of the bulkhead (for logging/metrics)
	MaxConcurrent   int           // Maximum concurrent executions (default: 10)
	MaxWaitDuration time.Duration // Max time to wait for a slot (default: 0 = no wait)
}

// DefaultBulkheadConfig returns default configuration
func DefaultBulkheadConfig(name string) BulkheadConfig {
	return BulkheadConfig{
		Name:            name,
		MaxConcurrent:   10,
		MaxWaitDuration: 0, // No waiting by default (fail fast)
	}
}

// Bulkhead implements the bulkhead pattern for resource isolation
// It limits the number of concurrent calls to a resource, preventing
// cascading failures when a dependency is slow or unresponsive
type Bulkhead struct {
	config BulkheadConfig
	sem    chan struct{}
	logger *zap.Logger

	mu          sync.RWMutex
	active      int
	rejected    int64
	totalCalls  int64
	waitingTime time.Duration
}

// NewBulkhead creates a new bulkhead
func NewBulkhead(config BulkheadConfig, logger *zap.Logger) *Bulkhead {
	if config.MaxConcurrent <= 0 {
		config.MaxConcurrent = 10
	}

	return &Bulkhead{
		config: config,
		sem:    make(chan struct{}, config.MaxConcurrent),
		logger: logger,
	}
}

// Execute runs the given function with bulkhead protection
func (b *Bulkhead) Execute(fn func() error) error {
	return b.ExecuteWithContext(context.Background(), func(ctx context.Context) error {
		return fn()
	})
}

// ExecuteWithContext runs the given function with context and bulkhead protection
func (b *Bulkhead) ExecuteWithContext(ctx context.Context, fn func(context.Context) error) error {
	// Try to acquire a slot
	if err := b.acquire(ctx); err != nil {
		return err
	}
	defer b.release()

	// Execute the function
	return fn(ctx)
}

// acquire tries to acquire a slot in the bulkhead
func (b *Bulkhead) acquire(ctx context.Context) error {
	b.mu.Lock()
	b.totalCalls++
	b.mu.Unlock()

	startWait := time.Now()

	// No waiting - fail fast
	if b.config.MaxWaitDuration == 0 {
		select {
		case b.sem <- struct{}{}:
			b.mu.Lock()
			b.active++
			b.mu.Unlock()
			return nil
		default:
			b.mu.Lock()
			b.rejected++
			b.mu.Unlock()
			b.logger.Warn("Bulkhead full, request rejected",
				zap.String("name", b.config.Name),
				zap.Int("max_concurrent", b.config.MaxConcurrent))
			return ErrBulkheadFull
		}
	}

	// With waiting - use timeout
	timer := time.NewTimer(b.config.MaxWaitDuration)
	defer timer.Stop()

	select {
	case b.sem <- struct{}{}:
		b.mu.Lock()
		b.active++
		b.waitingTime += time.Since(startWait)
		b.mu.Unlock()
		return nil
	case <-timer.C:
		b.mu.Lock()
		b.rejected++
		b.mu.Unlock()
		b.logger.Warn("Bulkhead acquire timeout",
			zap.String("name", b.config.Name),
			zap.Duration("wait_duration", b.config.MaxWaitDuration))
		return ErrBulkheadTimeout
	case <-ctx.Done():
		return ctx.Err()
	}
}

// release releases a slot in the bulkhead
func (b *Bulkhead) release() {
	<-b.sem
	b.mu.Lock()
	b.active--
	b.mu.Unlock()
}

// ActiveCount returns the number of currently active executions
func (b *Bulkhead) ActiveCount() int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.active
}

// AvailableSlots returns the number of available slots
func (b *Bulkhead) AvailableSlots() int {
	return b.config.MaxConcurrent - b.ActiveCount()
}

// RejectedCount returns the total number of rejected requests
func (b *Bulkhead) RejectedCount() int64 {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.rejected
}

// TotalCalls returns the total number of calls
func (b *Bulkhead) TotalCalls() int64 {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.totalCalls
}

// Stats returns bulkhead statistics
func (b *Bulkhead) Stats() BulkheadStats {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return BulkheadStats{
		Name:          b.config.Name,
		MaxConcurrent: b.config.MaxConcurrent,
		Active:        b.active,
		Available:     b.config.MaxConcurrent - b.active,
		TotalCalls:    b.totalCalls,
		Rejected:      b.rejected,
	}
}

// BulkheadStats holds statistics for a bulkhead
type BulkheadStats struct {
	Name          string `json:"name"`
	MaxConcurrent int    `json:"max_concurrent"`
	Active        int    `json:"active"`
	Available     int    `json:"available"`
	TotalCalls    int64  `json:"total_calls"`
	Rejected      int64  `json:"rejected"`
}

// BulkheadGroup manages multiple bulkheads for different resources
type BulkheadGroup struct {
	bulkheads map[string]*Bulkhead
	mu        sync.RWMutex
	logger    *zap.Logger
}

// NewBulkheadGroup creates a new bulkhead group
func NewBulkheadGroup(logger *zap.Logger) *BulkheadGroup {
	return &BulkheadGroup{
		bulkheads: make(map[string]*Bulkhead),
		logger:    logger,
	}
}

// Get returns the bulkhead for the given name, creating it if needed
func (bg *BulkheadGroup) Get(name string, maxConcurrent int) *Bulkhead {
	bg.mu.RLock()
	if bh, ok := bg.bulkheads[name]; ok {
		bg.mu.RUnlock()
		return bh
	}
	bg.mu.RUnlock()

	// Create new bulkhead
	bg.mu.Lock()
	defer bg.mu.Unlock()

	// Double-check after acquiring write lock
	if bh, ok := bg.bulkheads[name]; ok {
		return bh
	}

	config := BulkheadConfig{
		Name:            name,
		MaxConcurrent:   maxConcurrent,
		MaxWaitDuration: 0,
	}
	bh := NewBulkhead(config, bg.logger)
	bg.bulkheads[name] = bh

	bg.logger.Info("Created new bulkhead",
		zap.String("name", name),
		zap.Int("max_concurrent", maxConcurrent))

	return bh
}

// AllStats returns statistics for all bulkheads
func (bg *BulkheadGroup) AllStats() []BulkheadStats {
	bg.mu.RLock()
	defer bg.mu.RUnlock()

	stats := make([]BulkheadStats, 0, len(bg.bulkheads))
	for _, bh := range bg.bulkheads {
		stats = append(stats, bh.Stats())
	}
	return stats
}
