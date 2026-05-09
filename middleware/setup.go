package middleware

import (
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// SetupConfig describes the canonical middleware chain a Niaga HTTP service
// installs at startup. Optional slots take nil/empty values when the service
// chooses to skip a layer (e.g. tracing not yet wired, no rate limiter
// for a low-traffic admin tool).
type SetupConfig struct {
	// Logger is required — used by recovery and request logger layers.
	Logger *zap.Logger

	// PreRequestID runs after recovery and before request-id generation.
	// Wire telemetry.TracingMiddleware(...) and monitoring.SentryMonitor.GinMiddleware()
	// here in that order. Leave nil to skip.
	PreRequestID []gin.HandlerFunc

	// AllowedOrigins is the comma-separated CORS allow-list. Empty string
	// disables the CORS layer entirely (typical for internal-only services
	// behind nginx; the gateway will set CORS upstream).
	AllowedOrigins string

	// SecurityHeaders overrides the security headers config. Nil installs
	// DefaultSecurityConfig() which enables strict CSP. Pass a custom config
	// in dev to relax CSP for hot-reload tooling.
	SecurityHeaders *SecurityConfig

	// EnableInputValidation toggles the SQL/XSS pattern check on query
	// strings and form bodies. Off by default — most services validate at
	// the handler/binding layer and the regex layer adds latency.
	EnableInputValidation bool

	// PostValidation runs after input validation and before route handlers.
	// Typical wiring is the global rate limiter (RateLimiter.RateLimit())
	// and any auth middleware that should apply to every route. Per-route
	// auth/rate limits should still be installed on the relevant groups,
	// not here. Leave nil to skip.
	PostValidation []gin.HandlerFunc
}

// SetupCommonMiddleware installs the canonical middleware chain on the given
// router engine. Order:
//
//	recovery → tracing → sentry → request-id → logger → cors →
//	security-headers → input-validation → rate-limiter
//
// Tracing and Sentry are passed in via PreRequestID; rate limiting via
// PostValidation. The order between them is preserved as written. Per-route
// concerns (auth, endpoint-specific rate limits) are NOT installed here —
// those belong on individual route groups.
//
// Panics if cfg.Logger is nil — the recovery and logger layers require it.
func SetupCommonMiddleware(r *gin.Engine, cfg SetupConfig) {
	if cfg.Logger == nil {
		panic("middleware.SetupCommonMiddleware: cfg.Logger is required")
	}

	r.Use(RecoveryMiddleware(cfg.Logger))

	for _, h := range cfg.PreRequestID {
		if h != nil {
			r.Use(h)
		}
	}

	r.Use(RequestID())
	r.Use(LoggerMiddleware(cfg.Logger))

	if cfg.AllowedOrigins != "" {
		r.Use(CORSWithOrigins(cfg.AllowedOrigins))
	}

	if cfg.SecurityHeaders != nil {
		r.Use(SecurityHeadersWithConfig(*cfg.SecurityHeaders))
	} else {
		r.Use(SecurityHeaders())
	}

	if cfg.EnableInputValidation {
		r.Use(InputValidation())
	}

	for _, h := range cfg.PostValidation {
		if h != nil {
			r.Use(h)
		}
	}
}
