package handlers

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/archivus/archivus/internal/domain/repositories"
	"github.com/archivus/archivus/internal/domain/services"
	"github.com/archivus/archivus/internal/infrastructure/database/models"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// FolderHandler handles folder management operations
type FolderHandler struct {
	documentService *services.DocumentService
	userService     *services.UserService
}

// NewFolderHandler creates a new folder handler
func NewFolderHandler(
	documentService *services.DocumentService,
	userService *services.UserService,
) *FolderHandler {
	return &FolderHandler{
		documentService: documentService,
		userService:     userService,
	}
}

// RegisterRoutes sets up the folder management routes
func (h *FolderHandler) RegisterRoutes(router *gin.RouterGroup) {
	folders := router.Group("/folders")
	// Note: Auth middleware should be applied at server level
	{
		// CRUD operations
		folders.POST("", h.CreateFolder)
		folders.GET("", h.ListFolders)
		folders.GET("/:id", h.GetFolder)
		folders.PUT("/:id", h.UpdateFolder)
		folders.DELETE("/:id", h.DeleteFolder)

		// Special operations
		folders.GET("/:id/tree", h.GetFolderTree)
		folders.POST("/:id/move", h.MoveFolder)
		folders.GET("/:id/documents", h.GetFolderDocuments)
	}
}

// Request/Response DTOs

// CreateFolderRequest contains folder creation data
type CreateFolderRequest struct {
	Name        string  `json:"name" binding:"required,min=1,max=255"`
	Description string  `json:"description,omitempty" binding:"max=1000"`
	ParentID    *string `json:"parent_id,omitempty" binding:"omitempty,uuid"`
	Color       string  `json:"color,omitempty" binding:"omitempty,len=7"`
	Icon        string  `json:"icon,omitempty" binding:"omitempty,max=50"`
}

// UpdateFolderRequest contains folder update data
type UpdateFolderRequest struct {
	Name        *string `json:"name,omitempty" binding:"omitempty,min=1,max=255"`
	Description *string `json:"description,omitempty" binding:"omitempty,max=1000"`
	Color       *string `json:"color,omitempty" binding:"omitempty,len=7"`
	Icon        *string `json:"icon,omitempty" binding:"omitempty,max=50"`
}

// MoveFolderRequest contains folder move data
type MoveFolderRequest struct {
	NewParentID *string `json:"new_parent_id,omitempty" binding:"omitempty,uuid"`
}

// FolderResponse represents folder data in API responses
type FolderResponse struct {
	ID            uuid.UUID       `json:"id"`
	Name          string          `json:"name"`
	Description   string          `json:"description"`
	Path          string          `json:"path"`
	Level         int             `json:"level"`
	IsSystem      bool            `json:"is_system"`
	Color         string          `json:"color"`
	Icon          string          `json:"icon"`
	ParentID      *uuid.UUID      `json:"parent_id,omitempty"`
	DocumentCount int64           `json:"document_count"`
	CreatedBy     uuid.UUID       `json:"created_by"`
	CreatedAt     string          `json:"created_at"`
	UpdatedAt     string          `json:"updated_at"`
	Parent        *FolderSummary  `json:"parent,omitempty"`
	Children      []FolderSummary `json:"children,omitempty"`
}

// FolderSummary represents simplified folder info
type FolderSummary struct {
	ID            uuid.UUID `json:"id"`
	Name          string    `json:"name"`
	Path          string    `json:"path"`
	Level         int       `json:"level"`
	IsSystem      bool      `json:"is_system"`
	Color         string    `json:"color"`
	Icon          string    `json:"icon"`
	ChildCount    int       `json:"child_count"`
	DocumentCount int64     `json:"document_count"`
}

// FolderTreeResponse represents folder hierarchy
type FolderTreeResponse struct {
	Folders []FolderTreeNode `json:"folders"`
}

