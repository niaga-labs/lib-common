package middleware

import (
	"strings"

	"github.com/gin-gonic/gin"
)

// CORSMiddleware handles Cross-Origin Resource Sharing with wildcard (DEPRECATED - use CORSWithOrigins instead)
// This function is kept for backward compatibility but should not be used in production
func CORSMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS, GET, PUT, DELETE, PATCH")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	}
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
func isOriginAllowed(origin string, allowedOrigins []string) bool {
	if origin == "" {
		return false
	}

	for _, allowed := range allowedOrigins {
		if allowed == "*" || allowed == origin {
			return true
		}
	}

	return false
}
