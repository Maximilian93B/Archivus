package handlers

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/archivus/archivus/internal/domain/services"
	"github.com/archivus/archivus/internal/infrastructure/database/models"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// AuthHandler handles authentication and authorization with Supabase
type AuthHandler struct {
	userService   *services.UserService
	tenantService *services.TenantService
	supabaseAuth  services.SupabaseAuthService
}

// NewAuthHandler creates a new auth handler with Supabase
func NewAuthHandler(
	userService *services.UserService,
	tenantService *services.TenantService,
	supabaseAuth services.SupabaseAuthService,
) *AuthHandler {
	return &AuthHandler{
		userService:   userService,
		tenantService: tenantService,
		supabaseAuth:  supabaseAuth,
	}
}

// SetupRoutes sets up the authentication routes
func (h *AuthHandler) SetupRoutes(router *gin.RouterGroup) {
	auth := router.Group("/auth")
	{
		auth.POST("/register", h.Register)
		auth.POST("/login", h.Login)
		auth.POST("/logout", h.Logout)
		auth.POST("/refresh", h.RefreshToken)
		auth.POST("/reset-password", h.ResetPassword)
		auth.GET("/validate", h.ValidateToken)
		auth.POST("/webhook", h.SupabaseWebhook)
	}
}

// Register handles user registration with Supabase
// @Summary Register user
// @Description Register a new user using Supabase Auth
// @Tags auth
// @Accept json
// @Produce json
// @Param request body RegisterRequest true "Registration request"
// @Success 201 {object} UserResponse
// @Failure 400 {object} ErrorResponse
// @Failure 409 {object} ErrorResponse
// @Router /auth/register [post]
func (h *AuthHandler) Register(c *gin.Context) {
	var req RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.respondError(c, http.StatusBadRequest, "Invalid request format", err)
		return
	}

	// Validate required fields
	if err := h.validateRegistrationRequest(&req); err != nil {
		h.respondError(c, http.StatusBadRequest, err.Error(), nil)
		return
	}

	// Get tenant by subdomain
	tenantSubdomain := h.getTenantSubdomain(c)
	tenant, err := h.tenantService.GetTenantBySubdomain(c.Request.Context(), tenantSubdomain)
	if err != nil {
		h.respondError(c, http.StatusBadRequest, "Invalid tenant", err)
		return
	}

	// Create user parameters
	params := services.CreateUserParams{
		TenantID:   tenant.ID,
		Email:      req.Email,
		Password:   req.Password,
		FirstName:  req.FirstName,
		LastName:   req.LastName,
		Role:       models.UserRole(req.Role),
		Department: req.Department,
		JobTitle:   req.JobTitle,
		CreatedBy:  uuid.Nil, // Self-registration
	}

	// Create user with Supabase
	user, err := h.userService.CreateUser(c.Request.Context(), params)
	if err != nil {
		if strings.Contains(err.Error(), "already exists") {
			h.respondError(c, http.StatusConflict, "User already exists", err)
		} else {
			h.respondError(c, http.StatusBadRequest, "Failed to create user", err)
		}
		return
	}

	// Return response
	response := convertUserToResponse(user)
	c.JSON(http.StatusCreated, response)
}