// FolderTreeNode represents a node in the folder tree
type FolderTreeNode struct {
	ID            uuid.UUID        `json:"id"`
	Name          string           `json:"name"`
	Path          string           `json:"path"`
	Level         int              `json:"level"`
	IsSystem      bool             `json:"is_system"`
	Color         string           `json:"color"`
	Icon          string           `json:"icon"`
	DocumentCount int64            `json:"document_count"`
	Children      []FolderTreeNode `json:"children"`
}

// FolderListResponse represents paginated folder list
type FolderListResponse struct {
	Folders    []FolderResponse `json:"folders"`
	Total      int64            `json:"total"`
	Page       int              `json:"page"`
	PerPage    int              `json:"per_page"`
	TotalPages int              `json:"total_pages"`
}

// FolderDocumentsResponse represents documents in a folder
type FolderDocumentsResponse struct {
	Documents  []DocumentSummary `json:"documents"`
	Total      int64             `json:"total"`
	Page       int               `json:"page"`
	PerPage    int               `json:"per_page"`
	TotalPages int               `json:"total_pages"`
}

// DocumentSummary represents simplified document info for folder contents
type DocumentSummary struct {
	ID           uuid.UUID `json:"id"`
	FileName     string    `json:"file_name"`
	OriginalName string    `json:"original_name"`
	Title        string    `json:"title"`
	ContentType  string    `json:"content_type"`
	FileSize     int64     `json:"file_size"`
	Status       string    `json:"status"`
	CreatedAt    string    `json:"created_at"`
	UpdatedAt    string    `json:"updated_at"`
}

// Handler Methods

// CreateFolder creates a new folder
// @Summary Create folder
// @Description Create a new folder in the document hierarchy
// @Tags folders
// @Accept json
// @Produce json
// @Param request body CreateFolderRequest true "Folder creation request"
// @Success 201 {object} FolderResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 409 {object} ErrorResponse
// @Router /folders [post]
func (h *FolderHandler) CreateFolder(c *gin.Context) {
	userCtx := getUserContextFromGin(c)
	if userCtx == nil {
		c.JSON(http.StatusUnauthorized, ErrorResponse{
			Error:   "unauthorized",
			Message: "User context not found",
		})
		return
	}

	var req CreateFolderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "invalid_request",
			Message: "Invalid request format",
			Details: err.Error(),
		})
		return
	}

	// Parse parent ID if provided
	var parentID *uuid.UUID
	if req.ParentID != nil && *req.ParentID != "" {
		id, err := uuid.Parse(*req.ParentID)
		if err != nil {
			c.JSON(http.StatusBadRequest, ErrorResponse{
				Error:   "invalid_parent_id",
				Message: "Invalid parent folder ID format",
			})
			return
		}
		parentID = &id
	}

	// Create folder
	folder, err := h.createFolder(c.Request.Context(), userCtx.TenantID, userCtx.UserID, req.Name, req.Description, parentID, req.Color, req.Icon)
	if err != nil {
		if strings.Contains(err.Error(), "already exists") {
			c.JSON(http.StatusConflict, ErrorResponse{
				Error:   "folder_exists",
				Message: "A folder with this name already exists in the parent directory",
			})
			return
		}

		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "create_failed",
			Message: "Failed to create folder",
			Details: err.Error(),
		})
		return
	}

	response := h.convertToFolderResponse(folder)
	c.JSON(http.StatusCreated, response)
}

