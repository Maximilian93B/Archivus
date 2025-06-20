package handlers

import (
	"io"
	"net/http"
	"strconv"

	"github.com/archivus/archivus/internal/app/middleware"
	"github.com/archivus/archivus/internal/domain/repositories"
	"github.com/archivus/archivus/internal/domain/services"
	"github.com/archivus/archivus/internal/infrastructure/database/models"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// DocumentHandler handles HTTP requests for document operations
type DocumentHandler struct {
	*BaseHandler
	documentService *services.DocumentService
	userService     *services.UserService
}

// NewDocumentHandler creates a new document handler
func NewDocumentHandler(documentService *services.DocumentService, userService *services.UserService) *DocumentHandler {
	return &DocumentHandler{
		BaseHandler:     NewBaseHandler(),
		documentService: documentService,
		userService:     userService,
	}
}

// UploadDocumentRequest represents the document upload request
type UploadDocumentRequest struct {
	FolderID     *string                `form:"folder_id"`
	Title        string                 `form:"title"`
	Description  string                 `form:"description"`
	DocumentType string                 `form:"document_type"`
	Tags         []string               `form:"tags"`
	Categories   []string               `form:"categories"`
	CustomFields map[string]interface{} `form:"custom_fields"`

	// Financial fields
	Amount       *float64 `form:"amount"`
	Currency     string   `form:"currency"`
	TaxAmount    *float64 `form:"tax_amount"`
	VendorName   string   `form:"vendor_name"`
	CustomerName string   `form:"customer_name"`
	DocumentDate string   `form:"document_date"` // ISO format
	DueDate      string   `form:"due_date"`      // ISO format
	ExpiryDate   string   `form:"expiry_date"`   // ISO format

	// Processing options
	EnableAI           bool `form:"enable_ai"`
	EnableOCR          bool `form:"enable_ocr"`
	SkipDuplicateCheck bool `form:"skip_duplicate_check"`
}

// DocumentResponse represents the document response
type DocumentResponse struct {
	*models.Document
	DownloadURL string          `json:"download_url,omitempty"`
	PreviewURL  string          `json:"preview_url,omitempty"`
	Permissions map[string]bool `json:"permissions"`
}

// SearchRequest represents document search parameters
type SearchRequest struct {
	Query         string   `json:"query" form:"q"`
	DocumentTypes []string `json:"document_types" form:"types"`
	FolderIDs     []string `json:"folder_ids" form:"folders"`
	TagIDs        []string `json:"tag_ids" form:"tags"`
	DateFrom      string   `json:"date_from" form:"date_from"`
	DateTo        string   `json:"date_to" form:"date_to"`
	Fuzzy         bool     `json:"fuzzy" form:"fuzzy"`
	Limit         int      `json:"limit" form:"limit"`
	Page          int      `json:"page" form:"page"`
	PageSize      int      `json:"page_size" form:"page_size"`
}

// PaginatedResponse represents paginated API response
type PaginatedResponse struct {
	Data       interface{} `json:"data"`
	Total      int64       `json:"total"`
	Page       int         `json:"page"`
	PageSize   int         `json:"page_size"`
	TotalPages int         `json:"total_pages"`
}

// RegisterRoutes registers all document routes
func (h *DocumentHandler) RegisterRoutes(router *gin.RouterGroup) {
	docs := router.Group("/documents")
	{
		docs.POST("/upload", h.UploadDocument)
		docs.GET("/", h.ListDocuments)
		docs.GET("/search", h.SearchDocuments)
		docs.GET("/:id", h.GetDocument)
		docs.PUT("/:id", h.UpdateDocument)
		docs.DELETE("/:id", h.DeleteDocument)
		docs.GET("/:id/download", h.DownloadDocument)
		docs.GET("/:id/preview", h.PreviewDocument)
		docs.POST("/:id/process-financial", h.ProcessFinancialDocument)
		docs.GET("/duplicates", h.FindDuplicates)
		docs.GET("/expiring", h.GetExpiringDocuments)
	}
}

// UploadDocument handles document upload
// @Summary Upload a document
// @Description Upload a new document with optional AI processing
// @Tags documents
// @Accept multipart/form-data
// @Produce json
// @Param file formData file true "Document file"
// @Param data formData string false "Document metadata (JSON)"
// @Success 201 {object} DocumentResponse
// @Failure 400 {object} ErrorResponse
// @Failure 413 {object} ErrorResponse "File too large"
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/documents/upload [post]
func (h *DocumentHandler) UploadDocument(c *gin.Context) {
	userCtx, ok := h.AuthenticateUser(c)
	if !ok {
		return
	}

	// Parse multipart form
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		h.RespondBadRequest(c, "No file uploaded or invalid file", err.Error())
		return
	}
	defer file.Close()

	// Parse form data
	var req UploadDocumentRequest
	if err := c.ShouldBind(&req); err != nil {
		h.RespondBadRequest(c, "Invalid form data", err.Error())
		return
	}

	// Convert form data to service parameters
	params := services.UploadDocumentParams{
		TenantID:           userCtx.TenantID,
		UserID:             userCtx.UserID,
		File:               header,
		Title:              req.Title,
		Description:        req.Description,
		Tags:               req.Tags,
		Categories:         req.Categories,
		CustomFields:       req.CustomFields,
		Amount:             req.Amount,
		Currency:           req.Currency,
		TaxAmount:          req.TaxAmount,
		VendorName:         req.VendorName,
		CustomerName:       req.CustomerName,
		EnableAI:           req.EnableAI,
		EnableOCR:          req.EnableOCR,
		SkipDuplicateCheck: req.SkipDuplicateCheck,
	}

	// Parse folder ID if provided
	if req.FolderID != nil && *req.FolderID != "" {
		folderID, err := uuid.Parse(*req.FolderID)
		if err != nil {
			c.JSON(http.StatusBadRequest, ErrorResponse{
				Error:   "invalid_folder_id",
				Message: "Invalid folder ID format",
			})
			return
		}
		params.FolderID = &folderID
	}

	// Parse document type
	if req.DocumentType != "" {
		params.DocumentType = models.DocumentType(req.DocumentType)
	}

	// Parse dates
	if req.DocumentDate != "" {
		if date, err := parseDate(req.DocumentDate); err == nil {
			params.DocumentDate = &date
		}
	}
	if req.DueDate != "" {
		if date, err := parseDate(req.DueDate); err == nil {
			params.DueDate = &date
		}
	}
	if req.ExpiryDate != "" {
		if date, err := parseDate(req.ExpiryDate); err == nil {
			params.ExpiryDate = &date
		}
	}

	// Upload document
	document, err := h.documentService.UploadDocument(c.Request.Context(), params)
	if err != nil {
		statusCode := http.StatusInternalServerError
		errorCode := "upload_failed"

		// Map specific errors to appropriate HTTP status codes
		switch err {
		case services.ErrQuotaExceeded:
			statusCode = http.StatusPaymentRequired
			errorCode = "quota_exceeded"
		case services.ErrDocumentTooLarge:
			statusCode = http.StatusRequestEntityTooLarge
			errorCode = "file_too_large"
		case services.ErrUnsupportedFormat:
			statusCode = http.StatusUnsupportedMediaType
			errorCode = "unsupported_format"
		case services.ErrDocumentExists:
			statusCode = http.StatusConflict
			errorCode = "document_exists"
		}

		c.JSON(statusCode, ErrorResponse{
			Error:   errorCode,
			Message: err.Error(),
		})
		return
	}

	// Build response with permissions
	response := &DocumentResponse{
		Document:    document,
		Permissions: h.getDocumentPermissions(userCtx, document),
	}

	c.JSON(http.StatusCreated, response)
}

