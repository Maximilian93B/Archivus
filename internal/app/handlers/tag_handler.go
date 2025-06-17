package handlers

import (
	"net/http"
	"strconv"

	"github.com/archivus/archivus/internal/domain/services"
	"github.com/archivus/archivus/internal/infrastructure/database/models"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// TagHandler handles tag management operations
type TagHandler struct {
	documentService *services.DocumentService
	userService     *services.UserService
}

// NewTagHandler creates a new tag handler
func NewTagHandler(
	documentService *services.DocumentService,
	userService *services.UserService,
) *TagHandler {
	return &TagHandler{
		documentService: documentService,
		userService:     userService,
	}
}

// RegisterRoutes sets up the tag management routes
func (h *TagHandler) RegisterRoutes(router *gin.RouterGroup) {
	tags := router.Group("/tags")
	// Note: Auth middleware should be applied at server level
	{
		// CRUD operations
		tags.POST("", h.CreateTag)
		tags.GET("", h.ListTags)
		tags.GET("/:id", h.GetTag)
		tags.PUT("/:id", h.UpdateTag)
		tags.DELETE("/:id", h.DeleteTag)

		// Special operations
		tags.GET("/popular", h.GetPopularTags)
		tags.GET("/suggestions", h.GetTagSuggestions)
	}
}

// Request/Response DTOs

// CreateTagRequest contains tag creation data
type CreateTagRequest struct {
	Name  string `json:"name" binding:"required,min=1,max=50"`
	Color string `json:"color,omitempty" binding:"omitempty,len=7"`
}

// UpdateTagRequest contains tag update data
type UpdateTagRequest struct {
	Name  *string `json:"name,omitempty" binding:"omitempty,min=1,max=50"`
	Color *string `json:"color,omitempty" binding:"omitempty,len=7"`
}

// TagResponse represents tag data in API responses
type TagResponse struct {
	ID            uuid.UUID `json:"id"`
	Name          string    `json:"name"`
	Color         string    `json:"color"`
	IsAIGenerated bool      `json:"is_ai_generated"`
	UsageCount    int       `json:"usage_count"`
	CreatedAt     string    `json:"created_at"`
}

// TagSuggestionsRequest contains tag suggestion parameters
type TagSuggestionsRequest struct {
	Text  string `json:"text" form:"text" binding:"required,min=1"`
	Limit int    `json:"limit" form:"limit" binding:"omitempty,min=1,max=50"`
}

// TagSuggestionsResponse represents tag suggestions
type TagSuggestionsResponse struct {
	Suggestions []string `json:"suggestions"`
	Count       int      `json:"count"`
}

// TagListResponse represents paginated tag list
type TagListResponse struct {
	Tags       []TagResponse `json:"tags"`
	Total      int           `json:"total"`
	Page       int           `json:"page"`
	PerPage    int           `json:"per_page"`
	TotalPages int           `json:"total_pages"`
}

// PopularTagsResponse represents popular tags list
type PopularTagsResponse struct {
	Tags  []TagResponse `json:"tags"`
	Count int           `json:"count"`
}

// Handler Methods

// CreateTag creates a new tag
// @Summary Create tag
// @Description Create a new tag for document labeling
// @Tags tags
// @Accept json
// @Produce json
// @Param request body CreateTagRequest true "Tag creation request"
// @Success 201 {object} TagResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 409 {object} ErrorResponse
// @Router /tags [post]
func (h *TagHandler) CreateTag(c *gin.Context) {
	userCtx := getUserContextFromGin(c)
	if userCtx == nil {
		c.JSON(http.StatusUnauthorized, ErrorResponse{
			Error:   "unauthorized",
			Message: "User context not found",
		})
		return
	}

	var req CreateTagRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "invalid_request",
			Message: "Invalid request format",
			Details: err.Error(),
		})
		return
	}

	// Create tag
	tag, err := h.documentService.CreateTag(c.Request.Context(), userCtx.TenantID, userCtx.UserID, req.Name, req.Color)
	if err != nil {
		if err.Error() == "tag with name '"+req.Name+"' already exists" {
			c.JSON(http.StatusConflict, ErrorResponse{
				Error:   "tag_exists",
				Message: "A tag with this name already exists",
			})
			return
		}

		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "create_failed",
			Message: "Failed to create tag",
			Details: err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, h.convertToTagResponse(tag))
}

