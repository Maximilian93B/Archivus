package handlers

import (
	"net/http"
	"strconv"

	"github.com/archivus/archivus/internal/app/middleware"
	"github.com/archivus/archivus/internal/domain/repositories"
	"github.com/archivus/archivus/internal/domain/services"
	"github.com/archivus/archivus/internal/infrastructure/database/models"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// UserHandler handles user management operations
type UserHandler struct {
	userService   *services.UserService
	tenantService *services.TenantService
}

// NewUserHandler creates a new user handler
func NewUserHandler(
	userService *services.UserService,
	tenantService *services.TenantService,
) *UserHandler {
	return &UserHandler{
		userService:   userService,
		tenantService: tenantService,
	}
}

// RegisterRoutes sets up the user management routes
func (h *UserHandler) RegisterRoutes(router *gin.RouterGroup) {
	users := router.Group("/users")
	// Note: Auth middleware should be applied at the server level for consistency
	{
		// User profile routes
		users.GET("/profile", h.GetProfile)
		users.PUT("/profile", h.UpdateProfile)
		users.POST("/change-password", h.ChangePassword)

		// Admin user management routes (require admin privileges)
		adminUsers := users.Group("")
		adminUsers.Use(h.requireAdminMiddleware())
		{
			adminUsers.GET("", h.ListUsers)
			adminUsers.POST("", h.CreateUser)
			adminUsers.PUT("/:id", h.UpdateUser)
			adminUsers.DELETE("/:id", h.DeleteUser)
			adminUsers.PUT("/:id/role", h.UpdateUserRole)
			adminUsers.PUT("/:id/activate", h.ActivateUser)
			adminUsers.PUT("/:id/deactivate", h.DeactivateUser)
		}
	}
}

// Request/Response DTOs

// UpdateProfileRequest contains user profile update data
type UpdateProfileRequest struct {
	FirstName  string `json:"first_name" binding:"required,min=2,max=50"`
	LastName   string `json:"last_name" binding:"required,min=2,max=50"`
	Department string `json:"department,omitempty" binding:"max=100"`
	JobTitle   string `json:"job_title,omitempty" binding:"max=100"`
	Phone      string `json:"phone,omitempty" binding:"max=20"`
}

// ChangePasswordRequest contains password change data
type ChangePasswordRequest struct {
	CurrentPassword string `json:"current_password" binding:"required"`
	NewPassword     string `json:"new_password" binding:"required,min=8"`
}

// CreateUserRequest contains user creation data (admin only)
type CreateUserRequest struct {
	Email      string          `json:"email" binding:"required,email"`
	Password   string          `json:"password" binding:"required,min=8"`
	FirstName  string          `json:"first_name" binding:"required,min=2,max=50"`
	LastName   string          `json:"last_name" binding:"required,min=2,max=50"`
	Role       models.UserRole `json:"role" binding:"required"`
	Department string          `json:"department,omitempty" binding:"max=100"`
	JobTitle   string          `json:"job_title,omitempty" binding:"max=100"`
}

// UpdateUserRequest contains user update data (admin only)
type UpdateUserRequest struct {
	FirstName  *string          `json:"first_name,omitempty" binding:"omitempty,min=2,max=50"`
	LastName   *string          `json:"last_name,omitempty" binding:"omitempty,min=2,max=50"`
	Role       *models.UserRole `json:"role,omitempty"`
	Department *string          `json:"department,omitempty" binding:"omitempty,max=100"`
	JobTitle   *string          `json:"job_title,omitempty" binding:"omitempty,max=100"`
	IsActive   *bool            `json:"is_active,omitempty"`
}

// UpdateRoleRequest contains role update data
type UpdateRoleRequest struct {
	Role models.UserRole `json:"role" binding:"required"`
}

// UserProfileResponse represents user data in API responses
type UserProfileResponse struct {
	ID            uuid.UUID       `json:"id"`
	Email         string          `json:"email"`
	FirstName     string          `json:"first_name"`
	LastName      string          `json:"last_name"`
	Role          models.UserRole `json:"role"`
	Department    string          `json:"department,omitempty"`
	JobTitle      string          `json:"job_title,omitempty"`
	Phone         string          `json:"phone,omitempty"`
	IsActive      bool            `json:"is_active"`
	EmailVerified bool            `json:"email_verified"`
	MFAEnabled    bool            `json:"mfa_enabled"`
	LastLoginAt   *string         `json:"last_login_at,omitempty"`
	CreatedAt     string          `json:"created_at"`
	UpdatedAt     string          `json:"updated_at"`
}

// UserListResponse represents paginated user list
type UserListResponse struct {
	Users      []UserProfileResponse `json:"users"`
	Total      int64                 `json:"total"`
	Page       int                   `json:"page"`
	PerPage    int                   `json:"per_page"`
	TotalPages int                   `json:"total_pages"`
}

// Handler Methods

// GetProfile retrieves the current user's profile
// @Summary Get user profile
// @Description Get current authenticated user's profile information
// @Tags users
// @Produce json
// @Success 200 {object} UserProfileResponse
// @Failure 401 {object} ErrorResponse
// @Router /users/profile [get]
func (h *UserHandler) GetProfile(c *gin.Context) {
	userCtx := getUserContext(c)
	if userCtx == nil {
		c.JSON(http.StatusUnauthorized, ErrorResponse{
			Error:   "unauthorized",
			Message: "User context not found",
		})
		return
	}

	// Get user profile through UserService
	profile, err := h.userService.GetUserProfile(c.Request.Context(), userCtx.UserID)
	if err != nil {
		c.JSON(http.StatusNotFound, ErrorResponse{
			Error:   "user_not_found",
			Message: "User not found",
		})
		return
	}

	c.JSON(http.StatusOK, convertToUserProfileResponse(profile.User))
}

// UpdateProfile updates the current user's profile
// @Summary Update user profile
// @Description Update current authenticated user's profile information
// @Tags users
// @Accept json
// @Produce json
// @Param request body UpdateProfileRequest true "Profile update request"
// @Success 200 {object} UserProfileResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Router /users/profile [put]
func (h *UserHandler) UpdateProfile(c *gin.Context) {
	userCtx := getUserContext(c)
	if userCtx == nil {
		c.JSON(http.StatusUnauthorized, ErrorResponse{
			Error:   "unauthorized",
			Message: "User context not found",
		})
		return
	}

	var req UpdateProfileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "invalid_request",
			Message: "Invalid request format",
			Details: err.Error(),
		})
		return
	}

	// Prepare updates map
	updates := map[string]interface{}{
		"first_name": req.FirstName,
		"last_name":  req.LastName,
		"department": req.Department,
		"job_title":  req.JobTitle,
	}

	// Update user through UserService
	updatedUser, err := h.userService.UpdateUser(c.Request.Context(), userCtx.UserID, updates, userCtx.UserID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "update_failed",
			Message: "Failed to update user profile",
			Details: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, convertToUserProfileResponse(updatedUser))
}

