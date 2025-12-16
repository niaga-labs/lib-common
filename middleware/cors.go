package middleware

import (
	"strings"

	"github.com/gin-gonic/gin"
)

// CORSMiddleware is REMOVED for security - use CORSWithOrigins instead
// Wildcard CORS (*) enables CSRF attacks and is not safe for production
// This function now panics to prevent accidental use
func CORSMiddleware() gin.HandlerFunc {
	panic("CORSMiddleware with wildcard (*) is disabled for security. Use CORSWithOrigins() with explicit origins instead.")
}

// CORSWithOrigins handles CORS with specific allowed origins (RECOMMENDED)
// allowedOrigins should be a comma-separated string like "http://localhost:3000,http://localhost:3001"
func CORSWithOrigins(allowedOrigins string) gin.HandlerFunc {
	origins := parseOrigins(allowedOrigins)

	return func(c *gin.Context) {
		origin := c.Request.Header.Get("Origin")

		// Check if origin is allowed
		if isOriginAllowed(origin, origins) {
			c.Writer.Header().Set("Access-Control-Allow-Origin", origin)
			c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		}

		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS, GET, PUT, DELETE, PATCH")
		c.Writer.Header().Set("Access-Control-Max-Age", "86400") // 24 hours

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	}
}

// parseOrigins splits comma-separated origins string into a slice
func parseOrigins(originsStr string) []string {
	if originsStr == "" {
		return []string{}
	}

	origins := strings.Split(originsStr, ",")
	result := make([]string, 0, len(origins))

	for _, origin := range origins {
		trimmed := strings.TrimSpace(origin)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}

	return result
}

// isOriginAllowed checks if the origin is in the allowed list
// NOTE: Wildcard (*) is explicitly NOT supported for security reasons
func isOriginAllowed(origin string, allowedOrigins []string) bool {
	if origin == "" {
		return false
	}

	for _, allowed := range allowedOrigins {
		// Exact match only - wildcards are not allowed
		if allowed == origin {
			return true
		}
	}

	return false
}
