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

// TenantHandler handles tenant management operations
type TenantHandler struct {
	tenantService *services.TenantService
	userService   *services.UserService
}

// NewTenantHandler creates a new tenant handler
func NewTenantHandler(
	tenantService *services.TenantService,
	userService *services.UserService,
) *TenantHandler {
	return &TenantHandler{
		tenantService: tenantService,
		userService:   userService,
	}
}

// RegisterRoutes sets up the tenant management routes
func (h *TenantHandler) RegisterRoutes(router *gin.RouterGroup) {
	tenant := router.Group("/tenant")
	// Note: Auth middleware should be applied at server level
	{
		// Tenant settings
		tenant.GET("/settings", h.GetSettings)
		tenant.PUT("/settings", h.requireAdminMiddleware(), h.UpdateSettings)

		// Usage statistics
		tenant.GET("/usage", h.GetUsage)

		// Tenant user management (admin only)
		tenantUsers := tenant.Group("/users")
		tenantUsers.Use(h.requireAdminMiddleware())
		{
			tenantUsers.GET("", h.GetTenantUsers)
		}
	}
}

// Request/Response DTOs

// TenantSettingsRequest contains tenant settings update data
type TenantSettingsRequest struct {
	Name         string                 `json:"name" binding:"required,min=2,max=100"`
	BusinessType string                 `json:"business_type,omitempty" binding:"max=50"`
	Industry     string                 `json:"industry,omitempty" binding:"max=50"`
	CompanySize  string                 `json:"company_size,omitempty" binding:"max=20"`
	TaxID        string                 `json:"tax_id,omitempty" binding:"max=50"`
	Address      map[string]interface{} `json:"address,omitempty"`
	Settings     map[string]interface{} `json:"settings,omitempty"`
}

// TenantSettingsResponse represents tenant settings in API responses
type TenantSettingsResponse struct {
	ID           uuid.UUID              `json:"id"`
	Name         string                 `json:"name"`
	Subdomain    string                 `json:"subdomain"`
	BusinessType string                 `json:"business_type"`
	Industry     string                 `json:"industry"`
	CompanySize  string                 `json:"company_size"`
	TaxID        string                 `json:"tax_id"`
	Address      map[string]interface{} `json:"address"`
	Settings     map[string]interface{} `json:"settings"`
	IsActive     bool                   `json:"is_active"`
	CreatedAt    string                 `json:"created_at"`
	UpdatedAt    string                 `json:"updated_at"`
}

// TenantUsageResponse represents tenant usage statistics
type TenantUsageResponse struct {
	TenantID       uuid.UUID `json:"tenant_id"`
	StorageUsed    int64     `json:"storage_used_bytes"`
	StorageQuota   int64     `json:"storage_quota_bytes"`
	StoragePercent float64   `json:"storage_usage_percent"`
	APIUsed        int       `json:"api_used"`
	APIQuota       int       `json:"api_quota"`
	APIPercent     float64   `json:"api_usage_percent"`
	TotalUsers     int64     `json:"total_users"`
	TotalDocuments int64     `json:"total_documents"`
	LastUpdated    string    `json:"last_updated"`
}

// TenantUsersResponse represents tenant users list
type TenantUsersResponse struct {
	Users      []UserSummary `json:"users"`
	Total      int64         `json:"total"`
	Page       int           `json:"page"`
	PerPage    int           `json:"per_page"`
	TotalPages int           `json:"total_pages"`
}

// UserSummary represents a summary of user information
type UserSummary struct {
	ID          uuid.UUID       `json:"id"`
	Email       string          `json:"email"`
	FirstName   string          `json:"first_name"`
	LastName    string          `json:"last_name"`
	Role        models.UserRole `json:"role"`
	Department  string          `json:"department"`
	IsActive    bool            `json:"is_active"`
	LastLoginAt *string         `json:"last_login_at,omitempty"`
	CreatedAt   string          `json:"created_at"`
}

// Handler Methods