// GetDocument retrieves a specific document
// @Summary Get document by ID
// @Description Retrieve a specific document by its ID
// @Tags documents
// @Produce json
// @Param id path string true "Document ID"
// @Success 200 {object} DocumentResponse
// @Failure 404 {object} ErrorResponse
// @Router /api/v1/documents/{id} [get]
func (h *DocumentHandler) GetDocument(c *gin.Context) {
	userCtx, ok := h.AuthenticateUser(c)
	if !ok {
		return
	}

	documentID, ok := h.ValidateUUID(c, "document ID", c.Param("id"))
	if !ok {
		return
	}

	document, err := h.documentService.GetDocument(c.Request.Context(), documentID, userCtx.TenantID, userCtx.UserID)
	if err != nil {
		if err == services.ErrDocumentNotFound {
			c.JSON(http.StatusNotFound, ErrorResponse{
				Error:   "document_not_found",
				Message: "Document not found",
			})
			return
		}
		if err == services.ErrUnauthorizedAccess {
			c.JSON(http.StatusForbidden, ErrorResponse{
				Error:   "access_denied",
				Message: "Access denied to this document",
			})
			return
		}

		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "internal_error",
			Message: "Failed to retrieve document",
			Details: err.Error(),
		})
		return
	}

	response := &DocumentResponse{
		Document:    document,
		Permissions: h.getDocumentPermissions(userCtx, document),
	}

	c.JSON(http.StatusOK, response)
}