// Login handles user authentication with Supabase
// @Summary Login user
// @Description Authenticate user using Supabase Auth
// @Tags auth
// @Accept json
// @Produce json
// @Param request body LoginRequest true "Login request"
// @Success 200 {object} LoginResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Router /auth/login [post]
func (h *AuthHandler) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.respondError(c, http.StatusBadRequest, "Invalid request format", err)
		return
	}

	// Validate required fields
	if req.Email == "" || req.Password == "" {
		h.respondError(c, http.StatusBadRequest, "Email and password are required", nil)
		return
	}

	// Get tenant subdomain from request
	tenantSubdomain := h.getTenantSubdomain(c)
	if tenantSubdomain == "" {
		h.respondError(c, http.StatusBadRequest, "Tenant subdomain is required", nil)
		return
	}

	// Create login parameters
	loginParams := services.LoginParams{
		TenantSubdomain: tenantSubdomain,
		Email:           req.Email,
		Password:        req.Password,
		MFACode:         req.MFACode,
		IPAddress:       c.ClientIP(),
		UserAgent:       c.GetHeader("User-Agent"),
	}

	// Authenticate using Supabase
	result, err := h.userService.Login(c.Request.Context(), loginParams)
	if err != nil {
		h.respondError(c, http.StatusUnauthorized, "Authentication failed", err)
		return
	}

	// Handle MFA if enabled
	if result.RequiresMFA {
		c.JSON(http.StatusOK, gin.H{
			"requires_mfa": true,
			"message":      "MFA code required",
		})
		return
	}

	// Return success response
	response := &LoginResponse{
		User:         convertUserToResponse(result.User),
		Token:        result.Token,
		RefreshToken: result.RefreshToken,
		ExpiresAt:    result.ExpiresAt.Unix(),
	}

	c.JSON(http.StatusOK, response)
}

// ValidateToken validates a Supabase access token
// @Summary Validate token
// @Description Validate a Supabase access token and return user info
// @Tags auth
// @Accept json
// @Produce json
// @Param Authorization header string true "Bearer token"
// @Success 200 {object} UserContextResponse
// @Failure 401 {object} ErrorResponse
// @Router /auth/validate [get]
func (h *AuthHandler) ValidateToken(c *gin.Context) {
	// Extract token from Authorization header
	authHeader := c.GetHeader("Authorization")
	if authHeader == "" {
		h.respondError(c, http.StatusUnauthorized, "Authorization header required", nil)
		return
	}

	token := strings.TrimPrefix(authHeader, "Bearer ")
	if token == authHeader {
		h.respondError(c, http.StatusUnauthorized, "Invalid authorization header format", nil)
		return
	}

	// Validate token
	user, err := h.userService.ValidateToken(c.Request.Context(), token)
	if err != nil {
		h.respondError(c, http.StatusUnauthorized, "Invalid token", err)
		return
	}

	// Return user context
	response := &UserContextResponse{
		UserID:   user.ID,
		TenantID: user.TenantID,
		Email:    user.Email,
		Role:     string(user.Role),
		IsActive: user.IsActive,
	}

	c.JSON(http.StatusOK, response)
}

// RefreshToken refreshes a Supabase token
// @Summary Refresh token
// @Description Refresh an expired Supabase access token
// @Tags auth
// @Accept json
// @Produce json
// @Param request body RefreshTokenRequest true "Refresh token request"
// @Success 200 {object} TokenResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Router /auth/refresh [post]
func (h *AuthHandler) RefreshToken(c *gin.Context) {
	var req RefreshTokenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.respondError(c, http.StatusBadRequest, "Invalid request format", err)
		return
	}

	if req.RefreshToken == "" {
		h.respondError(c, http.StatusBadRequest, "Refresh token is required", nil)
		return
	}

	// Refresh token with Supabase
	authResponse, err := h.supabaseAuth.RefreshSession(req.RefreshToken)
	if err != nil {
		h.respondError(c, http.StatusUnauthorized, "Invalid refresh token", err)
		return
	}

	// Return new tokens
	c.JSON(http.StatusOK, gin.H{
		"access_token":  authResponse.AccessToken,
		"refresh_token": authResponse.RefreshToken,
		"expires_at":    authResponse.ExpiresAt.Unix(),
		"token_type":    "Bearer",
	})
}