// ChangePassword changes the current user's password
// @Summary Change password
// @Description Change current authenticated user's password
// @Tags users
// @Accept json
// @Produce json
// @Param request body ChangePasswordRequest true "Password change request"
// @Success 200 {object} SuccessResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Router /users/change-password [post]
func (h *UserHandler) ChangePassword(c *gin.Context) {
	userCtx := getUserContext(c)
	if userCtx == nil {
		c.JSON(http.StatusUnauthorized, ErrorResponse{
			Error:   "unauthorized",
			Message: "User context not found",
		})
		return
	}

	var req ChangePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "invalid_request",
			Message: "Invalid request format",
			Details: err.Error(),
		})
		return
	}

	// Get access token from context
	accessToken, exists := c.Get("access_token")
	if !exists {
		c.JSON(http.StatusUnauthorized, ErrorResponse{
			Error:   "access_token_missing",
			Message: "Access token not found",
		})
		return
	}

	// Change password through user service (correct signature)
	err := h.userService.ChangePassword(c.Request.Context(), userCtx.UserID, accessToken.(string), req.NewPassword)
	if err != nil {
		if err == services.ErrInvalidCredentials {
			c.JSON(http.StatusBadRequest, ErrorResponse{
				Error:   "invalid_current_password",
				Message: "Current password is incorrect",
			})
			return
		}
		if err == services.ErrWeakPassword {
			c.JSON(http.StatusBadRequest, ErrorResponse{
				Error:   "weak_password",
				Message: "New password does not meet security requirements",
			})
			return
		}

		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "password_change_failed",
			Message: "Failed to change password",
			Details: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, SuccessResponse{
		Message: "Password changed successfully",
	})
}