// ListFolders lists folders with hierarchy support
// @Summary List folders
// @Description List folders with optional hierarchy view and filtering
// @Tags folders
// @Produce json
// @Param parent_id query string false "Filter by parent folder ID (omit for root folders)"
// @Param include_children query bool false "Include child folders in response"
// @Param page query int false "Page number" default(1)
// @Param per_page query int false "Items per page" default(20)
// @Success 200 {object} FolderListResponse
// @Failure 401 {object} ErrorResponse
// @Router /folders [get]
func (h *FolderHandler) ListFolders(c *gin.Context) {
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

	includeChildren := c.Query("include_children") == "true"
	parentIDStr := c.Query("parent_id")

	// Get folders
	folders, err := h.getFolders(c.Request.Context(), userCtx.TenantID, parentIDStr)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "list_failed",
			Message: "Failed to list folders",
			Details: err.Error(),
		})
		return
	}

	// Convert to response format
	var folderResponses []FolderResponse
	for _, folder := range folders {
		folderResponse := h.convertToFolderResponse(&folder)

		// Add children if requested
		if includeChildren {
			children, _ := h.getFolderChildren(c.Request.Context(), folder.ID)
			folderResponse.Children = children
		}

		folderResponses = append(folderResponses, folderResponse)
	}

	// Apply pagination
	total := int64(len(folderResponses))
	startIdx := (page - 1) * perPage
	endIdx := startIdx + perPage
	if startIdx > len(folderResponses) {
		startIdx = len(folderResponses)
	}
	if endIdx > len(folderResponses) {
		endIdx = len(folderResponses)
	}

	paginatedFolders := folderResponses[startIdx:endIdx]
	totalPages := int((total + int64(perPage) - 1) / int64(perPage))

	response := FolderListResponse{
		Folders:    paginatedFolders,
		Total:      total,
		Page:       page,
		PerPage:    perPage,
		TotalPages: totalPages,
	}

	c.JSON(http.StatusOK, response)
}

// GetFolder retrieves a specific folder with its details
// @Summary Get folder details
// @Description Get detailed information about a specific folder
// @Tags folders
// @Produce json
// @Param id path string true "Folder ID"
// @Param include_children query bool false "Include child folders"
// @Success 200 {object} FolderResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Router /folders/{id} [get]
func (h *FolderHandler) GetFolder(c *gin.Context) {
	userCtx := getUserContextFromGin(c)
	if userCtx == nil {
		c.JSON(http.StatusUnauthorized, ErrorResponse{
			Error:   "unauthorized",
			Message: "User context not found",
		})
		return
	}

	folderID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "invalid_folder_id",
			Message: "Invalid folder ID format",
		})
		return
	}

	includeChildren := c.Query("include_children") == "true"

	// Get folder
	folder, err := h.getFolder(c.Request.Context(), folderID, userCtx.TenantID)
	if err != nil {
		c.JSON(http.StatusNotFound, ErrorResponse{
			Error:   "folder_not_found",
			Message: "Folder not found",
		})
		return
	}

	response := h.convertToFolderResponse(folder)

	// Add children if requested
	if includeChildren {
		children, _ := h.getFolderChildren(c.Request.Context(), folder.ID)
		response.Children = children
	}

	c.JSON(http.StatusOK, response)
}

// UpdateFolder updates an existing folder
// @Summary Update folder
// @Description Update folder information (name, description, etc.)
// @Tags folders
// @Accept json
// @Produce json
// @Param id path string true "Folder ID"
// @Param request body UpdateFolderRequest true "Folder update request"
// @Success 200 {object} FolderResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Router /folders/{id} [put]
func (h *FolderHandler) UpdateFolder(c *gin.Context) {
	userCtx := getUserContextFromGin(c)
	if userCtx == nil {
		c.JSON(http.StatusUnauthorized, ErrorResponse{
			Error:   "unauthorized",
			Message: "User context not found",
		})
		return
	}

	folderID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "invalid_folder_id",
			Message: "Invalid folder ID format",
		})
		return
	}

	var req UpdateFolderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "invalid_request",
			Message: "Invalid request format",
			Details: err.Error(),
		})
		return
	}

	// Update folder
	folder, err := h.updateFolder(c.Request.Context(), folderID, userCtx.TenantID, userCtx.UserID, req)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			c.JSON(http.StatusNotFound, ErrorResponse{
				Error:   "folder_not_found",
				Message: "Folder not found",
			})
			return
		}
		if strings.Contains(err.Error(), "system folder") {
			c.JSON(http.StatusForbidden, ErrorResponse{
				Error:   "system_folder",
				Message: "Cannot modify system folders",
			})
			return
		}

		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "update_failed",
			Message: "Failed to update folder",
			Details: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, h.convertToFolderResponse(folder))
}