// ListTags lists all tags for the tenant
// @Summary List tags
// @Description List all tags with usage statistics
// @Tags tags
// @Produce json
// @Param page query int false "Page number" default(1)
// @Param per_page query int false "Items per page" default(20)
// @Param sort_by query string false "Sort field (name, usage_count, created_at)" default("usage_count")
// @Param sort_desc query bool false "Sort descending" default(true)
// @Success 200 {object} TagListResponse
// @Failure 401 {object} ErrorResponse
// @Router /tags [get]
func (h *TagHandler) ListTags(c *gin.Context) {
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

	sortBy := c.DefaultQuery("sort_by", "usage_count")
	sortDesc := c.DefaultQuery("sort_desc", "true") == "true"

	// Get tags from service
	tags, err := h.documentService.ListTags(c.Request.Context(), userCtx.TenantID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "list_failed",
			Message: "Failed to list tags",
			Details: err.Error(),
		})
		return
	}

	// Sort tags (simple in-memory sorting)
	h.sortTags(tags, sortBy, sortDesc)

	// Convert to response format
	var tagResponses []TagResponse
	for _, tag := range tags {
		tagResponses = append(tagResponses, h.convertToTagResponse(&tag))
	}

	// Apply pagination
	total := len(tagResponses)
	startIdx := (page - 1) * perPage
	endIdx := startIdx + perPage
	if startIdx > total {
		startIdx = total
	}
	if endIdx > total {
		endIdx = total
	}

	paginatedTags := tagResponses[startIdx:endIdx]
	totalPages := (total + perPage - 1) / perPage

	response := TagListResponse{
		Tags:       paginatedTags,
		Total:      total,
		Page:       page,
		PerPage:    perPage,
		TotalPages: totalPages,
	}

	c.JSON(http.StatusOK, response)
}

// GetTag retrieves a specific tag
// @Summary Get tag details
// @Description Get detailed information about a specific tag
// @Tags tags
// @Produce json
// @Param id path string true "Tag ID"
// @Success 200 {object} TagResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Router /tags/{id} [get]
func (h *TagHandler) GetTag(c *gin.Context) {
	userCtx := getUserContextFromGin(c)
	if userCtx == nil {
		c.JSON(http.StatusUnauthorized, ErrorResponse{
			Error:   "unauthorized",
			Message: "User context not found",
		})
		return
	}

	tagID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "invalid_tag_id",
			Message: "Invalid tag ID format",
		})
		return
	}

	// Get tag
	tag, err := h.documentService.GetTag(c.Request.Context(), tagID, userCtx.TenantID)
	if err != nil {
		c.JSON(http.StatusNotFound, ErrorResponse{
			Error:   "tag_not_found",
			Message: "Tag not found",
		})
		return
	}

	c.JSON(http.StatusOK, h.convertToTagResponse(tag))
}

// UpdateTag updates an existing tag
// @Summary Update tag
// @Description Update tag information (name, color)
// @Tags tags
// @Accept json
// @Produce json
// @Param id path string true "Tag ID"
// @Param request body UpdateTagRequest true "Tag update request"
// @Success 200 {object} TagResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 409 {object} ErrorResponse
// @Router /tags/{id} [put]
func (h *TagHandler) UpdateTag(c *gin.Context) {
	userCtx := getUserContextFromGin(c)
	if userCtx == nil {
		c.JSON(http.StatusUnauthorized, ErrorResponse{
			Error:   "unauthorized",
			Message: "User context not found",
		})
		return
	}

	tagID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "invalid_tag_id",
			Message: "Invalid tag ID format",
		})
		return
	}

	var req UpdateTagRequest
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
	if req.Color != nil {
		updates["color"] = *req.Color
	}

	// Update tag
	tag, err := h.documentService.UpdateTag(c.Request.Context(), tagID, userCtx.TenantID, updates, userCtx.UserID)
	if err != nil {
		if err.Error() == "tag not found" {
			c.JSON(http.StatusNotFound, ErrorResponse{
				Error:   "tag_not_found",
				Message: "Tag not found",
			})
			return
		}
		if err.Error() == "tag with name '"+*req.Name+"' already exists" {
			c.JSON(http.StatusConflict, ErrorResponse{
				Error:   "tag_exists",
				Message: "A tag with this name already exists",
			})
			return
		}

		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "update_failed",
			Message: "Failed to update tag",
			Details: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, h.convertToTagResponse(tag))
}

// DeleteTag deletes a tag
// @Summary Delete tag
// @Description Delete a tag (removes it from all documents)
// @Tags tags
// @Param id path string true "Tag ID"
// @Success 200 {object} SuccessResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Router /tags/{id} [delete]
func (h *TagHandler) DeleteTag(c *gin.Context) {
	userCtx := getUserContextFromGin(c)
	if userCtx == nil {
		c.JSON(http.StatusUnauthorized, ErrorResponse{
			Error:   "unauthorized",
			Message: "User context not found",
		})
		return
	}

	tagID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "invalid_tag_id",
			Message: "Invalid tag ID format",
		})
		return
	}

	// Delete tag
	err = h.documentService.DeleteTag(c.Request.Context(), tagID, userCtx.TenantID, userCtx.UserID)
	if err != nil {
		if err.Error() == "tag not found" {
			c.JSON(http.StatusNotFound, ErrorResponse{
				Error:   "tag_not_found",
				Message: "Tag not found",
			})
			return
		}

		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "delete_failed",
			Message: "Failed to delete tag",
			Details: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, SuccessResponse{
		Message: "Tag deleted successfully",
	})
}

