package handlers

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/archivus/archivus/internal/domain/dto"
	"github.com/archivus/archivus/internal/infrastructure/database/models"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// UserContext represents the authenticated user context
type UserContext struct {
	UserID   uuid.UUID       `json:"user_id"`
	TenantID uuid.UUID       `json:"tenant_id"`
	Email    string          `json:"email"`
	Role     models.UserRole `json:"role"`
	IsActive bool            `json:"is_active"`
}

// AuthTokenClaims represents JWT token claims
type AuthTokenClaims struct {
	UserID   uuid.UUID `json:"user_id"`
	TenantID uuid.UUID `json:"tenant_id"`
	Email    string    `json:"email"`
	Role     string    `json:"role"`
	IsActive bool      `json:"is_active"`
}

// Context keys for middleware
const (
	UserContextKey = "user_context"
	RequestIDKey   = "request_id"
	TenantIDKey    = "tenant_id"
)

// GetUserContext retrieves user context from gin context
func GetUserContext(c *gin.Context) *UserContext {
	if value, exists := c.Get(UserContextKey); exists {
		if userCtx, ok := value.(*UserContext); ok {
			return userCtx
		}
	}
	return nil
}

// SetUserContext sets user context in gin context
func SetUserContext(c *gin.Context, userCtx *UserContext) {
	c.Set(UserContextKey, userCtx)
}

// GetRequestID retrieves request ID from gin context
func GetRequestID(c *gin.Context) string {
	if value, exists := c.Get(RequestIDKey); exists {
		if requestID, ok := value.(string); ok {
			return requestID
		}
	}
	return ""
}

// SetRequestID sets request ID in gin context
func SetRequestID(c *gin.Context, requestID string) {
	c.Set(RequestIDKey, requestID)
}

// GetTenantID retrieves tenant ID from gin context
func GetTenantID(c *gin.Context) *uuid.UUID {
	if value, exists := c.Get(TenantIDKey); exists {
		if tenantID, ok := value.(uuid.UUID); ok {
			return &tenantID
		}
	}
	return nil
}

// SetTenantID sets tenant ID in gin context
func SetTenantID(c *gin.Context, tenantID uuid.UUID) {
	c.Set(TenantIDKey, tenantID)
}

// Helper functions for parsing query parameters

// getIntParam safely parses an integer query parameter with a default value
func getIntParam(c *gin.Context, param string, defaultValue int) int {
	value := c.Query(param)
	if value == "" {
		return defaultValue
	}

	parsed, err := strconv.Atoi(value)
	if err != nil {
		return defaultValue
	}

	return parsed
}

// getFloatParam safely parses a float query parameter with a default value
func getFloatParam(c *gin.Context, param string, defaultValue float64) float64 {
	value := c.Query(param)
	if value == "" {
		return defaultValue
	}

	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return defaultValue
	}

	return parsed
}

// getBoolParam safely parses a boolean query parameter with a default value
func getBoolParam(c *gin.Context, param string, defaultValue bool) bool {
	value := c.Query(param)
	if value == "" {
		return defaultValue
	}

	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return defaultValue
	}

	return parsed
}

// getStringArrayParam safely parses a comma-separated string array parameter
func getStringArrayParam(c *gin.Context, param string) []string {
	value := c.Query(param)
	if value == "" {
		return []string{}
	}

	// Split by comma and clean up
	result := []string{}
	for _, item := range strings.Split(value, ",") {
		item = strings.TrimSpace(item)
		if item != "" {
			result = append(result, item)
		}
	}

	return result
}

// getUUIDParam safely parses a UUID query parameter
func getUUIDParam(c *gin.Context, param string) *uuid.UUID {
	value := c.Query(param)
	if value == "" {
		return nil
	}

	parsed, err := uuid.Parse(value)
	if err != nil {
		return nil
	}

	return &parsed
}

// getUUIDArrayParam safely parses a comma-separated UUID array parameter
func getUUIDArrayParam(c *gin.Context, param string) []uuid.UUID {
	value := c.Query(param)
	if value == "" {
		return []uuid.UUID{}
	}

	result := []uuid.UUID{}
	for _, item := range strings.Split(value, ",") {
		item = strings.TrimSpace(item)
		if item != "" {
			if parsed, err := uuid.Parse(item); err == nil {
				result = append(result, parsed)
			}
		}
	}

	return result
}

// parseDate parses a date string in ISO format
func parseDate(dateStr string) (time.Time, error) {
	// Try parsing different formats
	formats := []string{
		"2006-01-02T15:04:05Z07:00", // RFC3339
		"2006-01-02T15:04:05",       // ISO 8601 without timezone
		"2006-01-02",                // Date only
		"2006-01-02 15:04:05",       // SQL datetime
	}

	for _, format := range formats {
		if parsed, err := time.Parse(format, dateStr); err == nil {
			return parsed, nil
		}
	}

	return time.Time{}, fmt.Errorf("invalid date format: %s", dateStr)
}