// ListDocuments lists documents with filtering and pagination
// @Summary List documents
// @Description List documents with optional filtering and pagination
// @Tags documents
// @Produce json
// @Param folder_id query string false "Filter by folder ID"
// @Param document_type query string false "Filter by document type"
// @Param status query string false "Filter by status"
// @Param page query int false "Page number" default(1)
// @Param page_size query int false "Page size" default(20)
// @Success 200 {object} PaginatedResponse
// @Router /api/v1/documents [get]
func (h *DocumentHandler) ListDocuments(c *gin.Context) {
	userCtx, ok := h.AuthenticateUser(c)
	if !ok {
		return
	}

	page, pageSize := h.ParsePagination(c)
	sortBy, sortDesc := h.ParseSorting(c, "created_at")

	// Parse query parameters
	filters := repositories.DocumentFilters{
		ListParams: repositories.ListParams{
			Page:     page,
			PageSize: pageSize,
			SortBy:   sortBy,
			SortDesc: sortDesc,
			Search:   c.Query("search"),
		},
	}

	// Parse filters
	if folderID := c.Query("folder_id"); folderID != "" {
		if id, err := uuid.Parse(folderID); err == nil {
			filters.FolderID = &id
		}
	}

	if docType := c.Query("document_type"); docType != "" {
		filters.DocumentType = []models.DocumentType{models.DocumentType(docType)}
	}

	if status := c.Query("status"); status != "" {
		filters.Status = []models.DocStatus{models.DocStatus(status)}
	}

	// Get documents
	documents, total, err := h.documentService.ListDocuments(c.Request.Context(), userCtx.TenantID, filters)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "list_failed",
			Message: "Failed to list documents",
			Details: err.Error(),
		})
		return
	}

	// Build response with permissions for each document
	var responses []DocumentResponse
	for _, doc := range documents {
		responses = append(responses, DocumentResponse{
			Document:    &doc,
			Permissions: h.getDocumentPermissions(userCtx, &doc),
		})
	}

	// Calculate pagination
	totalPages := int((total + int64(filters.PageSize) - 1) / int64(filters.PageSize))

	c.JSON(http.StatusOK, PaginatedResponse{
		Data:       responses,
		Total:      total,
		Page:       filters.Page,
		PageSize:   filters.PageSize,
		TotalPages: totalPages,
	})
}

// SearchDocuments performs intelligent document search
// @Summary Search documents
// @Description Search documents using full-text and semantic search
// @Tags documents
// @Accept json
// @Produce json
// @Param request body SearchRequest true "Search parameters"
// @Success 200 {array} DocumentResponse
// @Router /api/v1/documents/search [post]
func (h *DocumentHandler) SearchDocuments(c *gin.Context) {
	userCtx, ok := h.AuthenticateUser(c)
	if !ok {
		return
	}

	var req SearchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.RespondBadRequest(c, "Invalid search parameters", err.Error())
		return
	}

	// Convert to service query
	query := repositories.SearchQuery{
		Query: req.Query,
		Fuzzy: req.Fuzzy,
		Limit: req.Limit,
	}

	if req.Limit == 0 {
		query.Limit = 50 // Default limit
	}

	// Parse document types
	for _, dt := range req.DocumentTypes {
		query.DocumentTypes = append(query.DocumentTypes, models.DocumentType(dt))
	}

	// Parse folder IDs
	for _, fid := range req.FolderIDs {
		if id, err := uuid.Parse(fid); err == nil {
			query.FolderIDs = append(query.FolderIDs, id)
		}
	}

	// Parse tag IDs
	for _, tid := range req.TagIDs {
		if id, err := uuid.Parse(tid); err == nil {
			query.TagIDs = append(query.TagIDs, id)
		}
	}

	// Parse dates
	if req.DateFrom != "" {
		if date, err := parseDate(req.DateFrom); err == nil {
			query.DateFrom = &date
		}
	}
	if req.DateTo != "" {
		if date, err := parseDate(req.DateTo); err == nil {
			query.DateTo = &date
		}
	}

	// Perform search
	documents, err := h.documentService.SearchDocuments(c.Request.Context(), userCtx.TenantID, query)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "search_failed",
			Message: "Search failed",
			Details: err.Error(),
		})
		return
	}

	// Build response
	var responses []DocumentResponse
	for _, doc := range documents {
		responses = append(responses, DocumentResponse{
			Document:    &doc,
			Permissions: h.getDocumentPermissions(userCtx, &doc),
		})
	}

	c.JSON(http.StatusOK, responses)
}