// DeleteFolder soft deletes a folder
// @Summary Delete folder
// @Description Delete a folder (must be empty of documents and subfolders)
// @Tags folders
// @Param id path string true "Folder ID"
// @Success 200 {object} SuccessResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 409 {object} ErrorResponse
// @Router /folders/{id} [delete]
func (h *FolderHandler) DeleteFolder(c *gin.Context) {
	userCtx := getUserContextFromGin(c)
	if userCtx == nil {
		c.JSON(http.StatusUnauthorized, ErrorResponse{
			Error:   "unauthorized",
			Message: "User context not found",
		})
		return
	}

	folderID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "invalid_folder_id",
			Message: "Invalid folder ID format",
		})
		return
	}

	// Delete folder
	err = h.deleteFolder(c.Request.Context(), folderID, userCtx.TenantID, userCtx.UserID)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			c.JSON(http.StatusNotFound, ErrorResponse{
				Error:   "folder_not_found",
				Message: "Folder not found",
			})
			return
		}
		if strings.Contains(err.Error(), "system folder") {
			c.JSON(http.StatusForbidden, ErrorResponse{
				Error:   "system_folder",
				Message: "Cannot delete system folders",
			})
			return
		}
		if strings.Contains(err.Error(), "child folders") ||
			strings.Contains(err.Error(), "containing documents") {
			c.JSON(http.StatusConflict, ErrorResponse{
				Error:   "folder_not_empty",
				Message: "Cannot delete folder that contains documents or subfolders",
			})
			return
		}

		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "delete_failed",
			Message: "Failed to delete folder",
			Details: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, SuccessResponse{
		Message: "Folder deleted successfully",
	})
}

// GetFolderTree retrieves folder hierarchy/tree
// @Summary Get folder tree
// @Description Get the complete folder hierarchy starting from a specific folder
// @Tags folders
// @Produce json
// @Param id path string true "Root folder ID (use 'root' for tenant root)"
// @Success 200 {object} FolderTreeResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Router /folders/{id}/tree [get]
func (h *FolderHandler) GetFolderTree(c *gin.Context) {
	userCtx := getUserContextFromGin(c)
	if userCtx == nil {
		c.JSON(http.StatusUnauthorized, ErrorResponse{
			Error:   "unauthorized",
			Message: "User context not found",
		})
		return
	}

	idParam := c.Param("id")

	// Get folder tree
	tree, err := h.getFolderTree(c.Request.Context(), userCtx.TenantID, idParam)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "tree_fetch_failed",
			Message: "Failed to fetch folder tree",
			Details: err.Error(),
		})
		return
	}

	response := FolderTreeResponse{
		Folders: h.convertToFolderTreeNodes(tree),
	}

	c.JSON(http.StatusOK, response)
}

// MoveFolder moves a folder to a new parent
// @Summary Move folder
// @Description Move a folder to a new parent location in the hierarchy
// @Tags folders
// @Accept json
// @Produce json
// @Param id path string true "Folder ID to move"
// @Param request body MoveFolderRequest true "Move folder request"
// @Success 200 {object} FolderResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 409 {object} ErrorResponse
// @Router /folders/{id}/move [post]
func (h *FolderHandler) MoveFolder(c *gin.Context) {
	userCtx := getUserContextFromGin(c)
	if userCtx == nil {
		c.JSON(http.StatusUnauthorized, ErrorResponse{
			Error:   "unauthorized",
			Message: "User context not found",
		})
		return
	}

	folderID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "invalid_folder_id",
			Message: "Invalid folder ID format",
		})
		return
	}

	var req MoveFolderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "invalid_request",
			Message: "Invalid request format",
			Details: err.Error(),
		})
		return
	}

	// Parse new parent ID
	var newParentID uuid.UUID
	if req.NewParentID != nil && *req.NewParentID != "" {
		id, err := uuid.Parse(*req.NewParentID)
		if err != nil {
			c.JSON(http.StatusBadRequest, ErrorResponse{
				Error:   "invalid_parent_id",
				Message: "Invalid new parent ID format",
			})
			return
		}
		newParentID = id
	}

	// Move folder
	folder, err := h.moveFolder(c.Request.Context(), folderID, newParentID, userCtx.TenantID, userCtx.UserID)
	if err != nil {
		if strings.Contains(err.Error(), "itself or its descendant") {
			c.JSON(http.StatusConflict, ErrorResponse{
				Error:   "invalid_move",
				Message: "Cannot move folder to itself or its descendant",
			})
			return
		}

		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "move_failed",
			Message: "Failed to move folder",
			Details: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, h.convertToFolderResponse(folder))
}