// ListUsers lists all users (admin only)
// @Summary List users
// @Description List all users in the tenant (admin only)
// @Tags users
// @Produce json
// @Param page query int false "Page number" default(1)
// @Param per_page query int false "Items per page" default(20)
// @Param search query string false "Search term"
// @Param sort_by query string false "Sort field"
// @Param sort_desc query bool false "Sort descending"
// @Success 200 {object} UserListResponse
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Router /users [get]
func (h *UserHandler) ListUsers(c *gin.Context) {
	userCtx := getUserContext(c)
	if userCtx == nil {
		c.JSON(http.StatusUnauthorized, ErrorResponse{
			Error:   "unauthorized",
			Message: "User context not found",
		})
		return
	}

	// Parse query parameters
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	perPage, _ := strconv.Atoi(c.DefaultQuery("per_page", "20"))
	if page < 1 {
		page = 1
	}
	if perPage < 1 || perPage > 100 {
		perPage = 20
	}

	search := c.Query("search")
	sortBy := c.Query("sort_by")
	sortDesc := c.Query("sort_desc") == "true"

	// Create list parameters using the repository interface
	params := repositories.ListParams{
		Page:     page,
		PageSize: perPage,
		Search:   search,
		SortBy:   sortBy,
		SortDesc: sortDesc,
	}

	// Get users using the correct UserService method signature
	users, total, err := h.userService.ListUsers(c.Request.Context(), userCtx.TenantID, params)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "list_failed",
			Message: "Failed to list users",
			Details: err.Error(),
		})
		return
	}

	// Convert to response format
	userResponses := make([]UserProfileResponse, len(users))
	for i, user := range users {
		userResponses[i] = convertToUserProfileResponse(&user)
	}

	totalPages := int((total + int64(perPage) - 1) / int64(perPage))

	response := UserListResponse{
		Users:      userResponses,
		Total:      total,
		Page:       page,
		PerPage:    perPage,
		TotalPages: totalPages,
	}

	c.JSON(http.StatusOK, response)
}

// CreateUser creates a new user (admin only)
// @Summary Create user
// @Description Create a new user in the tenant (admin only)
// @Tags users
// @Accept json
// @Produce json
// @Param request body CreateUserRequest true "User creation request"
// @Success 201 {object} UserProfileResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Failure 409 {object} ErrorResponse
// @Router /users [post]
func (h *UserHandler) CreateUser(c *gin.Context) {
	userCtx := getUserContext(c)
	if userCtx == nil {
		c.JSON(http.StatusUnauthorized, ErrorResponse{
			Error:   "unauthorized",
			Message: "User context not found",
		})
		return
	}

	var req CreateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "invalid_request",
			Message: "Invalid request format",
			Details: err.Error(),
		})
		return
	}

	// Create user parameters
	params := services.CreateUserParams{
		TenantID:   userCtx.TenantID,
		Email:      req.Email,
		Password:   req.Password,
		FirstName:  req.FirstName,
		LastName:   req.LastName,
		Role:       req.Role,
		Department: req.Department,
		JobTitle:   req.JobTitle,
		CreatedBy:  userCtx.UserID,
	}

	// Create user
	user, err := h.userService.CreateUser(c.Request.Context(), params)
	if err != nil {
		if err == services.ErrUserExists {
			c.JSON(http.StatusConflict, ErrorResponse{
				Error:   "user_exists",
				Message: "User with this email already exists",
			})
			return
		}
		if err == services.ErrWeakPassword {
			c.JSON(http.StatusBadRequest, ErrorResponse{
				Error:   "weak_password",
				Message: "Password does not meet security requirements",
			})
			return
		}

		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "create_failed",
			Message: "Failed to create user",
			Details: err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, convertToUserProfileResponse(user))
}