// parseDateRange parses date range parameters
func parseDateRange(c *gin.Context, fromParam, toParam string) (from, to *time.Time) {
	if fromStr := c.Query(fromParam); fromStr != "" {
		if parsed, err := parseDate(fromStr); err == nil {
			from = &parsed
		}
	}

	if toStr := c.Query(toParam); toStr != "" {
		if parsed, err := parseDate(toStr); err == nil {
			to = &parsed
		}
	}

	return
}

// Response helpers

// SendError sends a structured error response
func SendError(c *gin.Context, statusCode int, error, message string, details ...string) {
	response := dto.ErrorResponse{
		Error: error,
		Code:  message,
	}

	if len(details) > 0 {
		response.Details = map[string]interface{}{"details": details[0]}
	}

	c.JSON(statusCode, response)
}

// SendSuccess sends a structured success response
func SendSuccess(c *gin.Context, statusCode int, message string, data interface{}) {
	response := dto.SuccessResponse{
		Message: message,
		Data:    data,
	}

	c.JSON(statusCode, response)
}

// SendData sends data without wrapper
func SendData(c *gin.Context, statusCode int, data interface{}) {
	c.JSON(statusCode, data)
}

// Validation helpers

// validateRequired checks if required fields are present
func validateRequired(fields map[string]interface{}) error {
	for field, value := range fields {
		if value == nil {
			return fmt.Errorf("field '%s' is required", field)
		}

		// Check for empty strings
		if str, ok := value.(string); ok && str == "" {
			return fmt.Errorf("field '%s' cannot be empty", field)
		}

		// Check for zero UUIDs
		if id, ok := value.(uuid.UUID); ok && id == uuid.Nil {
			return fmt.Errorf("field '%s' cannot be empty", field)
		}
	}

	return nil
}

// validateUUID validates UUID format
func validateUUID(id string) error {
	if _, err := uuid.Parse(id); err != nil {
		return fmt.Errorf("invalid UUID format: %s", id)
	}
	return nil
}

// validatePositiveInt validates positive integer
func validatePositiveInt(value int, fieldName string) error {
	if value <= 0 {
		return fmt.Errorf("field '%s' must be positive", fieldName)
	}
	return nil
}

// validatePositiveFloat validates positive float
func validatePositiveFloat(value float64, fieldName string) error {
	if value <= 0 {
		return fmt.Errorf("field '%s' must be positive", fieldName)
	}
	return nil
}

// validateEmail validates email format
func validateEmail(email string) error {
	if email == "" {
		return fmt.Errorf("email is required")
	}

	// Simple email validation regex
	emailRegex := regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)
	if !emailRegex.MatchString(email) {
		return fmt.Errorf("invalid email format")
	}

	return nil
}

// validateDateRange validates date range
func validateDateRange(from, to *time.Time) error {
	if from != nil && to != nil && from.After(*to) {
		return fmt.Errorf("start date cannot be after end date")
	}
	return nil
}

// Pagination helpers

// calculatePagination calculates pagination metadata
func calculatePagination(page, pageSize int, total int64) (int, int, int) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100 // Maximum page size
	}

	totalPages := int((total + int64(pageSize) - 1) / int64(pageSize))
	if totalPages == 0 {
		totalPages = 1
	}

	return page, pageSize, totalPages
}

// getPaginationParams extracts pagination parameters from request
func getPaginationParams(c *gin.Context) (page, pageSize int) {
	page = getIntParam(c, "page", 1)
	pageSize = getIntParam(c, "page_size", 20)

	// Validate and constrain values
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}

	return page, pageSize
}

// Security helpers

// sanitizeInput sanitizes user input to prevent XSS
func sanitizeInput(input string) string {
	// Basic HTML escaping
	input = strings.ReplaceAll(input, "<", "&lt;")
	input = strings.ReplaceAll(input, ">", "&gt;")
	input = strings.ReplaceAll(input, "\"", "&quot;")
	input = strings.ReplaceAll(input, "'", "&#x27;")
	input = strings.ReplaceAll(input, "&", "&amp;")
	return input
}

// checkRateLimit checks if request is within rate limits
func checkRateLimit(c *gin.Context, key string, limit int, window time.Duration) bool {
	// TODO: Implement rate limiting using Redis
	// For now, return true (no rate limiting)
	return true
}

// logSecurityEvent logs security-related events
func logSecurityEvent(event, description string, userID *uuid.UUID, ip string) {
	// TODO: Implement security event logging
	// This would log to security audit log
}

// File upload helpers

// validateFileSize validates uploaded file size
func validateFileSize(size int64, maxSize int64) error {
	if size > maxSize {
		return fmt.Errorf("file size %d bytes exceeds maximum allowed size %d bytes", size, maxSize)
	}
	return nil
}

// validateFileType validates uploaded file type
func validateFileType(contentType string, allowedTypes []string) error {
	for _, allowed := range allowedTypes {
		if contentType == allowed {
			return nil
		}
	}
	return fmt.Errorf("file type %s not allowed", contentType)
}

// getFileExtension gets file extension from filename
func getFileExtension(filename string) string {
	ext := filepath.Ext(filename)
	if ext != "" {
		return strings.ToLower(ext[1:]) // Remove the dot and convert to lowercase
	}
	return ""
}
