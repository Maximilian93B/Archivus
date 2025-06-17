package middleware

import (
	"net/http"
	"strings"

	"github.com/archivus/archivus/internal/domain/services"
	"github.com/archivus/archivus/internal/infrastructure/database/models"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// UserContext holds user information extracted from JWT token
type UserContext struct {
	UserID   uuid.UUID       `json:"user_id"`
	TenantID uuid.UUID       `json:"tenant_id"`
	Email    string          `json:"email"`
	Role     models.UserRole `json:"role"`
	IsActive bool            `json:"is_active"`
}

// AuthMiddleware creates authentication middleware using Supabase
func AuthMiddleware(authService services.SupabaseAuthService, userService *services.UserService) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Extract token from Authorization header
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error":   "missing_authorization",
				"message": "Authorization header is required",
			})
			c.Abort()
			return
		}

		// Check Bearer token format
		tokenParts := strings.Split(authHeader, " ")
		if len(tokenParts) != 2 || tokenParts[0] != "Bearer" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error":   "invalid_authorization_format",
				"message": "Authorization header must be in format: Bearer <token>",
			})
			c.Abort()
			return
		}

		accessToken := tokenParts[1]

		// Validate token with Supabase
		supabaseUser, err := authService.ValidateToken(accessToken)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error":   "invalid_token",
				"message": "Token validation failed",
				"details": err.Error(),
			})
			c.Abort()
			return
		}

		if supabaseUser == nil {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error":   "invalid_user",
				"message": "User not found or inactive",
			})
			c.Abort()
			return
		}

		// Get full user details from our database using the validated token
		user, err := userService.ValidateToken(c.Request.Context(), accessToken)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error":   "user_not_found",
				"message": "User not found in system",
				"details": err.Error(),
			})
			c.Abort()
			return
		}

		// Check if user is active
		if !user.IsActive {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error":   "user_inactive",
				"message": "User account is inactive",
			})
			c.Abort()
			return
		}

		// Create user context
		userCtx := &UserContext{
			UserID:   user.ID,
			TenantID: user.TenantID,
			Email:    user.Email,
			Role:     user.Role,
			IsActive: user.IsActive,
		}

		// Store user context in gin context
		c.Set("user", userCtx)
		c.Set("user_id", user.ID)
		c.Set("tenant_id", user.TenantID)
		c.Set("user_role", user.Role)
		c.Set("access_token", accessToken)

		// Continue to next handler
		c.Next()
	}
}

// OptionalAuthMiddleware allows both authenticated and unauthenticated requests
// Used for endpoints that provide different responses based on auth status
func OptionalAuthMiddleware(authService services.SupabaseAuthService, userService *services.UserService) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			// No auth header - continue without user context
			c.Next()
			return
		}

		// Try to validate token - if it fails, continue without user context
		tokenParts := strings.Split(authHeader, " ")
		if len(tokenParts) != 2 || tokenParts[0] != "Bearer" {
			c.Next()
			return
		}

		accessToken := tokenParts[1]
		supabaseUser, err := authService.ValidateToken(accessToken)
		if err != nil || supabaseUser == nil {
			c.Next()
			return
		}

		// Get user from database using validated token
		user, err := userService.ValidateToken(c.Request.Context(), accessToken)
		if err != nil || !user.IsActive {
			c.Next()
			return
		}

		// Store user context if validation succeeds
		userCtx := &UserContext{
			UserID:   user.ID,
			TenantID: user.TenantID,
			Email:    user.Email,
			Role:     user.Role,
			IsActive: user.IsActive,
		}

		c.Set("user", userCtx)
		c.Set("user_id", user.ID)
		c.Set("tenant_id", user.TenantID)
		c.Set("user_role", user.Role)
		c.Set("access_token", accessToken)

		c.Next()
	}
}

// AdminRequiredMiddleware ensures only admin users can access the endpoint
func AdminRequiredMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		userCtx := GetUserContext(c)
		if userCtx == nil {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error":   "authentication_required",
				"message": "User must be authenticated",
			})
			c.Abort()
			return
		}

		if userCtx.Role != models.UserRoleAdmin {
			c.JSON(http.StatusForbidden, gin.H{
				"error":   "admin_required",
				"message": "Admin privileges required",
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

// TenantIsolationMiddleware ensures users can only access their tenant's data
// This middleware should be used on endpoints that accept tenant-scoped resource IDs
func TenantIsolationMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		userCtx := GetUserContext(c)
		if userCtx == nil {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error":   "authentication_required",
				"message": "User must be authenticated",
			})
			c.Abort()
			return
		}

		// Add tenant ID to query parameters if not present
		// This ensures all database queries are automatically scoped to the user's tenant
		if c.Query("tenant_id") == "" {
			// For POST/PUT requests, we can add tenant_id to the request context
			// The handlers should use this to filter data
			c.Set("enforced_tenant_id", userCtx.TenantID)
		}

		c.Next()
	}
}

// GetUserContext retrieves user context from gin context
// This is a helper function used by handlers to get current user info
func GetUserContext(c *gin.Context) *UserContext {
	if userCtx, exists := c.Get("user"); exists {
		if user, ok := userCtx.(*UserContext); ok {
			return user
		}
	}
	return nil
}

// GetUserID retrieves user ID from gin context
func GetUserID(c *gin.Context) (uuid.UUID, bool) {
	if userID, exists := c.Get("user_id"); exists {
		if id, ok := userID.(uuid.UUID); ok {
			return id, true
		}
	}
	return uuid.Nil, false
}

// GetTenantID retrieves tenant ID from gin context
func GetTenantID(c *gin.Context) (uuid.UUID, bool) {
	if tenantID, exists := c.Get("tenant_id"); exists {
		if id, ok := tenantID.(uuid.UUID); ok {
			return id, true
		}
	}
	return uuid.Nil, false
}

// RequirePermission creates middleware that checks for specific permission
func RequirePermission(permission string, userService *services.UserService) gin.HandlerFunc {
	return func(c *gin.Context) {
		userCtx := GetUserContext(c)
		if userCtx == nil {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error":   "authentication_required",
				"message": "User must be authenticated",
			})
			c.Abort()
			return
		}

		// Check permission using user service
		hasPermission, err := userService.CheckPermission(c.Request.Context(), userCtx.UserID, permission)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":   "permission_check_failed",
				"message": "Failed to check user permissions",
				"details": err.Error(),
			})
			c.Abort()
			return
		}

		if !hasPermission {
			c.JSON(http.StatusForbidden, gin.H{
				"error":   "insufficient_permissions",
				"message": "User does not have required permission: " + permission,
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

// ErrorResponse represents API error response
type ErrorResponse struct {
	Error   string      `json:"error"`
	Message string      `json:"message"`
	Details interface{} `json:"details,omitempty"`
	Code    string      `json:"code,omitempty"`
}