// GetFolderDocuments lists documents in a specific folder
// @Summary Get folder documents
// @Description Get all documents within a specific folder with pagination
// @Tags folders
// @Produce json
// @Param id path string true "Folder ID"
// @Param page query int false "Page number" default(1)
// @Param per_page query int false "Items per page" default(20)
// @Param sort_by query string false "Sort field" default("created_at")
// @Param sort_desc query bool false "Sort descending" default(true)
// @Success 200 {object} FolderDocumentsResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Router /folders/{id}/documents [get]
func (h *FolderHandler) GetFolderDocuments(c *gin.Context) {
	userCtx := getUserContextFromGin(c)
	if userCtx == nil {
		c.JSON(http.StatusUnauthorized, ErrorResponse{
			Error:   "unauthorized",
			Message: "User context not found",
		})
		return
	}

	folderID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "invalid_folder_id",
			Message: "Invalid folder ID format",
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

	sortBy := c.DefaultQuery("sort_by", "created_at")
	sortDesc := c.DefaultQuery("sort_desc", "true") == "true"

	// Get documents in folder using DocumentService
	filters := repositories.DocumentFilters{
		FolderID: &folderID,
		ListParams: repositories.ListParams{
			Page:     page,
			PageSize: perPage,
			SortBy:   sortBy,
			SortDesc: sortDesc,
		},
	}

	documents, total, err := h.documentService.ListDocuments(c.Request.Context(), userCtx.TenantID, filters)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "documents_fetch_failed",
			Message: "Failed to fetch folder documents",
			Details: err.Error(),
		})
		return
	}

	// Convert to response format
	var documentSummaries []DocumentSummary
	for _, doc := range documents {
		documentSummaries = append(documentSummaries, h.convertToDocumentSummary(&doc))
	}

	totalPages := int((total + int64(perPage) - 1) / int64(perPage))

	response := FolderDocumentsResponse{
		Documents:  documentSummaries,
		Total:      total,
		Page:       page,
		PerPage:    perPage,
		TotalPages: totalPages,
	}

	c.JSON(http.StatusOK, response)
}

// Helper Methods

// Service integration methods - These use the real DocumentService folder methods

func (h *FolderHandler) createFolder(ctx context.Context, tenantID, userID uuid.UUID, name, description string, parentID *uuid.UUID, color, icon string) (*models.Folder, error) {
	return h.documentService.CreateFolder(ctx, tenantID, userID, name, description, parentID, color, icon)
}

func (h *FolderHandler) getFolders(ctx context.Context, tenantID uuid.UUID, parentIDStr string) ([]models.Folder, error) {
	var parentID *uuid.UUID
	if parentIDStr != "" {
		id, err := uuid.Parse(parentIDStr)
		if err != nil {
			return nil, fmt.Errorf("invalid parent ID format")
		}
		parentID = &id
	}

	return h.documentService.GetFolders(ctx, tenantID, parentID)
}

func (h *FolderHandler) getFolder(ctx context.Context, folderID, tenantID uuid.UUID) (*models.Folder, error) {
	return h.documentService.GetFolder(ctx, folderID, tenantID)
}

func (h *FolderHandler) updateFolder(ctx context.Context, folderID, tenantID, userID uuid.UUID, req UpdateFolderRequest) (*models.Folder, error) {
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

	return h.documentService.UpdateFolder(ctx, folderID, tenantID, updates, userID)
}

func (h *FolderHandler) deleteFolder(ctx context.Context, folderID, tenantID, userID uuid.UUID) error {
	return h.documentService.DeleteFolder(ctx, folderID, tenantID, userID)
}

