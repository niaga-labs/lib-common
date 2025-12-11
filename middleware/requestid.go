package middleware

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

const (
	// RequestIDHeader is the header name for request ID
	RequestIDHeader = "X-Request-ID"
	// RequestIDKey is the context key for request ID
	RequestIDKey = "request_id"
)

// RequestID returns a middleware that generates or propagates request IDs
// If a request ID is provided in the header, it will be used
// Otherwise, a new UUID will be generated
func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Check if request ID exists in header
		requestID := c.GetHeader(RequestIDHeader)

		// Generate new request ID if not provided
		if requestID == "" {
			requestID = uuid.New().String()
		}

		// Set request ID in context
		c.Set(RequestIDKey, requestID)

		// Set request ID in response header
		c.Header(RequestIDHeader, requestID)

		c.Next()
	}
}

// GetRequestID returns the request ID from context
func GetRequestID(c *gin.Context) string {
	if requestID, exists := c.Get(RequestIDKey); exists {
		if id, ok := requestID.(string); ok {
			return id
		}
	}
	return ""
}
