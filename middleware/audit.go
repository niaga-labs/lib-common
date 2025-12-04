package middleware

import (
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// AuditEvent represents a security-critical event
type AuditEvent struct {
	Timestamp  time.Time         `json:"timestamp"`
	EventType  string            `json:"event_type"`
	UserID     string            `json:"user_id,omitempty"`
	Email      string            `json:"email,omitempty"`
	IPAddress  string            `json:"ip_address"`
	UserAgent  string            `json:"user_agent"`
	Method     string            `json:"method"`
	Path       string            `json:"path"`
	StatusCode int               `json:"status_code,omitempty"`
	Details    map[string]string `json:"details,omitempty"`
	Success    bool              `json:"success"`
}

// AuditLogger middleware logs security-critical events
func AuditLogger(logger *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Skip non-sensitive routes
		if !isSensitiveRoute(c.Request.URL.Path) {
			c.Next()
			return
		}

		start := time.Now()
		c.Next()

		// Log after request is processed
		duration := time.Since(start)

		event := AuditEvent{
			Method:     c.Request.Method,
			Path:       c.Request.URL.Path,
			IPAddress:  c.ClientIP(),
			StatusCode: c.Writer.Status(),
			Success:    c.Writer.Status() < 400,
		}

		// Extract user info if available
		if userID, exists := c.Get("user_id"); exists {
			if uid, ok := userID.(string); ok {
				event.UserID = uid
			}
		}

		if email, exists := c.Get("email"); exists {
			if e, ok := email.(string); ok {
				event.Email = e
			}
		}

		// Determine event type
		event.EventType = determineEventType(c.Request.URL.Path, c.Request.Method)

		// Log the audit event
		logger.Info("Security Audit Event",
			zap.String("event_type", event.EventType),
			zap.String("user_id", event.UserID),
			zap.String("email", event.Email),
			zap.String("ip_address", event.IPAddress),
			zap.String("method", event.Method),
			zap.String("path", event.Path),
			zap.Int("status_code", event.StatusCode),
			zap.Bool("success", event.Success),
			zap.Duration("duration", duration),
		)
	}
}

// isSensitiveRoute checks if the route should be audited
func isSensitiveRoute(path string) bool {
	sensitivePatterns := []string{
		"/auth/login",
		"/auth/register",
		"/auth/logout",
		"/auth/refresh",
		"/auth/forgot-password",
		"/auth/reset-password",
		"/auth/change-password",
		"/admin/",
		"/permissions/",
		"/roles/",
		"/users/",
	}

	for _, pattern := range sensitivePatterns {
		if contains(path, pattern) {
			return true
		}
	}

	return false
}

// determineEventType determines the event type based on path and method
func determineEventType(path, method string) string {
	switch {
	case contains(path, "/login"):
		return "LOGIN_ATTEMPT"
	case contains(path, "/register"):
		return "REGISTRATION"
	case contains(path, "/logout"):
		return "LOGOUT"
	case contains(path, "/refresh"):
		return "TOKEN_REFRESH"
	case contains(path, "/forgot-password"):
		return "PASSWORD_RESET_REQUEST"
	case contains(path, "/reset-password"):
		return "PASSWORD_RESET"
	case contains(path, "/change-password"):
		return "PASSWORD_CHANGE"
	case contains(path, "/permissions/") && method == "POST":
		return "PERMISSION_GRANTED"
	case contains(path, "/permissions/") && method == "DELETE":
		return "PERMISSION_REVOKED"
	case contains(path, "/roles/") && method == "POST":
		return "ROLE_CREATED"
	case contains(path, "/roles/") && method == "PUT":
		return "ROLE_UPDATED"
	case contains(path, "/roles/") && method == "DELETE":
		return "ROLE_DELETED"
	case contains(path, "/admin/"):
		return "ADMIN_ACTION"
	default:
		return "SENSITIVE_ACCESS"
	}
}

// contains checks if a string contains a substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || containsSubstring(s, substr))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
