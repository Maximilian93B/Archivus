package handlers

import (
	"net/http"

	"github.com/archivus/archivus/internal/app/middleware"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// BaseHandler provides common functionality for all handlers
type BaseHandler struct {
	config *HandlerConfig
}

// NewBaseHandler creates a new base handler
func NewBaseHandler() *BaseHandler {
	return &BaseHandler{
		config: NewHandlerConfig(),
	}
}

// AuthenticateUser extracts and validates user context
func (b *BaseHandler) AuthenticateUser(c *gin.Context) (*middleware.UserContext, bool) {
	userCtx := getUserContextFromGin(c)
	if userCtx == nil {
		b.RespondUnauthorized(c, "User authentication required")
		return nil, false
	}
	return userCtx, true
}

// RespondError sends a standardized error response
func (b *BaseHandler) RespondError(c *gin.Context, statusCode int, errorCode, message string, details ...string) {
	response := ErrorResponse{
		Error:   errorCode,
		Message: message,
		Status:  statusCode,
	}

	// Include details based on environment
	if len(details) > 0 && b.config.EnableDebugErrors {
		response.Details = details[0]
	}

	c.JSON(statusCode, response)
}

// RespondUnauthorized sends a standardized unauthorized response
func (b *BaseHandler) RespondUnauthorized(c *gin.Context, message string) {
	b.RespondError(c, http.StatusUnauthorized, "unauthorized", message)
}

// RespondBadRequest sends a standardized bad request response
func (b *BaseHandler) RespondBadRequest(c *gin.Context, message string, details ...string) {
	b.RespondError(c, http.StatusBadRequest, "invalid_request", message, details...)
}

// RespondNotFound sends a standardized not found response
func (b *BaseHandler) RespondNotFound(c *gin.Context, message string) {
	b.RespondError(c, http.StatusNotFound, "not_found", message)
}

// RespondConflict sends a standardized conflict response
func (b *BaseHandler) RespondConflict(c *gin.Context, message string) {
	b.RespondError(c, http.StatusConflict, "conflict", message)
}

// RespondInternalError sends a standardized internal server error response
func (b *BaseHandler) RespondInternalError(c *gin.Context, message string, details ...string) {
	b.RespondError(c, http.StatusInternalServerError, "internal_error", message, details...)
}

// RespondSuccess sends a standardized success response
func (b *BaseHandler) RespondSuccess(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, data)
}

// RespondCreated sends a standardized created response
func (b *BaseHandler) RespondCreated(c *gin.Context, data interface{}) {
	c.JSON(http.StatusCreated, data)
}

// ParsePagination extracts and validates pagination parameters
func (b *BaseHandler) ParsePagination(c *gin.Context) (page, pageSize int) {
	page = getIntParam(c, "page", 1)
	pageSize = getIntParam(c, "per_page", b.config.DefaultPageSize)

	if page < 1 {
		page = 1
	}
	pageSize = b.config.ValidatePageSize(pageSize)

	return page, pageSize
}

// ParseSorting extracts and validates sorting parameters
func (b *BaseHandler) ParseSorting(c *gin.Context, defaultSortBy string) (sortBy string, sortDesc bool) {
	sortBy = c.DefaultQuery("sort_by", defaultSortBy)
	sortDesc = c.DefaultQuery("sort_desc", "true") == "true"
	return sortBy, sortDesc
}

// ValidateUUID validates UUID parameter and responds with error if invalid
func (b *BaseHandler) ValidateUUID(c *gin.Context, paramName, uuidStr string) (uuid.UUID, bool) {
	id, err := uuid.Parse(uuidStr)
	if err != nil {
		b.RespondBadRequest(c, "Invalid "+paramName+" format")
		return uuid.Nil, false
	}
	return id, true
}

// ValidateTenantAccess checks if user has access to the tenant
func (b *BaseHandler) ValidateTenantAccess(c *gin.Context, userCtx *middleware.UserContext, resourceTenantID uuid.UUID) bool {
	if userCtx.TenantID != resourceTenantID {
		b.RespondError(c, http.StatusForbidden, "access_denied", "Access denied to resource")
		return false
	}
	return true
}
