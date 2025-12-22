package middleware

import (
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/niaga-platform/lib-common/auth"
	"github.com/niaga-platform/lib-common/response"
)

// AuthMiddleware validates JWT tokens
func AuthMiddleware(jwtManager *auth.JWTManager) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			response.Unauthorized(c, "Authorization header required")
			c.Abort()
			return
		}

		// Extract token from "Bearer <token>"
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || parts[0] != "Bearer" {
			response.Unauthorized(c, "Invalid authorization header format")
			c.Abort()
			return
		}

		tokenString := parts[1]
		claims, err := jwtManager.ValidateToken(tokenString)
		if err != nil {
			response.Unauthorized(c, "Invalid or expired token")
			c.Abort()
			return
		}

		// Set user information in context
		c.Set("user_id", claims.UserID.String())
		c.Set("user_email", claims.Email)
		c.Set("user_role", claims.Role)
		c.Set("claims", claims)

		c.Next()
	}
}

// RoleMiddleware checks if user has required role
func RoleMiddleware(allowedRoles ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		role, exists := c.Get("user_role")
		if !exists {
			response.Forbidden(c, "User role not found")
			c.Abort()
			return
		}

		userRole := role.(string)
		for _, allowedRole := range allowedRoles {
			if userRole == allowedRole {
				c.Next()
				return
			}
		}

		response.Forbidden(c, "Insufficient permissions")
		c.Abort()
	}
}

// RequireAdmin is a convenience middleware that checks for admin role
// It can parse JWT from Authorization header or use pre-set claims/role from context
func RequireAdmin() gin.HandlerFunc {
	return func(c *gin.Context) {
		var userRole string

		// First try to get role from context (if set by prior middleware)
		if claims, exists := c.Get("claims"); exists {
			if claimsMap, ok := claims.(map[string]interface{}); ok {
				if role, ok := claimsMap["role"].(string); ok {
					userRole = role
				}
			}
		}

		// Try direct role check from context
		if userRole == "" {
			if role, exists := c.Get("role"); exists {
				if r, ok := role.(string); ok {
					userRole = r
				}
			}
		}
		if userRole == "" {
			if role, exists := c.Get("user_role"); exists {
				if r, ok := role.(string); ok {
					userRole = r
				}
			}
		}

		// If still no role, try to parse JWT from Authorization header
		if userRole == "" {
			authHeader := c.GetHeader("Authorization")
			if authHeader != "" {
				parts := strings.SplitN(authHeader, " ", 2)
				if len(parts) == 2 && parts[0] == "Bearer" {
					tokenString := parts[1]
					// Parse JWT without verification (just extract claims)
					// The token is already verified by the auth service
					role := auth.ExtractRoleFromToken(tokenString)
					if role != "" {
						userRole = role
						c.Set("user_role", role)
					}
				}
			}
		}

		if userRole == "" {
			response.Forbidden(c, "User role not found")
			c.Abort()
			return
		}

		if userRole != "admin" && userRole != "super_admin" {
			response.Forbidden(c, "Admin access required")
			c.Abort()
			return
		}

		c.Next()
	}
}