// UpdateUser updates an existing user (admin only)
// @Summary Update user
// @Description Update an existing user (admin only)
// @Tags users
// @Accept json
// @Produce json
// @Param id path string true "User ID"
// @Param request body UpdateUserRequest true "User update request"
// @Success 200 {object} UserProfileResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Router /users/{id} [put]
func (h *UserHandler) UpdateUser(c *gin.Context) {
	userCtx := getUserContext(c)
	if userCtx == nil {
		c.JSON(http.StatusUnauthorized, ErrorResponse{
			Error:   "unauthorized",
			Message: "User context not found",
		})
		return
	}

	// Parse user ID
	userIDStr := c.Param("id")
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "invalid_user_id",
			Message: "Invalid user ID format",
		})
		return
	}

	var req UpdateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "invalid_request",
			Message: "Invalid request format",
			Details: err.Error(),
		})
		return
	}

	// Get existing user to verify tenant access
	profile, err := h.userService.GetUserProfile(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusNotFound, ErrorResponse{
			Error:   "user_not_found",
			Message: "User not found",
		})
		return
	}

	// Check tenant access
	if profile.User.TenantID != userCtx.TenantID {
		c.JSON(http.StatusForbidden, ErrorResponse{
			Error:   "access_denied",
			Message: "Cannot access user from different tenant",
		})
		return
	}

	// Prepare updates map
	updates := make(map[string]interface{})
	if req.FirstName != nil {
		updates["first_name"] = *req.FirstName
	}
	if req.LastName != nil {
		updates["last_name"] = *req.LastName
	}
	if req.Role != nil {
		updates["role"] = *req.Role
	}
	if req.Department != nil {
		updates["department"] = *req.Department
	}
	if req.JobTitle != nil {
		updates["job_title"] = *req.JobTitle
	}

	// Update user
	updatedUser, err := h.userService.UpdateUser(c.Request.Context(), userID, updates, userCtx.UserID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "update_failed",
			Message: "Failed to update user",
			Details: err.Error(),
		})
		return
	}

	// Handle activation/deactivation separately
	if req.IsActive != nil {
		if *req.IsActive {
			err = h.userService.ReactivateUser(c.Request.Context(), userID, userCtx.UserID)
		} else {
			err = h.userService.DeactivateUser(c.Request.Context(), userID, userCtx.UserID)
		}
		if err != nil {
			c.JSON(http.StatusInternalServerError, ErrorResponse{
				Error:   "status_update_failed",
				Message: "Failed to update user status",
				Details: err.Error(),
			})
			return
		}
		// Get updated user
		profile, _ = h.userService.GetUserProfile(c.Request.Context(), userID)
		updatedUser = profile.User
	}

	c.JSON(http.StatusOK, convertToUserProfileResponse(updatedUser))
}

// DeleteUser soft deletes a user (admin only)
// @Summary Delete user
// @Description Soft delete a user (admin only)
// @Tags users
// @Param id path string true "User ID"
// @Success 200 {object} SuccessResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Router /users/{id} [delete]
func (h *UserHandler) DeleteUser(c *gin.Context) {
	userCtx := getUserContext(c)
	if userCtx == nil {
		c.JSON(http.StatusUnauthorized, ErrorResponse{
			Error:   "unauthorized",
			Message: "User context not found",
		})
		return
	}

	// Parse user ID
	userIDStr := c.Param("id")
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "invalid_user_id",
			Message: "Invalid user ID format",
		})
		return
	}

	// Prevent self-deletion
	if userID == userCtx.UserID {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "cannot_delete_self",
			Message: "Cannot delete your own account",
		})
		return
	}

	// Get user to check tenant
	profile, err := h.userService.GetUserProfile(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusNotFound, ErrorResponse{
			Error:   "user_not_found",
			Message: "User not found",
		})
		return
	}

	// Check tenant access
	if profile.User.TenantID != userCtx.TenantID {
		c.JSON(http.StatusForbidden, ErrorResponse{
			Error:   "access_denied",
			Message: "Cannot access user from different tenant",
		})
		return
	}

	// Deactivate user (we use deactivate instead of hard delete)
	err = h.userService.DeactivateUser(c.Request.Context(), userID, userCtx.UserID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "delete_failed",
			Message: "Failed to delete user",
			Details: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, SuccessResponse{
		Message: "User deleted successfully",
	})
}

// UpdateUserRole updates a user's role (admin only)
// @Summary Update user role
// @Description Update a user's role (admin only)
// @Tags users
// @Accept json
// @Produce json
// @Param id path string true "User ID"
// @Param request body UpdateRoleRequest true "Role update request"
// @Success 200 {object} UserProfileResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Router /users/{id}/role [put]
func (h *UserHandler) UpdateUserRole(c *gin.Context) {
	userCtx := getUserContext(c)
	if userCtx == nil {
		c.JSON(http.StatusUnauthorized, ErrorResponse{
			Error:   "unauthorized",
			Message: "User context not found",
		})
		return
	}

	// Parse user ID
	userIDStr := c.Param("id")
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "invalid_user_id",
			Message: "Invalid user ID format",
		})
		return
	}

	var req UpdateRoleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "invalid_request",
			Message: "Invalid request format",
			Details: err.Error(),
		})
		return
	}

	// Get user to check tenant
	profile, err := h.userService.GetUserProfile(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusNotFound, ErrorResponse{
			Error:   "user_not_found",
			Message: "User not found",
		})
		return
	}

	// Check tenant access
	if profile.User.TenantID != userCtx.TenantID {
		c.JSON(http.StatusForbidden, ErrorResponse{
			Error:   "access_denied",
			Message: "Cannot access user from different tenant",
		})
		return
	}

	// Update role
	updates := map[string]interface{}{
		"role": req.Role,
	}
	updatedUser, err := h.userService.UpdateUser(c.Request.Context(), userID, updates, userCtx.UserID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "update_failed",
			Message: "Failed to update user role",
			Details: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, convertToUserProfileResponse(updatedUser))
}