// UpdateDocument updates document metadata
// @Summary Update document
// @Description Update document metadata and properties
// @Tags documents
// @Accept json
// @Produce json
// @Param id path string true "Document ID"
// @Param updates body map[string]interface{} true "Document updates"
// @Success 200 {object} DocumentResponse
// @Router /api/v1/documents/{id} [put]
func (h *DocumentHandler) UpdateDocument(c *gin.Context) {
	userCtx := middleware.GetUserContext(c)
	if userCtx == nil {
		c.JSON(http.StatusUnauthorized, ErrorResponse{
			Error:   "unauthorized",
			Message: "User not authenticated",
		})
		return
	}

	documentID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "invalid_document_id",
			Message: "Invalid document ID format",
		})
		return
	}

	// Check permissions
	hasPermission, err := h.userService.CheckPermission(c.Request.Context(), userCtx.UserID, "documents.update")
	if err != nil || !hasPermission {
		c.JSON(http.StatusForbidden, ErrorResponse{
			Error:   "permission_denied",
			Message: "Insufficient permissions to update documents",
		})
		return
	}

	var updates map[string]interface{}
	if err := c.ShouldBindJSON(&updates); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "invalid_request",
			Message: "Invalid update data",
			Details: err.Error(),
		})
		return
	}

	// Update document
	document, err := h.documentService.UpdateDocument(c.Request.Context(), documentID, updates, userCtx.UserID)
	if err != nil {
		if err == services.ErrDocumentNotFound {
			c.JSON(http.StatusNotFound, ErrorResponse{
				Error:   "document_not_found",
				Message: "Document not found",
			})
			return
		}

		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "update_failed",
			Message: "Failed to update document",
			Details: err.Error(),
		})
		return
	}

	response := &DocumentResponse{
		Document:    document,
		Permissions: h.getDocumentPermissions(userCtx, document),
	}

	c.JSON(http.StatusOK, response)
}

// DeleteDocument soft deletes a document
// @Summary Delete document
// @Description Soft delete a document
// @Tags documents
// @Param id path string true "Document ID"
// @Success 204
// @Router /api/v1/documents/{id} [delete]
func (h *DocumentHandler) DeleteDocument(c *gin.Context) {
	userCtx := middleware.GetUserContext(c)
	if userCtx == nil {
		c.JSON(http.StatusUnauthorized, ErrorResponse{
			Error:   "unauthorized",
			Message: "User not authenticated",
		})
		return
	}

	documentID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "invalid_document_id",
			Message: "Invalid document ID format",
		})
		return
	}

	// Check permissions
	hasPermission, err := h.userService.CheckPermission(c.Request.Context(), userCtx.UserID, "documents.delete")
	if err != nil || !hasPermission {
		c.JSON(http.StatusForbidden, ErrorResponse{
			Error:   "permission_denied",
			Message: "Insufficient permissions to delete documents",
		})
		return
	}

	// Delete document
	err = h.documentService.DeleteDocument(c.Request.Context(), documentID, userCtx.UserID)
	if err != nil {
		if err == services.ErrDocumentNotFound {
			c.JSON(http.StatusNotFound, ErrorResponse{
				Error:   "document_not_found",
				Message: "Document not found",
			})
			return
		}

		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "delete_failed",
			Message: "Failed to delete document",
			Details: err.Error(),
		})
		return
	}

	c.Status(http.StatusNoContent)
}

