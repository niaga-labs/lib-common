package middleware

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"
)

// RateLimiter stores limiters for each client
type RateLimiter struct {
	limiters map[string]*rate.Limiter
	mu       sync.RWMutex
	rate     rate.Limit
	burst    int
	done     chan struct{} // Channel for graceful shutdown
}

// NewRateLimiter creates a new rate limiter
func NewRateLimiter(requestsPerSecond int, burst int) *RateLimiter {
	return &RateLimiter{
		limiters: make(map[string]*rate.Limiter),
		rate:     rate.Limit(requestsPerSecond),
		burst:    burst,
		done:     make(chan struct{}),
	}
}

// getLimiter returns a limiter for the given key (IP address)
func (rl *RateLimiter) getLimiter(key string) *rate.Limiter {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	limiter, exists := rl.limiters[key]
	if !exists {
		limiter = rate.NewLimiter(rl.rate, rl.burst)
		rl.limiters[key] = limiter
	}

	return limiter
}

// RateLimit middleware
func (rl *RateLimiter) RateLimit() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get client IP
		ip := c.ClientIP()

		// Get limiter for this IP
		limiter := rl.getLimiter(ip)

		// Check if request is allowed
		if !limiter.Allow() {
			c.JSON(http.StatusTooManyRequests, gin.H{
				"error":   "Rate limit exceeded",
				"message": "Too many requests. Please try again later.",
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

// CleanupLimiters removes old limiters periodically
// Call this once to start the cleanup goroutine
func (rl *RateLimiter) CleanupLimiters() {
	ticker := time.NewTicker(1 * time.Hour)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				rl.mu.Lock()
				// Clear all limiters (they'll be recreated as needed)
				rl.limiters = make(map[string]*rate.Limiter)
				rl.mu.Unlock()
			case <-rl.done:
				return // Graceful exit
			}
		}
	}()
}

// Shutdown gracefully stops the cleanup goroutine
func (rl *RateLimiter) Shutdown() {
	close(rl.done)
}