// GetSettings retrieves tenant settings
// @Summary Get tenant settings
// @Description Get current tenant's settings and configuration
// @Tags tenant
// @Produce json
// @Success 200 {object} TenantSettingsResponse
// @Failure 401 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Router /tenant/settings [get]
func (h *TenantHandler) GetSettings(c *gin.Context) {
	userCtx := getUserContextFromGin(c)
	if userCtx == nil {
		c.JSON(http.StatusUnauthorized, ErrorResponse{
			Error:   "unauthorized",
			Message: "User context not found",
		})
		return
	}

	// Get tenant information
	tenantInfo, err := h.tenantService.GetTenant(c.Request.Context(), userCtx.TenantID)
	if err != nil {
		c.JSON(http.StatusNotFound, ErrorResponse{
			Error:   "tenant_not_found",
			Message: "Tenant not found",
		})
		return
	}

	c.JSON(http.StatusOK, convertToTenantSettingsResponse(tenantInfo.Tenant))
}

// UpdateSettings updates tenant settings
// @Summary Update tenant settings
// @Description Update tenant settings and configuration (admin only)
// @Tags tenant
// @Accept json
// @Produce json
// @Param request body TenantSettingsRequest true "Tenant settings update request"
// @Success 200 {object} TenantSettingsResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Router /tenant/settings [put]
func (h *TenantHandler) UpdateSettings(c *gin.Context) {
	userCtx := getUserContextFromGin(c)
	if userCtx == nil {
		c.JSON(http.StatusUnauthorized, ErrorResponse{
			Error:   "unauthorized",
			Message: "User context not found",
		})
		return
	}

	var req TenantSettingsRequest
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
		"name":          req.Name,
		"business_type": req.BusinessType,
		"industry":      req.Industry,
		"company_size":  req.CompanySize,
		"tax_id":        req.TaxID,
	}

	if req.Address != nil {
		updates["address"] = req.Address
	}
	if req.Settings != nil {
		updates["settings"] = req.Settings
	}

	// Update tenant
	updatedTenant, err := h.tenantService.UpdateTenant(c.Request.Context(), userCtx.TenantID, updates, userCtx.UserID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "update_failed",
			Message: "Failed to update tenant settings",
			Details: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, convertToTenantSettingsResponse(updatedTenant))
}

// GetUsage retrieves tenant usage statistics
// @Summary Get tenant usage
// @Description Get current tenant's usage statistics and quotas
// @Tags tenant
// @Produce json
// @Success 200 {object} TenantUsageResponse
// @Failure 401 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /tenant/usage [get]
func (h *TenantHandler) GetUsage(c *gin.Context) {
	userCtx := getUserContextFromGin(c)
	if userCtx == nil {
		c.JSON(http.StatusUnauthorized, ErrorResponse{
			Error:   "unauthorized",
			Message: "User context not found",
		})
		return
	}

	// Get usage statistics using the correct method
	usage, err := h.tenantService.GetTenantUsage(c.Request.Context(), userCtx.TenantID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "usage_fetch_failed",
			Message: "Failed to fetch usage statistics",
			Details: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, convertToTenantUsageResponse(usage))
}

// GetTenantUsers lists all users in the tenant
// @Summary List tenant users
// @Description List all users in the current tenant (admin only)
// @Tags tenant
// @Produce json
// @Param page query int false "Page number" default(1)
// @Param per_page query int false "Items per page" default(20)
// @Param search query string false "Search term"
// @Param role query string false "Filter by role"
// @Param active query bool false "Filter by active status"
// @Success 200 {object} TenantUsersResponse
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Router /tenant/users [get]
func (h *TenantHandler) GetTenantUsers(c *gin.Context) {
	userCtx := getUserContextFromGin(c)
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
	role := c.Query("role")
	activeStr := c.Query("active")

	// Create list parameters
	params := repositories.ListParams{
		Page:     page,
		PageSize: perPage,
		Search:   search,
		SortBy:   "created_at",
		SortDesc: true,
	}

	// Get users
	users, _, err := h.userService.ListUsers(c.Request.Context(), userCtx.TenantID, params)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "users_fetch_failed",
			Message: "Failed to fetch tenant users",
			Details: err.Error(),
		})
		return
	}

	// Apply additional filters
	filteredUsers := filterUsers(users, role, activeStr)
	filteredTotal := int64(len(filteredUsers))

	// Apply pagination to filtered results
	startIdx := (page - 1) * perPage
	endIdx := startIdx + perPage
	if startIdx > len(filteredUsers) {
		startIdx = len(filteredUsers)
	}
	if endIdx > len(filteredUsers) {
		endIdx = len(filteredUsers)
	}
	paginatedUsers := filteredUsers[startIdx:endIdx]

	// Convert to response format
	userSummaries := make([]UserSummary, len(paginatedUsers))
	for i, user := range paginatedUsers {
		userSummaries[i] = convertToUserSummary(&user)
	}

	totalPages := int((filteredTotal + int64(perPage) - 1) / int64(perPage))

	response := TenantUsersResponse{
		Users:      userSummaries,
		Total:      filteredTotal,
		Page:       page,
		PerPage:    perPage,
		TotalPages: totalPages,
	}

	c.JSON(http.StatusOK, response)
}

