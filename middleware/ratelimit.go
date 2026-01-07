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

// EndpointRateLimiter provides endpoint-specific rate limiting
type EndpointRateLimiter struct {
	limiters map[string]*RateLimiter
	mu       sync.RWMutex
}

// NewEndpointRateLimiter creates an endpoint-specific rate limiter
func NewEndpointRateLimiter() *EndpointRateLimiter {
	return &EndpointRateLimiter{
		limiters: make(map[string]*RateLimiter),
	}
}

// Configure sets up rate limiting for a specific endpoint pattern
// requestsPerMinute is the number of requests allowed per minute
func (erl *EndpointRateLimiter) Configure(endpoint string, requestsPerMinute int, burst int) *EndpointRateLimiter {
	erl.mu.Lock()
	defer erl.mu.Unlock()

	// Convert requests per minute to requests per second
	rps := float64(requestsPerMinute) / 60.0
	if rps < 0.1 {
		rps = 0.1 // Minimum 6 requests per minute
	}

	limiter := &RateLimiter{
		limiters: make(map[string]*rate.Limiter),
		rate:     rate.Limit(rps),
		burst:    burst,
		done:     make(chan struct{}),
	}
	limiter.CleanupLimiters()

	erl.limiters[endpoint] = limiter
	return erl
}

// Limit returns a middleware that rate limits the endpoint
// Uses the configured rate for the endpoint, or applies default if not configured
func (erl *EndpointRateLimiter) Limit(endpoint string, defaultRPM int) gin.HandlerFunc {
	return func(c *gin.Context) {
		erl.mu.RLock()
		limiter, exists := erl.limiters[endpoint]
		erl.mu.RUnlock()

		if !exists {
			// Use default rate limiter for unconfigured endpoints
			erl.Configure(endpoint, defaultRPM, defaultRPM/10+1)
			erl.mu.RLock()
			limiter = erl.limiters[endpoint]
			erl.mu.RUnlock()
		}

		ip := c.ClientIP()
		ipLimiter := limiter.getLimiter(ip)

		if !ipLimiter.Allow() {
			c.JSON(http.StatusTooManyRequests, gin.H{
				"success": false,
				"error": gin.H{
					"code":    "RATE_LIMIT_EXCEEDED",
					"message": "Too many requests. Please slow down and try again.",
				},
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

// Shutdown gracefully stops all endpoint limiters
func (erl *EndpointRateLimiter) Shutdown() {
	erl.mu.Lock()
	defer erl.mu.Unlock()

	for _, limiter := range erl.limiters {
		limiter.Shutdown()
	}
}

// PaymentRateLimiter returns a pre-configured rate limiter for payment endpoints
// SECURITY: Strict rate limiting on payment endpoints prevents abuse
func PaymentRateLimiter() *EndpointRateLimiter {
	return NewEndpointRateLimiter().
		Configure("/api/v1/payment/process", 5, 2).         // 5 per minute, burst 2
		Configure("/api/v1/payment/curlec/initiate", 3, 1). // 3 per minute, burst 1
		Configure("/api/v1/payment/curlec/verify", 10, 3).  // 10 per minute, burst 3
		Configure("/api/v1/payment/webhook", 100, 20)       // 100 per minute for webhooks
}