// ActivateUser activates a user account (admin only)
// @Summary Activate user
// @Description Activate a user account (admin only)
// @Tags users
// @Param id path string true "User ID"
// @Success 200 {object} UserProfileResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Router /users/{id}/activate [put]
func (h *UserHandler) ActivateUser(c *gin.Context) {
	h.updateUserStatus(c, true)
}

// DeactivateUser deactivates a user account (admin only)
// @Summary Deactivate user
// @Description Deactivate a user account (admin only)
// @Tags users
// @Param id path string true "User ID"
// @Success 200 {object} UserProfileResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Router /users/{id}/deactivate [put]
func (h *UserHandler) DeactivateUser(c *gin.Context) {
	h.updateUserStatus(c, false)
}

// Helper Methods

// updateUserStatus is a helper to activate/deactivate users
func (h *UserHandler) updateUserStatus(c *gin.Context, isActive bool) {
	userCtx := getUserContext(c)
	if userCtx == nil {
		c.JSON(http.StatusUnauthorized, ErrorResponse{
			Error:   "unauthorized",
			Message: "User context not found",
		})
		return
	}

	// Parse user ID
	userIDStr := c.Param("id")
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "invalid_user_id",
			Message: "Invalid user ID format",
		})
		return
	}

	// Prevent self-deactivation
	if userID == userCtx.UserID && !isActive {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "cannot_deactivate_self",
			Message: "Cannot deactivate your own account",
		})
		return
	}

	// Get user to check tenant
	profile, err := h.userService.GetUserProfile(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusNotFound, ErrorResponse{
			Error:   "user_not_found",
			Message: "User not found",
		})
		return
	}

	// Check tenant access
	if profile.User.TenantID != userCtx.TenantID {
		c.JSON(http.StatusForbidden, ErrorResponse{
			Error:   "access_denied",
			Message: "Cannot access user from different tenant",
		})
		return
	}

	// Update status using appropriate service method
	if isActive {
		err = h.userService.ReactivateUser(c.Request.Context(), userID, userCtx.UserID)
	} else {
		err = h.userService.DeactivateUser(c.Request.Context(), userID, userCtx.UserID)
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "update_failed",
			Message: "Failed to update user status",
			Details: err.Error(),
		})
		return
	}

	// Get updated user
	profile, _ = h.userService.GetUserProfile(c.Request.Context(), userID)
	c.JSON(http.StatusOK, convertToUserProfileResponse(profile.User))
}

// requireAdminMiddleware checks if user has admin privileges
func (h *UserHandler) requireAdminMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		userCtx := getUserContext(c)
		if userCtx == nil || userCtx.Role != models.UserRoleAdmin {
			c.JSON(http.StatusForbidden, ErrorResponse{
				Error:   "admin_required",
				Message: "Administrator privileges required",
			})
			c.Abort()
			return
		}
		c.Next()
	}
}

// getUserContext extracts user context from gin context
func getUserContext(c *gin.Context) *middleware.UserContext {
	user, exists := c.Get("user")
	if !exists {
		return nil
	}
	userCtx, ok := user.(*middleware.UserContext)
	if !ok {
		return nil
	}
	return userCtx
}

// convertToUserProfileResponse converts domain model to API response
func convertToUserProfileResponse(user *models.User) UserProfileResponse {
	var lastLoginAt *string
	if user.LastLoginAt != nil {
		formatted := user.LastLoginAt.Format("2006-01-02T15:04:05Z")
		lastLoginAt = &formatted
	}

	return UserProfileResponse{
		ID:            user.ID,
		Email:         user.Email,
		FirstName:     user.FirstName,
		LastName:      user.LastName,
		Role:          user.Role,
		Department:    user.Department,
		JobTitle:      user.JobTitle,
		Phone:         "", // Phone field not available in User model
		IsActive:      user.IsActive,
		EmailVerified: user.EmailVerified,
		MFAEnabled:    user.MFAEnabled,
		LastLoginAt:   lastLoginAt,
		CreatedAt:     user.CreatedAt.Format("2006-01-02T15:04:05Z"),
		UpdatedAt:     user.UpdatedAt.Format("2006-01-02T15:04:05Z"),
	}
}