// Helper Methods

// requireAdminMiddleware checks if user has admin privileges
func (h *TenantHandler) requireAdminMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		userCtx := getUserContextFromGin(c)
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

// getUserContextFromGin extracts user context from gin context (renamed to avoid conflict)
func getUserContextFromGin(c *gin.Context) *middleware.UserContext {
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

// filterUsers applies role and active status filters
func filterUsers(users []models.User, role, activeStr string) []models.User {
	filtered := make([]models.User, 0, len(users))

	for _, user := range users {
		// Apply role filter
		if role != "" && string(user.Role) != role {
			continue
		}

		// Apply active status filter
		if activeStr != "" {
			activeFilter := activeStr == "true"
			if user.IsActive != activeFilter {
				continue
			}
		}

		filtered = append(filtered, user)
	}

	return filtered
}

// Conversion functions

// convertToTenantSettingsResponse converts domain model to API response
func convertToTenantSettingsResponse(tenant *models.Tenant) TenantSettingsResponse {
	return TenantSettingsResponse{
		ID:           tenant.ID,
		Name:         tenant.Name,
		Subdomain:    tenant.Subdomain,
		BusinessType: tenant.BusinessType,
		Industry:     tenant.Industry,
		CompanySize:  tenant.CompanySize,
		TaxID:        tenant.TaxID,
		Address:      map[string]interface{}(tenant.Address),
		Settings:     map[string]interface{}(tenant.Settings),
		IsActive:     tenant.IsActive,
		CreatedAt:    tenant.CreatedAt.Format("2006-01-02T15:04:05Z"),
		UpdatedAt:    tenant.UpdatedAt.Format("2006-01-02T15:04:05Z"),
	}
}

// convertToTenantUsageResponse converts usage stats to API response
func convertToTenantUsageResponse(usage *services.TenantUsage) TenantUsageResponse {
	storagePercent := 0.0
	if usage.QuotaStatus != nil && usage.QuotaStatus.StorageQuota > 0 {
		storagePercent = usage.QuotaStatus.StoragePercent
	}

	apiPercent := 0.0
	if usage.QuotaStatus != nil && usage.QuotaStatus.APIQuota > 0 {
		apiPercent = usage.QuotaStatus.APIPercent
	}

	storageUsed := int64(0)
	storageQuota := int64(0)
	apiUsed := 0
	apiQuota := 0

	if usage.QuotaStatus != nil {
		storageUsed = usage.QuotaStatus.StorageUsed
		storageQuota = usage.QuotaStatus.StorageQuota
		apiUsed = usage.QuotaStatus.APIUsed
		apiQuota = usage.QuotaStatus.APIQuota
	}

	return TenantUsageResponse{
		TenantID:       usage.TenantID,
		StorageUsed:    storageUsed,
		StorageQuota:   storageQuota,
		StoragePercent: storagePercent,
		APIUsed:        apiUsed,
		APIQuota:       apiQuota,
		APIPercent:     apiPercent,
		TotalUsers:     usage.TotalUsers,
		TotalDocuments: usage.TotalDocuments,
		LastUpdated:    usage.LastUpdated.Format("2006-01-02T15:04:05Z"),
	}
}

// convertToUserSummary converts user model to summary
func convertToUserSummary(user *models.User) UserSummary {
	var lastLoginAt *string
	if user.LastLoginAt != nil {
		formatted := user.LastLoginAt.Format("2006-01-02T15:04:05Z")
		lastLoginAt = &formatted
	}

	return UserSummary{
		ID:          user.ID,
		Email:       user.Email,
		FirstName:   user.FirstName,
		LastName:    user.LastName,
		Role:        user.Role,
		Department:  user.Department,
		IsActive:    user.IsActive,
		LastLoginAt: lastLoginAt,
		CreatedAt:   user.CreatedAt.Format("2006-01-02T15:04:05Z"),
	}
}