func (h *FolderHandler) getFolderTree(ctx context.Context, tenantID uuid.UUID, rootID string) ([]repositories.FolderNode, error) {
	// For now, get complete tree for tenant (rootID filtering can be added later)
	return h.documentService.GetFolderTree(ctx, tenantID)
}

func (h *FolderHandler) moveFolder(ctx context.Context, folderID, newParentID, tenantID, userID uuid.UUID) (*models.Folder, error) {
	return h.documentService.MoveFolder(ctx, folderID, newParentID, tenantID, userID)
}

func (h *FolderHandler) getFolderChildren(ctx context.Context, folderID uuid.UUID) ([]FolderSummary, error) {
	children, err := h.documentService.GetFolderChildren(ctx, folderID)
	if err != nil {
		return nil, err
	}

	var summaries []FolderSummary
	for _, child := range children {
		// Get document count for each child
		_, total, err := h.documentService.GetFolderDocuments(ctx, child.ID, child.TenantID, repositories.DocumentFilters{
			ListParams: repositories.ListParams{Page: 1, PageSize: 1},
		})

		docCount := int64(0)
		if err == nil {
			docCount = total
		}

		summary := FolderSummary{
			ID:            child.ID,
			Name:          child.Name,
			Path:          child.Path,
			Level:         child.Level,
			IsSystem:      child.IsSystem,
			Color:         child.Color,
			Icon:          child.Icon,
			ChildCount:    0, // Would need additional query to get child count
			DocumentCount: docCount,
		}
		summaries = append(summaries, summary)
	}

	return summaries, nil
}

// Conversion methods

func (h *FolderHandler) convertToFolderResponse(folder *models.Folder) FolderResponse {
	if folder == nil {
		return FolderResponse{}
	}

	response := FolderResponse{
		ID:            folder.ID,
		Name:          folder.Name,
		Description:   folder.Description,
		Path:          folder.Path,
		Level:         folder.Level,
		IsSystem:      folder.IsSystem,
		Color:         folder.Color,
		Icon:          folder.Icon,
		ParentID:      folder.ParentID,
		DocumentCount: 0, // Would be populated from service
		CreatedBy:     folder.CreatedBy,
		CreatedAt:     folder.CreatedAt.Format("2006-01-02T15:04:05Z"),
		UpdatedAt:     folder.UpdatedAt.Format("2006-01-02T15:04:05Z"),
	}

	// Add parent info if available
	if folder.Parent != nil {
		response.Parent = &FolderSummary{
			ID:       folder.Parent.ID,
			Name:     folder.Parent.Name,
			Path:     folder.Parent.Path,
			Level:    folder.Parent.Level,
			IsSystem: folder.Parent.IsSystem,
			Color:    folder.Parent.Color,
			Icon:     folder.Parent.Icon,
		}
	}

	return response
}

func (h *FolderHandler) convertToFolderTreeNodes(nodes []repositories.FolderNode) []FolderTreeNode {
	var result []FolderTreeNode
	for _, node := range nodes {
		treeNode := FolderTreeNode{
			ID:            node.Folder.ID,
			Name:          node.Folder.Name,
			Path:          node.Folder.Path,
			Level:         node.Folder.Level,
			IsSystem:      node.Folder.IsSystem,
			Color:         node.Folder.Color,
			Icon:          node.Folder.Icon,
			DocumentCount: node.DocumentCount,
			Children:      h.convertToFolderTreeNodes(node.Children),
		}
		result = append(result, treeNode)
	}
	return result
}

func (h *FolderHandler) convertToDocumentSummary(doc *models.Document) DocumentSummary {
	return DocumentSummary{
		ID:           doc.ID,
		FileName:     doc.FileName,
		OriginalName: doc.OriginalName,
		Title:        doc.Title,
		ContentType:  doc.ContentType,
		FileSize:     doc.FileSize,
		Status:       string(doc.Status),
		CreatedAt:    doc.CreatedAt.Format("2006-01-02T15:04:05Z"),
		UpdatedAt:    doc.UpdatedAt.Format("2006-01-02T15:04:05Z"),
	}
}
