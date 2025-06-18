package handlers

import (
	"strings"

	"github.com/archivus/archivus/internal/domain/services"
	"github.com/archivus/archivus/internal/infrastructure/database/models"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// CategoryHandler handles category management operations
type CategoryHandler struct {
	*BaseHandler
	documentService *services.DocumentService
	userService     *services.UserService
}

// NewCategoryHandler creates a new category handler
func NewCategoryHandler(
	documentService *services.DocumentService,
	userService *services.UserService,
) *CategoryHandler {
	return &CategoryHandler{
		BaseHandler:     NewBaseHandler(),
		documentService: documentService,
		userService:     userService,
	}
}

// RegisterRoutes sets up the category management routes
func (h *CategoryHandler) RegisterRoutes(router *gin.RouterGroup) {
	categories := router.Group("/categories")
	{
		categories.POST("", h.CreateCategory)
		categories.GET("", h.ListCategories)
		categories.GET("/:id", h.GetCategory)
		categories.PUT("/:id", h.UpdateCategory)
		categories.DELETE("/:id", h.DeleteCategory)
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
	userCtx, ok := h.AuthenticateUser(c)
	if !ok {
		return
	}

	var req CreateCategoryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.RespondBadRequest(c, "Invalid request format", err.Error())
		return
	}

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
		h.handleCategoryError(c, err, req.Name)
		return
	}

	h.RespondCreated(c, h.convertToCategoryResponse(category))
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
	userCtx, ok := h.AuthenticateUser(c)
	if !ok {
		return
	}

	page, pageSize := h.ParsePagination(c)
	includeCounts := c.DefaultQuery("include_counts", "true") == "true"

	if includeCounts {
		categoriesWithCounts, err := h.documentService.ListCategoriesWithDocumentCount(c.Request.Context(), userCtx.TenantID)
		if err != nil {
			h.RespondInternalError(c, "Failed to list categories", err.Error())
			return
		}

		response := h.buildCategoryListResponse(categoriesWithCounts, page, pageSize)
		h.RespondSuccess(c, response)
	} else {
		categories, err := h.documentService.ListCategories(c.Request.Context(), userCtx.TenantID)
		if err != nil {
			h.RespondInternalError(c, "Failed to list categories", err.Error())
			return
		}

		response := h.buildSimpleCategoryListResponse(categories, page, pageSize)
		h.RespondSuccess(c, response)
	}
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
	userCtx, ok := h.AuthenticateUser(c)
	if !ok {
		return
	}

	categoryID, ok := h.ValidateUUID(c, "category ID", c.Param("id"))
	if !ok {
		return
	}

	category, err := h.documentService.GetCategory(c.Request.Context(), categoryID, userCtx.TenantID)
	if err != nil {
		h.RespondNotFound(c, "Category not found")
		return
	}

	h.RespondSuccess(c, h.convertToCategoryResponse(category))
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
	userCtx, ok := h.AuthenticateUser(c)
	if !ok {
		return
	}

	categoryID, ok := h.ValidateUUID(c, "category ID", c.Param("id"))
	if !ok {
		return
	}

	var req UpdateCategoryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.RespondBadRequest(c, "Invalid request format", err.Error())
		return
	}

	updates := h.buildUpdateMap(req)
	category, err := h.documentService.UpdateCategory(c.Request.Context(), categoryID, userCtx.TenantID, updates, userCtx.UserID)
	if err != nil {
		h.handleCategoryUpdateError(c, err, req.Name)
		return
	}

	h.RespondSuccess(c, h.convertToCategoryResponse(category))
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
	userCtx, ok := h.AuthenticateUser(c)
	if !ok {
		return
	}

	categoryID, ok := h.ValidateUUID(c, "category ID", c.Param("id"))
	if !ok {
		return
	}

	err := h.documentService.DeleteCategory(c.Request.Context(), categoryID, userCtx.TenantID, userCtx.UserID)
	if err != nil {
		h.handleCategoryDeleteError(c, err)
		return
	}

	h.RespondSuccess(c, SuccessResponse{Message: "Category deleted successfully"})
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
	userCtx, ok := h.AuthenticateUser(c)
	if !ok {
		return
	}

	categories, err := h.documentService.GetSystemCategories(c.Request.Context(), userCtx.TenantID)
	if err != nil {
		h.RespondInternalError(c, "Failed to fetch system categories", err.Error())
		return
	}

	var responses []CategoryResponse
	for _, category := range categories {
		responses = append(responses, h.convertToCategoryResponse(&category))
	}

	h.RespondSuccess(c, SystemCategoriesResponse{
		Categories: responses,
		Count:      len(responses),
	})
}