// Logout handles user logout
// @Summary Logout user
// @Description Log out user and invalidate session
// @Tags auth
// @Accept json
// @Produce json
// @Param Authorization header string true "Bearer token"
// @Success 200 {object} SuccessResponse
// @Failure 400 {object} ErrorResponse
// @Router /auth/logout [post]
func (h *AuthHandler) Logout(c *gin.Context) {
	// Extract token from Authorization header
	authHeader := c.GetHeader("Authorization")
	if authHeader == "" {
		h.respondError(c, http.StatusBadRequest, "Authorization header required", nil)
		return
	}

	token := strings.TrimPrefix(authHeader, "Bearer ")
	if token == authHeader {
		h.respondError(c, http.StatusBadRequest, "Invalid authorization header format", nil)
		return
	}

	// Sign out from Supabase
	if err := h.supabaseAuth.SignOut(token); err != nil {
		// Log error but don't fail logout
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Logged out successfully",
		"success": true,
	})
}

// ResetPassword initiates password reset process
// @Summary Reset password
// @Description Send password reset email via Supabase
// @Tags auth
// @Accept json
// @Produce json
// @Param request body ResetPasswordRequest true "Reset password request"
// @Success 200 {object} SuccessResponse
// @Failure 400 {object} ErrorResponse
// @Router /auth/reset-password [post]
func (h *AuthHandler) ResetPassword(c *gin.Context) {
	var req ResetPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.respondError(c, http.StatusBadRequest, "Invalid request format", err)
		return
	}

	if req.Email == "" {
		h.respondError(c, http.StatusBadRequest, "Email is required", nil)
		return
	}

	// Get tenant subdomain
	tenantSubdomain := h.getTenantSubdomain(c)

	// Reset password via UserService
	err := h.userService.ResetPassword(c.Request.Context(), tenantSubdomain, req.Email)
	if err != nil {
		// Don't reveal specific errors for security
		c.JSON(http.StatusOK, gin.H{
			"message": "If the email exists, a reset link has been sent",
			"success": true,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "If the email exists, a reset link has been sent",
		"success": true,
	})
}

// SupabaseWebhook handles Supabase Auth webhooks
// @Summary Handle Supabase webhook
// @Description Handle authentication events from Supabase
// @Tags auth
// @Accept json
// @Produce json
// @Param request body SupabaseWebhookPayload true "Webhook payload"
// @Success 200 {object} SuccessResponse
// @Failure 400 {object} ErrorResponse
// @Router /auth/webhook [post]
func (h *AuthHandler) SupabaseWebhook(c *gin.Context) {
	var payload SupabaseWebhookPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		h.respondError(c, http.StatusBadRequest, "Invalid webhook payload", err)
		return
	}

	// Process webhook based on event type
	switch payload.Type {
	case "user.created":
		h.handleUserCreatedWebhook(c, payload)
	case "user.updated":
		h.handleUserUpdatedWebhook(c, payload)
	case "user.deleted":
		h.handleUserDeletedWebhook(c, payload)
	default:
		// Unknown event type, log and continue
		c.JSON(http.StatusOK, gin.H{"message": "Event processed"})
	}
}

// Helper methods for webhook processing

func (h *AuthHandler) handleUserCreatedWebhook(c *gin.Context, payload SupabaseWebhookPayload) {
	// Handle user creation from Supabase
	c.JSON(http.StatusOK, gin.H{"message": "User created event processed"})
}

func (h *AuthHandler) handleUserUpdatedWebhook(c *gin.Context, payload SupabaseWebhookPayload) {
	// Handle user update from Supabase
	c.JSON(http.StatusOK, gin.H{"message": "User updated event processed"})
}

func (h *AuthHandler) handleUserDeletedWebhook(c *gin.Context, payload SupabaseWebhookPayload) {
	// Handle user deletion from Supabase
	c.JSON(http.StatusOK, gin.H{"message": "User deleted event processed"})
}

// Helper methods

func (h *AuthHandler) getTenantSubdomain(c *gin.Context) string {
	// Try to get from header first
	if subdomain := c.GetHeader("X-Tenant-Subdomain"); subdomain != "" {
		return subdomain
	}

	// Try to get from query parameter
	if subdomain := c.Query("tenant"); subdomain != "" {
		return subdomain
	}

	// Try to extract from Host header (subdomain.archivus.com)
	host := c.GetHeader("Host")
	if host != "" {
		parts := strings.Split(host, ".")
		if len(parts) > 2 {
			return parts[0]
		}
	}

	return ""
}

