package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

// AdminRole represents the admin role levels
type AdminRole string

const (
	RoleSuperAdmin AdminRole = "super_admin"
	RoleAdmin      AdminRole = "admin"
	RoleManager    AdminRole = "manager"
	RoleStaff      AdminRole = "staff"
)

// RequireAdmin middleware ensures the user has admin privileges
func RequireAdmin() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get user claims from context (set by AuthMiddleware)
		claims, exists := c.Get("claims")
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "Authentication required",
				"code":  "AUTH_REQUIRED",
			})
			c.Abort()
			return
		}

		// Type assert to jwt.MapClaims
		userClaims, ok := claims.(jwt.MapClaims)
		if !ok {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "Invalid token claims",
				"code":  "INVALID_CLAIMS",
			})
			c.Abort()
			return
		}

		// Check if user has admin role
		role, _ := userClaims["role"].(string)
		if !isAdminRole(role) {
			c.JSON(http.StatusForbidden, gin.H{
				"error": "Admin access required",
				"code":  "ADMIN_REQUIRED",
			})
			c.Abort()
			return
		}

		// Store admin role in context for use in handlers
		c.Set("admin_role", role)
		c.Next()
	}
}

// RequireRole middleware ensures the user has a specific role or higher
func RequireRole(minRole AdminRole) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get user claims from context
		claims, exists := c.Get("claims")
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "Authentication required",
				"code":  "AUTH_REQUIRED",
			})
			c.Abort()
			return
		}

		// Type assert to jwt.MapClaims
		userClaims, ok := claims.(jwt.MapClaims)
		if !ok {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "Invalid token claims",
				"code":  "INVALID_CLAIMS",
			})
			c.Abort()
			return
		}

		// Get user role
		role, _ := userClaims["role"].(string)

		// Check if user has sufficient privileges
		if !hasMinimumRole(role, string(minRole)) {
			c.JSON(http.StatusForbidden, gin.H{
				"error": "Insufficient privileges",
				"code":  "INSUFFICIENT_PRIVILEGES",
				"required_role": minRole,
				"user_role": role,
			})
			c.Abort()
			return
		}

		c.Set("admin_role", role)
		c.Next()
	}
}

// RequireSuperAdmin middleware ensures the user is a super admin
func RequireSuperAdmin() gin.HandlerFunc {
	return RequireRole(RoleSuperAdmin)
}

// Helper function to check if a role is an admin role
func isAdminRole(role string) bool {
	switch role {
	case string(RoleSuperAdmin), string(RoleAdmin), string(RoleManager), string(RoleStaff):
		return true
	default:
		return false
	}
}

// Helper function to check role hierarchy
func hasMinimumRole(userRole, requiredRole string) bool {
	roleHierarchy := map[string]int{
		string(RoleSuperAdmin): 4,
		string(RoleAdmin):      3,
		string(RoleManager):    2,
		string(RoleStaff):      1,
		"customer":             0,
	}

	userLevel, userExists := roleHierarchy[userRole]
	requiredLevel, reqExists := roleHierarchy[requiredRole]

	if !userExists || !reqExists {
		return false
	}

	return userLevel >= requiredLevel
}

// GetAdminRole gets the admin role from context
func GetAdminRole(c *gin.Context) (string, bool) {
	role, exists := c.Get("admin_role")
	if !exists {
		return "", false
	}
	roleStr, ok := role.(string)
	return roleStr, ok
}

// GetUserID gets the user ID from JWT claims
func GetUserID(c *gin.Context) (string, bool) {
	claims, exists := c.Get("claims")
	if !exists {
		return "", false
	}

	userClaims, ok := claims.(jwt.MapClaims)
	if !ok {
		return "", false
	}

	userID, ok := userClaims["user_id"].(string)
	if !ok {
		// Try alternative claim names
		userID, ok = userClaims["sub"].(string)
		if !ok {
			userID, ok = userClaims["id"].(string)
		}
	}

	return userID, ok
}

// GetUserEmail gets the user email from JWT claims
func GetUserEmail(c *gin.Context) (string, bool) {
	claims, exists := c.Get("claims")
	if !exists {
		return "", false
	}

	userClaims, ok := claims.(jwt.MapClaims)
	if !ok {
		return "", false
	}

	email, ok := userClaims["email"].(string)
	return email, ok
}