// ProcessFinancialDocument triggers AI processing for financial documents
// @Summary Process financial document
// @Description Trigger AI processing to extract financial data
// @Tags documents
// @Param id path string true "Document ID"
// @Success 202 {object} map[string]string
// @Router /api/v1/documents/{id}/process-financial [post]
func (h *DocumentHandler) ProcessFinancialDocument(c *gin.Context) {
	userCtx := middleware.GetUserContext(c)
	if userCtx == nil {
		c.JSON(http.StatusUnauthorized, ErrorResponse{
			Error:   "unauthorized",
			Message: "User not authenticated",
		})
		return
	}

	documentID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "invalid_document_id",
			Message: "Invalid document ID format",
		})
		return
	}

	// Trigger financial processing
	err = h.documentService.ProcessFinancialDocument(c.Request.Context(), documentID, userCtx.UserID)
	if err != nil {
		statusCode := http.StatusInternalServerError
		errorCode := "processing_failed"

		switch err {
		case services.ErrDocumentNotFound:
			statusCode = http.StatusNotFound
			errorCode = "document_not_found"
		case services.ErrInvalidDocumentType:
			statusCode = http.StatusBadRequest
			errorCode = "invalid_document_type"
		}

		c.JSON(statusCode, ErrorResponse{
			Error:   errorCode,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusAccepted, gin.H{
		"message": "Financial processing initiated",
		"status":  "processing",
	})
}

// FindDuplicates finds potential duplicate documents
// @Summary Find duplicate documents
// @Description Find potential duplicate documents based on content similarity
// @Tags documents
// @Produce json
// @Param threshold query float64 false "Similarity threshold (0.0-1.0)" default(0.8)
// @Success 200 {array} repositories.DocumentDuplicate
// @Router /api/v1/documents/duplicates [get]
func (h *DocumentHandler) FindDuplicates(c *gin.Context) {
	userCtx := middleware.GetUserContext(c)
	if userCtx == nil {
		c.JSON(http.StatusUnauthorized, ErrorResponse{
			Error:   "unauthorized",
			Message: "User not authenticated",
		})
		return
	}

	threshold := 0.8 // Default threshold
	if t := c.Query("threshold"); t != "" {
		if parsed, err := strconv.ParseFloat(t, 64); err == nil && parsed >= 0 && parsed <= 1 {
			threshold = parsed
		}
	}

	duplicates, err := h.documentService.FindDuplicates(c.Request.Context(), userCtx.TenantID, threshold)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "search_failed",
			Message: "Failed to find duplicates",
			Details: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, duplicates)
}

// GetExpiringDocuments gets documents nearing expiration
// @Summary Get expiring documents
// @Description Get documents that are expiring within specified days
// @Tags documents
// @Produce json
// @Param days query int false "Days until expiration" default(30)
// @Success 200 {array} DocumentResponse
// @Router /api/v1/documents/expiring [get]
func (h *DocumentHandler) GetExpiringDocuments(c *gin.Context) {
	userCtx := middleware.GetUserContext(c)
	if userCtx == nil {
		c.JSON(http.StatusUnauthorized, ErrorResponse{
			Error:   "unauthorized",
			Message: "User not authenticated",
		})
		return
	}

	days := getIntParam(c, "days", 30)

	documents, err := h.documentService.GetExpiringDocuments(c.Request.Context(), userCtx.TenantID, days)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "query_failed",
			Message: "Failed to get expiring documents",
			Details: err.Error(),
		})
		return
	}

	var responses []DocumentResponse
	for _, doc := range documents {
		responses = append(responses, DocumentResponse{
			Document:    &doc,
			Permissions: h.getDocumentPermissions(userCtx, &doc),
		})
	}

	c.JSON(http.StatusOK, responses)
}