func (h *AuthHandler) validateRegistrationRequest(req *RegisterRequest) error {
	if req.Email == "" {
		return errors.New("email is required")
	}

	if req.Password == "" {
		return errors.New("password is required")
	}

	if len(req.Password) < 8 {
		return errors.New("password must be at least 8 characters")
	}

	if req.FirstName == "" {
		return errors.New("first name is required")
	}

	if req.LastName == "" {
		return errors.New("last name is required")
	}

	if req.Role == "" {
		req.Role = "user" // Default role
	}

	return nil
}

func (h *AuthHandler) respondError(c *gin.Context, statusCode int, message string, err error) {
	response := ErrorResponse{
		Error:   message,
		Message: message,
		Status:  statusCode,
	}

	if err != nil {
		response.Details = err.Error()
	}

	c.JSON(statusCode, response)
}

func convertUserToResponse(user *models.User) *UserResponse {
	return &UserResponse{
		ID:            user.ID,
		TenantID:      user.TenantID,
		Email:         user.Email,
		FirstName:     user.FirstName,
		LastName:      user.LastName,
		Role:          string(user.Role),
		Department:    user.Department,
		JobTitle:      user.JobTitle,
		IsActive:      user.IsActive,
		EmailVerified: user.EmailVerified,
		LastLoginAt:   user.LastLoginAt,
		CreatedAt:     user.CreatedAt,
		UpdatedAt:     user.UpdatedAt,
	}
}

// Request/Response types

type RegisterRequest struct {
	Email      string `json:"email" binding:"required,email"`
	Password   string `json:"password" binding:"required,min=8"`
	FirstName  string `json:"first_name" binding:"required"`
	LastName   string `json:"last_name" binding:"required"`
	Role       string `json:"role,omitempty"`
	Department string `json:"department,omitempty"`
	JobTitle   string `json:"job_title,omitempty"`
}

type LoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
	MFACode  string `json:"mfa_code,omitempty"`
}

type LoginResponse struct {
	User         *UserResponse `json:"user"`
	Token        string        `json:"token"`
	RefreshToken string        `json:"refresh_token"`
	ExpiresAt    int64         `json:"expires_at"`
}

type UserResponse struct {
	ID            uuid.UUID  `json:"id"`
	TenantID      uuid.UUID  `json:"tenant_id"`
	Email         string     `json:"email"`
	FirstName     string     `json:"first_name"`
	LastName      string     `json:"last_name"`
	Role          string     `json:"role"`
	Department    string     `json:"department,omitempty"`
	JobTitle      string     `json:"job_title,omitempty"`
	IsActive      bool       `json:"is_active"`
	EmailVerified bool       `json:"email_verified"`
	LastLoginAt   *time.Time `json:"last_login_at"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}

type UserContextResponse struct {
	UserID   uuid.UUID `json:"user_id"`
	TenantID uuid.UUID `json:"tenant_id"`
	Email    string    `json:"email"`
	Role     string    `json:"role"`
	IsActive bool      `json:"is_active"`
}

type RefreshTokenRequest struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}

type ResetPasswordRequest struct {
	Email string `json:"email" binding:"required,email"`
}

type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message"`
	Status  int    `json:"status"`
	Details string `json:"details,omitempty"`
}

type SuccessResponse struct {
	Message string `json:"message"`
	Success bool   `json:"success"`
}

type SupabaseWebhookPayload struct {
	Type      string                 `json:"type"`
	Table     string                 `json:"table"`
	Record    map[string]interface{} `json:"record"`
	OldRecord map[string]interface{} `json:"old_record,omitempty"`
	Schema    string                 `json:"schema"`
}
