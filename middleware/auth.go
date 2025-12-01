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