// DownloadDocument serves the document file for download
func (h *DocumentHandler) DownloadDocument(c *gin.Context) {
	userCtx := middleware.GetUserContext(c)
	if userCtx == nil {
		c.JSON(http.StatusUnauthorized, ErrorResponse{
			Error:   "unauthorized",
			Message: "User not authenticated",
		})
		return
	}

	documentID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "invalid_document_id",
			Message: "Invalid document ID format",
		})
		return
	}

	// Get document to verify access
	document, err := h.documentService.GetDocument(c.Request.Context(), documentID, userCtx.TenantID, userCtx.UserID)
	if err != nil {
		if err == services.ErrDocumentNotFound {
			c.JSON(http.StatusNotFound, ErrorResponse{
				Error:   "document_not_found",
				Message: "Document not found",
			})
			return
		}

		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "access_error",
			Message: "Failed to access document",
		})
		return
	}

	// Get file from storage service
	fileReader, err := h.documentService.DownloadDocument(c.Request.Context(), documentID, userCtx.TenantID, userCtx.UserID)
	if err != nil {
		if err == services.ErrDocumentNotFound {
			c.JSON(http.StatusNotFound, ErrorResponse{
				Error:   "file_not_found",
				Message: "Document file not found in storage",
			})
			return
		}

		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "download_error",
			Message: "Failed to download document",
			Details: err.Error(),
		})
		return
	}
	defer fileReader.Close()

	// Set headers for download
	c.Header("Content-Disposition", `attachment; filename="`+document.OriginalName+`"`)
	c.Header("Content-Type", document.ContentType)
	c.Header("Content-Length", strconv.FormatInt(document.FileSize, 10))
	c.Header("Cache-Control", "private, no-cache, no-store, must-revalidate")
	c.Header("Pragma", "no-cache")
	c.Header("Expires", "0")

	// Stream file content to client
	_, err = io.Copy(c.Writer, fileReader)
	if err != nil {
		// Can't send JSON error after headers are sent, just log
		// TODO: Add proper logging when available
		return
	}
}

// PreviewDocument serves a preview of the document
func (h *DocumentHandler) PreviewDocument(c *gin.Context) {
	userCtx := middleware.GetUserContext(c)
	if userCtx == nil {
		c.JSON(http.StatusUnauthorized, ErrorResponse{
			Error:   "unauthorized",
			Message: "User not authenticated",
		})
		return
	}

	documentID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "invalid_document_id",
			Message: "Invalid document ID format",
		})
		return
	}

	// Get document to verify access
	document, err := h.documentService.GetDocument(c.Request.Context(), documentID, userCtx.TenantID, userCtx.UserID)
	if err != nil {
		if err == services.ErrDocumentNotFound {
			c.JSON(http.StatusNotFound, ErrorResponse{
				Error:   "document_not_found",
				Message: "Document not found",
			})
			return
		}

		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "access_error",
			Message: "Failed to access document",
		})
		return
	}

	// Get preview from document service
	previewReader, contentType, err := h.documentService.GetDocumentPreview(c.Request.Context(), documentID, userCtx.TenantID, userCtx.UserID)
	if err != nil {
		if err == services.ErrDocumentNotFound {
			c.JSON(http.StatusNotFound, ErrorResponse{
				Error:   "preview_not_found",
				Message: "Document preview not available",
			})
			return
		}

		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "preview_error",
			Message: "Failed to generate preview",
			Details: err.Error(),
		})
		return
	}
	defer previewReader.Close()

	// Set headers for preview
	c.Header("Content-Type", contentType)
	c.Header("Cache-Control", "public, max-age=3600") // Cache for 1 hour
	c.Header("Content-Disposition", `inline; filename="preview_`+document.OriginalName+`"`)

	// Stream preview content to client
	_, err = io.Copy(c.Writer, previewReader)
	if err != nil {
		// Can't send JSON error after headers are sent, just log
		return
	}
}

// Helper methods

func (h *DocumentHandler) getDocumentPermissions(userCtx *middleware.UserContext, document *models.Document) map[string]bool {
	permissions := map[string]bool{
		"read":   true, // User can access document, so they can read
		"update": false,
		"delete": false,
		"share":  false,
	}

	// Check if user is owner or has admin role
	if document.CreatedBy == userCtx.UserID || userCtx.Role == models.UserRoleAdmin {
		permissions["update"] = true
		permissions["delete"] = true
		permissions["share"] = true
	} else if userCtx.Role == models.UserRoleManager {
		permissions["update"] = true
		permissions["share"] = true
	}

	return permissions
}