// GetPopularTags gets the most popular tags
// @Summary Get popular tags
// @Description Get the most frequently used tags in the tenant
// @Tags tags
// @Produce json
// @Param limit query int false "Number of tags to return" default(20)
// @Success 200 {object} PopularTagsResponse
// @Failure 401 {object} ErrorResponse
// @Router /tags/popular [get]
func (h *TagHandler) GetPopularTags(c *gin.Context) {
	userCtx := getUserContextFromGin(c)
	if userCtx == nil {
		c.JSON(http.StatusUnauthorized, ErrorResponse{
			Error:   "unauthorized",
			Message: "User context not found",
		})
		return
	}

	// Parse limit parameter
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	if limit < 1 || limit > 100 {
		limit = 20
	}

	// Get popular tags
	tags, err := h.documentService.GetPopularTags(c.Request.Context(), userCtx.TenantID, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "fetch_failed",
			Message: "Failed to fetch popular tags",
			Details: err.Error(),
		})
		return
	}

	// Convert to response format
	var tagResponses []TagResponse
	for _, tag := range tags {
		tagResponses = append(tagResponses, h.convertToTagResponse(&tag))
	}

	response := PopularTagsResponse{
		Tags:  tagResponses,
		Count: len(tagResponses),
	}

	c.JSON(http.StatusOK, response)
}

// GetTagSuggestions gets AI-powered tag suggestions
// @Summary Get tag suggestions
// @Description Get intelligent tag suggestions based on provided text
// @Tags tags
// @Produce json
// @Param text query string true "Text to analyze for tag suggestions"
// @Param limit query int false "Maximum number of suggestions" default(10)
// @Success 200 {object} TagSuggestionsResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Router /tags/suggestions [get]
func (h *TagHandler) GetTagSuggestions(c *gin.Context) {
	userCtx := getUserContextFromGin(c)
	if userCtx == nil {
		c.JSON(http.StatusUnauthorized, ErrorResponse{
			Error:   "unauthorized",
			Message: "User context not found",
		})
		return
	}

	// Parse query parameters
	text := c.Query("text")
	if text == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "missing_text",
			Message: "Text parameter is required",
		})
		return
	}

	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	if limit < 1 || limit > 50 {
		limit = 10
	}

	// Get tag suggestions
	suggestions, err := h.documentService.GetTagSuggestions(c.Request.Context(), userCtx.TenantID, text, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "suggestions_failed",
			Message: "Failed to generate tag suggestions",
			Details: err.Error(),
		})
		return
	}

	response := TagSuggestionsResponse{
		Suggestions: suggestions,
		Count:       len(suggestions),
	}

	c.JSON(http.StatusOK, response)
}

// Helper Methods

// convertToTagResponse converts domain model to API response
func (h *TagHandler) convertToTagResponse(tag *models.Tag) TagResponse {
	return TagResponse{
		ID:            tag.ID,
		Name:          tag.Name,
		Color:         tag.Color,
		IsAIGenerated: tag.IsAIGenerated,
		UsageCount:    tag.UsageCount,
		CreatedAt:     tag.CreatedAt.Format("2006-01-02T15:04:05Z"),
	}
}

// sortTags sorts tags by the specified field and order
func (h *TagHandler) sortTags(tags []models.Tag, sortBy string, sortDesc bool) {
	// Simple bubble sort implementation for small datasets
	n := len(tags)
	for i := 0; i < n-1; i++ {
		for j := 0; j < n-i-1; j++ {
			var shouldSwap bool

			switch sortBy {
			case "name":
				if sortDesc {
					shouldSwap = tags[j].Name < tags[j+1].Name
				} else {
					shouldSwap = tags[j].Name > tags[j+1].Name
				}
			case "usage_count":
				if sortDesc {
					shouldSwap = tags[j].UsageCount < tags[j+1].UsageCount
				} else {
					shouldSwap = tags[j].UsageCount > tags[j+1].UsageCount
				}
			case "created_at":
				if sortDesc {
					shouldSwap = tags[j].CreatedAt.Before(tags[j+1].CreatedAt)
				} else {
					shouldSwap = tags[j].CreatedAt.After(tags[j+1].CreatedAt)
				}
			default: // default to usage_count
				if sortDesc {
					shouldSwap = tags[j].UsageCount < tags[j+1].UsageCount
				} else {
					shouldSwap = tags[j].UsageCount > tags[j+1].UsageCount
				}
			}

			if shouldSwap {
				tags[j], tags[j+1] = tags[j+1], tags[j]
			}
		}
	}
}
