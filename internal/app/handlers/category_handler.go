package handlers

import (
	"net/http"
	"strconv"

	"github.com/archivus/archivus/internal/domain/services"
	"github.com/archivus/archivus/internal/infrastructure/database/models"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// CategoryHandler handles category management operations
type CategoryHandler struct {
	documentService *services.DocumentService
	userService     *services.UserService
}

// NewCategoryHandler creates a new category handler
func NewCategoryHandler(
	documentService *services.DocumentService,
	userService *services.UserService,
) *CategoryHandler {
	return &CategoryHandler{
		documentService: documentService,
		userService:     userService,
	}
}

// RegisterRoutes sets up the category management routes
func (h *CategoryHandler) RegisterRoutes(router *gin.RouterGroup) {
	categories := router.Group("/categories")
	// Note: Auth middleware should be applied at server level
	{
		// CRUD operations
		categories.POST("", h.CreateCategory)
		categories.GET("", h.ListCategories)
		categories.GET("/:id", h.GetCategory)
		categories.PUT("/:id", h.UpdateCategory)
		categories.DELETE("/:id", h.DeleteCategory)

		// Special operations
		categories.GET("/system", h.GetSystemCategories)
	}
}

// Request/Response DTOs

// CreateCategoryRequest contains category creation data
type CreateCategoryRequest struct {
	Name        string `json:"name" binding:"required,min=1,max=100"`
	Description string `json:"description,omitempty"`
	Color       string `json:"color,omitempty" binding:"omitempty,len=7"`
	Icon        string `json:"icon,omitempty" binding:"omitempty,max=50"`
	SortOrder   int    `json:"sort_order,omitempty" binding:"omitempty,min=0"`
}

// UpdateCategoryRequest contains category update data
type UpdateCategoryRequest struct {
	Name        *string `json:"name,omitempty" binding:"omitempty,min=1,max=100"`
	Description *string `json:"description,omitempty"`
	Color       *string `json:"color,omitempty" binding:"omitempty,len=7"`
	Icon        *string `json:"icon,omitempty" binding:"omitempty,max=50"`
	SortOrder   *int    `json:"sort_order,omitempty" binding:"omitempty,min=0"`
}

// CategoryResponse represents category data in API responses
type CategoryResponse struct {
	ID          uuid.UUID `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Color       string    `json:"color"`
	Icon        string    `json:"icon"`
	IsSystem    bool      `json:"is_system"`
	SortOrder   int       `json:"sort_order"`
	CreatedAt   string    `json:"created_at"`
}

// CategoryWithCountResponse represents category with document count
type CategoryWithCountResponse struct {
	CategoryResponse
	DocumentCount int `json:"document_count"`
}

// CategoryListResponse represents paginated category list
type CategoryListResponse struct {
	Categories []CategoryWithCountResponse `json:"categories"`
	Total      int                         `json:"total"`
	Page       int                         `json:"page"`
	PerPage    int                         `json:"per_page"`
	TotalPages int                         `json:"total_pages"`
}

// SystemCategoriesResponse represents system categories list
type SystemCategoriesResponse struct {
	Categories []CategoryResponse `json:"categories"`
	Count      int                `json:"count"`
}

// Handler Methods

// CreateCategory creates a new category
// @Summary Create category
// @Description Create a new category for document classification
// @Tags categories
// @Accept json
// @Produce json
// @Param request body CreateCategoryRequest true "Category creation request"
// @Success 201 {object} CategoryResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 409 {object} ErrorResponse
// @Router /categories [post]
func (h *CategoryHandler) CreateCategory(c *gin.Context) {
	userCtx := getUserContextFromGin(c)
	if userCtx == nil {
		c.JSON(http.StatusUnauthorized, ErrorResponse{
			Error:   "unauthorized",
			Message: "User context not found",
		})
		return
	}

	var req CreateCategoryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "invalid_request",
			Message: "Invalid request format",
			Details: err.Error(),
		})
		return
	}

	// Create category
	category, err := h.documentService.CreateCategory(
		c.Request.Context(),
		userCtx.TenantID,
		userCtx.UserID,
		req.Name,
		req.Description,
		req.Color,
		req.Icon,
		req.SortOrder,
	)
	if err != nil {
		if err.Error() == "category with name '"+req.Name+"' already exists" {
			c.JSON(http.StatusConflict, ErrorResponse{
				Error:   "category_exists",
				Message: "A category with this name already exists",
			})
			return
		}

		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "create_failed",
			Message: "Failed to create category",
			Details: err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, h.convertToCategoryResponse(category))
}

// ListCategories lists all categories for the tenant with document counts
// @Summary List categories
// @Description List all categories with document counts and usage statistics
// @Tags categories
// @Produce json
// @Param page query int false "Page number" default(1)
// @Param per_page query int false "Items per page" default(20)
// @Param include_counts query bool false "Include document counts" default(true)
// @Success 200 {object} CategoryListResponse
// @Failure 401 {object} ErrorResponse
// @Router /categories [get]
func (h *CategoryHandler) ListCategories(c *gin.Context) {
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
	includeCounts := c.DefaultQuery("include_counts", "true") == "true"

	if page < 1 {
		page = 1
	}
	if perPage < 1 || perPage > 100 {
		perPage = 20
	}

	var categoryResponses []CategoryWithCountResponse

	if includeCounts {
		// Get categories with document counts
		categoriesWithCounts, err := h.documentService.ListCategoriesWithDocumentCount(c.Request.Context(), userCtx.TenantID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, ErrorResponse{
				Error:   "list_failed",
				Message: "Failed to list categories",
				Details: err.Error(),
			})
			return
		}

		// Convert to response format
		for _, categoryWithCount := range categoriesWithCounts {
			categoryResponses = append(categoryResponses, h.convertToCategoryWithCountResponse(&categoryWithCount))
		}
	} else {
		// Get categories without counts
		categories, err := h.documentService.ListCategories(c.Request.Context(), userCtx.TenantID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, ErrorResponse{
				Error:   "list_failed",
				Message: "Failed to list categories",
				Details: err.Error(),
			})
			return
		}

		// Convert to response format without counts
		for _, category := range categories {
			categoryResponses = append(categoryResponses, CategoryWithCountResponse{
				CategoryResponse: h.convertToCategoryResponse(&category),
				DocumentCount:    0,
			})
		}
	}

	// Apply pagination
	total := len(categoryResponses)
	startIdx := (page - 1) * perPage
	endIdx := startIdx + perPage
	if startIdx > total {
		startIdx = total
	}
	if endIdx > total {
		endIdx = total
	}

	paginatedCategories := categoryResponses[startIdx:endIdx]
	totalPages := (total + perPage - 1) / perPage

	response := CategoryListResponse{
		Categories: paginatedCategories,
		Total:      total,
		Page:       page,
		PerPage:    perPage,
		TotalPages: totalPages,
	}

	c.JSON(http.StatusOK, response)
}

// GetCategory retrieves a specific category
// @Summary Get category details
// @Description Get detailed information about a specific category
// @Tags categories
// @Produce json
// @Param id path string true "Category ID"
// @Success 200 {object} CategoryResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Router /categories/{id} [get]
func (h *CategoryHandler) GetCategory(c *gin.Context) {
	userCtx := getUserContextFromGin(c)
	if userCtx == nil {
		c.JSON(http.StatusUnauthorized, ErrorResponse{
			Error:   "unauthorized",
			Message: "User context not found",
		})
		return
	}

	categoryID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "invalid_category_id",
			Message: "Invalid category ID format",
		})
		return
	}

	// Get category
	category, err := h.documentService.GetCategory(c.Request.Context(), categoryID, userCtx.TenantID)
	if err != nil {
		c.JSON(http.StatusNotFound, ErrorResponse{
			Error:   "category_not_found",
			Message: "Category not found",
		})
		return
	}

	c.JSON(http.StatusOK, h.convertToCategoryResponse(category))
}

// UpdateCategory updates an existing category
// @Summary Update category
// @Description Update category information (name, description, color, icon, sort order)
// @Tags categories
// @Accept json
// @Produce json
// @Param id path string true "Category ID"
// @Param request body UpdateCategoryRequest true "Category update request"
// @Success 200 {object} CategoryResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 409 {object} ErrorResponse
// @Router /categories/{id} [put]
func (h *CategoryHandler) UpdateCategory(c *gin.Context) {
	userCtx := getUserContextFromGin(c)
	if userCtx == nil {
		c.JSON(http.StatusUnauthorized, ErrorResponse{
			Error:   "unauthorized",
			Message: "User context not found",
		})
		return
	}

	categoryID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "invalid_category_id",
			Message: "Invalid category ID format",
		})
		return
	}

	var req UpdateCategoryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "invalid_request",
			Message: "Invalid request format",
			Details: err.Error(),
		})
		return
	}

	// Convert request to updates map
	updates := make(map[string]interface{})
	if req.Name != nil {
		updates["name"] = *req.Name
	}
	if req.Description != nil {
		updates["description"] = *req.Description
	}
	if req.Color != nil {
		updates["color"] = *req.Color
	}
	if req.Icon != nil {
		updates["icon"] = *req.Icon
	}
	if req.SortOrder != nil {
		updates["sort_order"] = *req.SortOrder
	}

	// Update category
	category, err := h.documentService.UpdateCategory(c.Request.Context(), categoryID, userCtx.TenantID, updates, userCtx.UserID)
	if err != nil {
		if err.Error() == "category not found" {
			c.JSON(http.StatusNotFound, ErrorResponse{
				Error:   "category_not_found",
				Message: "Category not found",
			})
			return
		}
		if err.Error() == "cannot modify system category" {
			c.JSON(http.StatusForbidden, ErrorResponse{
				Error:   "system_category",
				Message: "Cannot modify system category",
			})
			return
		}
		if req.Name != nil && err.Error() == "category with name '"+*req.Name+"' already exists" {
			c.JSON(http.StatusConflict, ErrorResponse{
				Error:   "category_exists",
				Message: "A category with this name already exists",
			})
			return
		}

		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "update_failed",
			Message: "Failed to update category",
			Details: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, h.convertToCategoryResponse(category))
}

// DeleteCategory deletes a category
// @Summary Delete category
// @Description Delete a category (removes it from all documents)
// @Tags categories
// @Param id path string true "Category ID"
// @Success 200 {object} SuccessResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Router /categories/{id} [delete]
func (h *CategoryHandler) DeleteCategory(c *gin.Context) {
	userCtx := getUserContextFromGin(c)
	if userCtx == nil {
		c.JSON(http.StatusUnauthorized, ErrorResponse{
			Error:   "unauthorized",
			Message: "User context not found",
		})
		return
	}

	categoryID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "invalid_category_id",
			Message: "Invalid category ID format",
		})
		return
	}

	// Delete category
	err = h.documentService.DeleteCategory(c.Request.Context(), categoryID, userCtx.TenantID, userCtx.UserID)
	if err != nil {
		if err.Error() == "category not found" {
			c.JSON(http.StatusNotFound, ErrorResponse{
				Error:   "category_not_found",
				Message: "Category not found",
			})
			return
		}
		if err.Error() == "cannot delete system category" {
			c.JSON(http.StatusForbidden, ErrorResponse{
				Error:   "system_category",
				Message: "Cannot delete system category",
			})
			return
		}

		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "delete_failed",
			Message: "Failed to delete category",
			Details: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, SuccessResponse{
		Message: "Category deleted successfully",
	})
}

// GetSystemCategories gets system-defined categories
// @Summary Get system categories
// @Description Get all system-defined categories for the tenant
// @Tags categories
// @Produce json
// @Success 200 {object} SystemCategoriesResponse
// @Failure 401 {object} ErrorResponse
// @Router /categories/system [get]
func (h *CategoryHandler) GetSystemCategories(c *gin.Context) {
	userCtx := getUserContextFromGin(c)
	if userCtx == nil {
		c.JSON(http.StatusUnauthorized, ErrorResponse{
			Error:   "unauthorized",
			Message: "User context not found",
		})
		return
	}

	// Get system categories
	categories, err := h.documentService.GetSystemCategories(c.Request.Context(), userCtx.TenantID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "fetch_failed",
			Message: "Failed to fetch system categories",
			Details: err.Error(),
		})
		return
	}

	// Convert to response format
	var categoryResponses []CategoryResponse
	for _, category := range categories {
		categoryResponses = append(categoryResponses, h.convertToCategoryResponse(&category))
	}

	response := SystemCategoriesResponse{
		Categories: categoryResponses,
		Count:      len(categoryResponses),
	}

	c.JSON(http.StatusOK, response)
}

// Helper Methods

// convertToCategoryResponse converts domain model to API response
func (h *CategoryHandler) convertToCategoryResponse(category *models.Category) CategoryResponse {
	return CategoryResponse{
		ID:          category.ID,
		Name:        category.Name,
		Description: category.Description,
		Color:       category.Color,
		Icon:        category.Icon,
		IsSystem:    category.IsSystem,
		SortOrder:   category.SortOrder,
		CreatedAt:   category.CreatedAt.Format("2006-01-02T15:04:05Z"),
	}
}

// convertToCategoryWithCountResponse converts category with count to API response
func (h *CategoryHandler) convertToCategoryWithCountResponse(categoryWithCount *services.CategoryWithCount) CategoryWithCountResponse {
	return CategoryWithCountResponse{
		CategoryResponse: h.convertToCategoryResponse(&categoryWithCount.Category),
		DocumentCount:    categoryWithCount.DocumentCount,
	}
}
