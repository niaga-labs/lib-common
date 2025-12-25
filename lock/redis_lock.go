package lock

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

var (
	// ErrLockNotAcquired is returned when the lock cannot be acquired
	ErrLockNotAcquired = errors.New("lock not acquired")
	// ErrLockNotHeld is returned when trying to release a lock not held
	ErrLockNotHeld = errors.New("lock not held by this client")
)

// RedisLock represents a distributed lock using Redis
type RedisLock struct {
	client *redis.Client
	key    string
	value  string
	ttl    time.Duration
}

// NewRedisLock creates a new Redis distributed lock
func NewRedisLock(client *redis.Client, key string, ttl time.Duration) *RedisLock {
	return &RedisLock{
		client: client,
		key:    "lock:" + key,
		value:  uuid.New().String(),
		ttl:    ttl,
	}
}

// Acquire attempts to acquire the lock with retry
func (l *RedisLock) Acquire(ctx context.Context) error {
	return l.AcquireWithRetry(ctx, 10, 50*time.Millisecond)
}

// AcquireWithRetry attempts to acquire the lock with custom retry settings
func (l *RedisLock) AcquireWithRetry(ctx context.Context, maxRetries int, retryDelay time.Duration) error {
	for i := 0; i < maxRetries; i++ {
		ok, err := l.client.SetNX(ctx, l.key, l.value, l.ttl).Result()
		if err != nil {
			return fmt.Errorf("redis error: %w", err)
		}
		if ok {
			return nil // Lock acquired
		}

		// Wait before retry
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(retryDelay):
		}
	}
	return ErrLockNotAcquired
}

// TryAcquire attempts to acquire the lock once without retrying
func (l *RedisLock) TryAcquire(ctx context.Context) (bool, error) {
	ok, err := l.client.SetNX(ctx, l.key, l.value, l.ttl).Result()
	if err != nil {
		return false, fmt.Errorf("redis error: %w", err)
	}
	return ok, nil
}

// Release releases the lock (only if we own it)
func (l *RedisLock) Release(ctx context.Context) error {
	// Lua script to release lock only if we own it
	script := `
		if redis.call("get", KEYS[1]) == ARGV[1] then
			return redis.call("del", KEYS[1])
		else
			return 0
		end
	`
	result, err := l.client.Eval(ctx, script, []string{l.key}, l.value).Int64()
	if err != nil {
		return fmt.Errorf("redis error: %w", err)
	}
	if result == 0 {
		return ErrLockNotHeld
	}
	return nil
}

// Extend extends the lock TTL (only if we own it)
func (l *RedisLock) Extend(ctx context.Context, extension time.Duration) error {
	script := `
		if redis.call("get", KEYS[1]) == ARGV[1] then
			return redis.call("pexpire", KEYS[1], ARGV[2])
		else
			return 0
		end
	`
	result, err := l.client.Eval(ctx, script, []string{l.key}, l.value, extension.Milliseconds()).Int64()
	if err != nil {
		return fmt.Errorf("redis error: %w", err)
	}
	if result == 0 {
		return ErrLockNotHeld
	}
	return nil
}

// WithLock executes a function while holding the lock
// The lock is automatically released after the function completes
func WithLock(ctx context.Context, client *redis.Client, key string, ttl time.Duration, fn func() error) error {
	lock := NewRedisLock(client, key, ttl)
	if err := lock.Acquire(ctx); err != nil {
		return err
	}
	defer lock.Release(ctx)
	return fn()
}

// WithLockResult executes a function while holding the lock and returns a result
func WithLockResult[T any](ctx context.Context, client *redis.Client, key string, ttl time.Duration, fn func() (T, error)) (T, error) {
	var zero T
	lock := NewRedisLock(client, key, ttl)
	if err := lock.Acquire(ctx); err != nil {
		return zero, err
	}
	defer lock.Release(ctx)
	return fn()
}

// LockManager provides a higher-level interface for managing distributed locks
type LockManager struct {
	client     *redis.Client
	defaultTTL time.Duration
}

// NewLockManager creates a new lock manager
func NewLockManager(client *redis.Client, defaultTTL time.Duration) *LockManager {
	return &LockManager{
		client:     client,
		defaultTTL: defaultTTL,
	}
}

// Lock acquires a lock with the default TTL
func (m *LockManager) Lock(ctx context.Context, key string) (*RedisLock, error) {
	return m.LockWithTTL(ctx, key, m.defaultTTL)
}

// LockWithTTL acquires a lock with a custom TTL
func (m *LockManager) LockWithTTL(ctx context.Context, key string, ttl time.Duration) (*RedisLock, error) {
	lock := NewRedisLock(m.client, key, ttl)
	if err := lock.Acquire(ctx); err != nil {
		return nil, err
	}
	return lock, nil
}

// TryLock attempts to acquire a lock without blocking
func (m *LockManager) TryLock(ctx context.Context, key string) (*RedisLock, bool, error) {
	lock := NewRedisLock(m.client, key, m.defaultTTL)
	acquired, err := lock.TryAcquire(ctx)
	if err != nil {
		return nil, false, err
	}
	if !acquired {
		return nil, false, nil
	}
	return lock, true, nil
}

// WithLock is a convenience method to execute a function with a lock
func (m *LockManager) WithLock(ctx context.Context, key string, fn func() error) error {
	return WithLock(ctx, m.client, key, m.defaultTTL, fn)
}
