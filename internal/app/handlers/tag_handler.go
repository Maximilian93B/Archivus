package handlers

import (
	"github.com/archivus/archivus/internal/domain/services"
	"github.com/archivus/archivus/internal/infrastructure/database/models"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// TagHandler handles tag management operations
type TagHandler struct {
	*BaseHandler
	documentService *services.DocumentService
	userService     *services.UserService
}

// NewTagHandler creates a new tag handler
func NewTagHandler(
	documentService *services.DocumentService,
	userService *services.UserService,
) *TagHandler {
	return &TagHandler{
		BaseHandler:     NewBaseHandler(),
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
	userCtx, ok := h.AuthenticateUser(c)
	if !ok {
		return
	}

	var req CreateTagRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.RespondBadRequest(c, "Invalid request format", err.Error())
		return
	}

	tag, err := h.documentService.CreateTag(c.Request.Context(), userCtx.TenantID, userCtx.UserID, req.Name, req.Color)
	if err != nil {
		h.handleTagError(c, err, req.Name)
		return
	}

	h.RespondCreated(c, h.convertToTagResponse(tag))
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
	userCtx, ok := h.AuthenticateUser(c)
	if !ok {
		return
	}

	page, pageSize := h.ParsePagination(c)
	sortBy, sortDesc := h.ParseSorting(c, "usage_count")

	// Get tags from service
	tags, err := h.documentService.ListTags(c.Request.Context(), userCtx.TenantID)
	if err != nil {
		h.RespondInternalError(c, "Failed to list tags", err.Error())
		return
	}

	// Sort tags (will be moved to service layer later)
	h.sortTags(tags, sortBy, sortDesc)

	response := h.buildTagListResponse(tags, page, pageSize)
	h.RespondSuccess(c, response)
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
	userCtx, ok := h.AuthenticateUser(c)
	if !ok {
		return
	}

	tagID, ok := h.ValidateUUID(c, "tag ID", c.Param("id"))
	if !ok {
		return
	}

	// Get tag
	tag, err := h.documentService.GetTag(c.Request.Context(), tagID, userCtx.TenantID)
	if err != nil {
		h.RespondNotFound(c, "Tag not found")
		return
	}

	h.RespondSuccess(c, h.convertToTagResponse(tag))
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
	userCtx, ok := h.AuthenticateUser(c)
	if !ok {
		return
	}

	tagID, ok := h.ValidateUUID(c, "tag ID", c.Param("id"))
	if !ok {
		return
	}

	var req UpdateTagRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.RespondBadRequest(c, "Invalid request format", err.Error())
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
			h.RespondNotFound(c, "Tag not found")
			return
		}
		if req.Name != nil && err.Error() == "tag with name '"+*req.Name+"' already exists" {
			h.RespondConflict(c, "A tag with this name already exists")
			return
		}
		h.RespondInternalError(c, "Failed to update tag", err.Error())
		return
	}

	h.RespondSuccess(c, h.convertToTagResponse(tag))
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
	userCtx, ok := h.AuthenticateUser(c)
	if !ok {
		return
	}

	tagID, ok := h.ValidateUUID(c, "tag ID", c.Param("id"))
	if !ok {
		return
	}

	// Delete tag
	err := h.documentService.DeleteTag(c.Request.Context(), tagID, userCtx.TenantID, userCtx.UserID)
	if err != nil {
		if err.Error() == "tag not found" {
			h.RespondNotFound(c, "Tag not found")
			return
		}
		h.RespondInternalError(c, "Failed to delete tag", err.Error())
		return
	}

	h.RespondSuccess(c, SuccessResponse{
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
	userCtx, ok := h.AuthenticateUser(c)
	if !ok {
		return
	}

	// Parse limit parameter
	limit := getIntParam(c, "limit", 20)
	if limit > 100 {
		limit = 100
	}

	// Get popular tags
	tags, err := h.documentService.GetPopularTags(c.Request.Context(), userCtx.TenantID, limit)
	if err != nil {
		h.RespondInternalError(c, "Failed to fetch popular tags", err.Error())
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

	h.RespondSuccess(c, response)
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
	userCtx, ok := h.AuthenticateUser(c)
	if !ok {
		return
	}

	// Parse query parameters
	text := c.Query("text")
	if text == "" {
		h.RespondBadRequest(c, "Text parameter is required")
		return
	}

	limit := getIntParam(c, "limit", 10)
	if limit > 50 {
		limit = 50
	}

	// Get tag suggestions
	suggestions, err := h.documentService.GetTagSuggestions(c.Request.Context(), userCtx.TenantID, text, limit)
	if err != nil {
		h.RespondInternalError(c, "Failed to generate tag suggestions", err.Error())
		return
	}

	response := TagSuggestionsResponse{
		Suggestions: suggestions,
		Count:       len(suggestions),
	}

	h.RespondSuccess(c, response)
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

// handleTagError handles tag-specific errors with appropriate responses
func (h *TagHandler) handleTagError(c *gin.Context, err error, tagName string) {
	if err.Error() == "tag with name '"+tagName+"' already exists" {
		h.RespondConflict(c, "A tag with this name already exists")
		return
	}
	h.RespondInternalError(c, "Failed to create tag", err.Error())
}

// buildTagListResponse builds a paginated tag list response
func (h *TagHandler) buildTagListResponse(tags []models.Tag, page, pageSize int) TagListResponse {
	// Convert to response format
	var tagResponses []TagResponse
	for _, tag := range tags {
		tagResponses = append(tagResponses, h.convertToTagResponse(&tag))
	}

	// Apply pagination
	total := len(tagResponses)
	startIdx := (page - 1) * pageSize
	endIdx := startIdx + pageSize
	if startIdx > total {
		startIdx = total
	}
	if endIdx > total {
		endIdx = total
	}

	paginatedTags := tagResponses[startIdx:endIdx]
	totalPages := (total + pageSize - 1) / pageSize

	return TagListResponse{
		Tags:       paginatedTags,
		Total:      total,
		Page:       page,
		PerPage:    pageSize,
		TotalPages: totalPages,
	}
}
