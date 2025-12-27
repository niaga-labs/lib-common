package middleware

import (
	"github.com/gin-gonic/gin"
)

// SecurityConfig holds configuration for security headers
type SecurityConfig struct {
	// EnableStrictCSP enables strict Content Security Policy (recommended for production)
	// When false, allows 'unsafe-inline' and 'unsafe-eval' for development
	EnableStrictCSP bool
	// CSPReportURI is the URI to report CSP violations (optional)
	CSPReportURI string
}

// DefaultSecurityConfig returns secure defaults
func DefaultSecurityConfig() SecurityConfig {
	return SecurityConfig{
		EnableStrictCSP: true,
		CSPReportURI:    "",
	}
}

// SecurityHeaders adds security headers to all responses
// Uses strict CSP by default - use SecurityHeadersWithConfig for customization
func SecurityHeaders() gin.HandlerFunc {
	return SecurityHeadersWithConfig(DefaultSecurityConfig())
}

// SecurityHeadersWithConfig adds security headers with custom configuration
func SecurityHeadersWithConfig(config SecurityConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Prevent clickjacking
		c.Header("X-Frame-Options", "DENY")

		// Prevent MIME type sniffing
		c.Header("X-Content-Type-Options", "nosniff")

		// Enable XSS protection (legacy, but still useful for older browsers)
		c.Header("X-XSS-Protection", "1; mode=block")

		// Enforce HTTPS with preload
		c.Header("Strict-Transport-Security", "max-age=31536000; includeSubDomains; preload")

		// Content Security Policy
		// SECURITY: Strict CSP prevents XSS attacks more effectively
		var csp string
		if config.EnableStrictCSP {
			// Strict CSP - no unsafe-inline or unsafe-eval
			// Applications should use nonces or hashes for inline scripts
			csp = "default-src 'self'; " +
				"script-src 'self'; " +
				"style-src 'self'; " +
				"img-src 'self' data: https:; " +
				"font-src 'self'; " +
				"connect-src 'self'; " +
				"frame-ancestors 'none'; " +
				"form-action 'self'; " +
				"base-uri 'self'; " +
				"object-src 'none'"
		} else {
			// Relaxed CSP for development or legacy applications
			// WARNING: This is less secure and should not be used in production
			csp = "default-src 'self'; " +
				"script-src 'self' 'unsafe-inline' 'unsafe-eval'; " +
				"style-src 'self' 'unsafe-inline'; " +
				"img-src 'self' data: https:; " +
				"font-src 'self'; " +
				"connect-src 'self'"
		}

		if config.CSPReportURI != "" {
			csp += "; report-uri " + config.CSPReportURI
		}

		c.Header("Content-Security-Policy", csp)

		// Referrer Policy
		c.Header("Referrer-Policy", "strict-origin-when-cross-origin")

		// Permissions Policy (formerly Feature-Policy)
		c.Header("Permissions-Policy", "geolocation=(), microphone=(), camera=(), payment=(), usb=()")

		// Prevent caching of sensitive responses
		c.Header("Cache-Control", "no-store, no-cache, must-revalidate, proxy-revalidate")
		c.Header("Pragma", "no-cache")
		c.Header("Expires", "0")

		c.Next()
	}
}

// CORS configures Cross-Origin Resource Sharing
func CORS(allowedOrigins []string) gin.HandlerFunc {
	return func(c *gin.Context) {
		origin := c.Request.Header.Get("Origin")

		// Check if origin is allowed
		allowed := false
		for _, allowedOrigin := range allowedOrigins {
			if origin == allowedOrigin || allowedOrigin == "*" {
				allowed = true
				break
			}
		}

		if allowed {
			c.Header("Access-Control-Allow-Origin", origin)
		}

		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Requested-With")
		c.Header("Access-Control-Allow-Credentials", "true")
		c.Header("Access-Control-Max-Age", "86400") // 24 hours

		// Handle preflight requests
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	}
}