// Helper Methods

func (h *CategoryHandler) handleCategoryError(c *gin.Context, err error, name string) {
	if strings.Contains(err.Error(), "already exists") {
		h.RespondConflict(c, "A category with this name already exists")
		return
	}
	h.RespondInternalError(c, "Failed to create category", err.Error())
}

func (h *CategoryHandler) handleCategoryUpdateError(c *gin.Context, err error, name *string) {
	if strings.Contains(err.Error(), "not found") {
		h.RespondNotFound(c, "Category not found")
		return
	}
	if name != nil && strings.Contains(err.Error(), "already exists") {
		h.RespondConflict(c, "A category with this name already exists")
		return
	}
	h.RespondInternalError(c, "Failed to update category", err.Error())
}

func (h *CategoryHandler) handleCategoryDeleteError(c *gin.Context, err error) {
	if strings.Contains(err.Error(), "not found") {
		h.RespondNotFound(c, "Category not found")
		return
	}
	if strings.Contains(err.Error(), "system category") {
		h.RespondError(c, 403, "forbidden", "Cannot delete system category")
		return
	}
	h.RespondInternalError(c, "Failed to delete category", err.Error())
}

func (h *CategoryHandler) buildUpdateMap(req UpdateCategoryRequest) map[string]interface{} {
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
	return updates
}

func (h *CategoryHandler) buildCategoryListResponse(categoriesWithCounts []services.CategoryWithCount, page, pageSize int) CategoryListResponse {
	var responses []CategoryWithCountResponse
	for _, categoryWithCount := range categoriesWithCounts {
		responses = append(responses, h.convertToCategoryWithCountResponse(&categoryWithCount))
	}

	return h.paginateCategoryResponses(responses, page, pageSize)
}

func (h *CategoryHandler) buildSimpleCategoryListResponse(categories []models.Category, page, pageSize int) CategoryListResponse {
	var responses []CategoryWithCountResponse
	for _, category := range categories {
		response := CategoryWithCountResponse{
			CategoryResponse: h.convertToCategoryResponse(&category),
			DocumentCount:    0, // No count requested
		}
		responses = append(responses, response)
	}

	return h.paginateCategoryResponses(responses, page, pageSize)
}

func (h *CategoryHandler) paginateCategoryResponses(responses []CategoryWithCountResponse, page, pageSize int) CategoryListResponse {
	total := len(responses)
	startIdx := (page - 1) * pageSize
	endIdx := startIdx + pageSize
	if startIdx > total {
		startIdx = total
	}
	if endIdx > total {
		endIdx = total
	}

	paginatedCategories := responses[startIdx:endIdx]
	totalPages := (total + pageSize - 1) / pageSize

	return CategoryListResponse{
		Categories: paginatedCategories,
		Total:      total,
		Page:       page,
		PerPage:    pageSize,
		TotalPages: totalPages,
	}
}

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

func (h *CategoryHandler) convertToCategoryWithCountResponse(categoryWithCount *services.CategoryWithCount) CategoryWithCountResponse {
	return CategoryWithCountResponse{
		CategoryResponse: h.convertToCategoryResponse(&categoryWithCount.Category),
		DocumentCount:    categoryWithCount.DocumentCount,
	}
}
