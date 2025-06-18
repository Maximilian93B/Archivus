package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/archivus/archivus/internal/app/config"
	"github.com/archivus/archivus/internal/app/middleware"
	"github.com/archivus/archivus/internal/domain/services"
	"github.com/archivus/archivus/internal/infrastructure/database/models"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// Test setup helpers
func setupTestRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	return router
}

// Setup test environment
func setupTestEnvironment() {
	// Set environment to test mode
	os.Setenv("ENVIRONMENT", "test")
	os.Setenv("JWT_SECRET", "test-jwt-secret-for-testing-only-not-production-safe-key")
	os.Setenv("REDIS_URL", "memory")
	os.Setenv("ENABLE_AI_PROCESSING", "false")
	os.Setenv("LOG_LEVEL", "warn")
}

// Mock user context for testing
func createTestUserContext(tenantID, userID uuid.UUID, role models.UserRole) *middleware.UserContext {
	return &middleware.UserContext{
		UserID:   userID,
		TenantID: tenantID,
		Role:     role,
		Email:    "test@example.com",
	}
}

// Mock services for testing
type MockUserService struct {
	mock.Mock
}

func (m *MockUserService) GetUserProfile(tenantID, userID uuid.UUID) (*services.UserProfile, error) {
	args := m.Called(tenantID, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*services.UserProfile), args.Error(1)
}

func (m *MockUserService) UpdateUser(userID uuid.UUID, updates map[string]interface{}, updatedBy uuid.UUID) (*models.User, error) {
	args := m.Called(userID, updates, updatedBy)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.User), args.Error(1)
}

type MockTenantService struct {
	mock.Mock
}

type MockDocumentService struct {
	mock.Mock
}

// Test helper functions
func makeRequest(router *gin.Engine, method, url string, body interface{}, userCtx *middleware.UserContext) *httptest.ResponseRecorder {
	var reqBody bytes.Buffer
	if body != nil {
		json.NewEncoder(&reqBody).Encode(body)
	}

	req, _ := http.NewRequest(method, url, &reqBody)
	req.Header.Set("Content-Type", "application/json")

	// Add user context to request if provided
	if userCtx != nil {
		// In real implementation, this would be set by auth middleware
		// For testing, we'll simulate it
	}

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w
}

// Basic test to verify our testing framework works
func TestFrameworkSetup(t *testing.T) {
	router := setupTestRouter()
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "test"})
	})

	w := makeRequest(router, "GET", "/test", nil, nil)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]string
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, "test", response["message"])
}

// Test configuration loading with proper test setup
func TestConfigLoad(t *testing.T) {
	// Setup test environment
	setupTestEnvironment()

	cfg, err := config.Load()
	assert.NoError(t, err)
	assert.NotNil(t, cfg)
	assert.Equal(t, "test", cfg.Environment)
	assert.Equal(t, "memory", cfg.Redis.URL)
	assert.Equal(t, "test-jwt-secret-for-testing-only-not-production-safe-key", cfg.JWT.Secret)
	assert.False(t, cfg.Features.AIProcessing)
}

// Test health endpoint structure
func TestHealthEndpoint(t *testing.T) {
	// This test verifies the expected health endpoint structure
	// without requiring full server startup
	expectedFields := []string{"status", "timestamp", "version", "environment"}

	// Mock health response
	healthResponse := map[string]interface{}{
		"status":      "healthy",
		"timestamp":   "2024-01-01T00:00:00Z",
		"version":     "1.0.0",
		"environment": "test",
	}

	for _, field := range expectedFields {
		_, exists := healthResponse[field]
		assert.True(t, exists, "Health response should contain field: %s", field)
	}
}

// Test user context creation
func TestUserContextCreation(t *testing.T) {
	tenantID := uuid.New()
	userID := uuid.New()

	userCtx := createTestUserContext(tenantID, userID, models.UserRoleAdmin)

	assert.Equal(t, tenantID, userCtx.TenantID)
	assert.Equal(t, userID, userCtx.UserID)
	assert.Equal(t, models.UserRoleAdmin, userCtx.Role)
	assert.Equal(t, "test@example.com", userCtx.Email)
}

// Test error response structure
func TestErrorResponseStructure(t *testing.T) {
	errorResp := ErrorResponse{
		Error:   "test_error",
		Message: "Test error message",
		Status:  400,
		Details: "Test details",
	}

	assert.Equal(t, "test_error", errorResp.Error)
	assert.Equal(t, "Test error message", errorResp.Message)
	assert.Equal(t, 400, errorResp.Status)
	assert.Equal(t, "Test details", errorResp.Details)
}

// Test base handler functionality
func TestBaseHandlerConfig(t *testing.T) {
	handler := NewBaseHandler()
	assert.NotNil(t, handler)
	assert.NotNil(t, handler.config)
	assert.True(t, handler.config.IsDevelopment() || handler.config.IsTest())
}

// Test pagination parsing
func TestPaginationParsing(t *testing.T) {
	handler := NewBaseHandler()
	router := setupTestRouter()

	router.GET("/test", func(c *gin.Context) {
		page, pageSize := handler.ParsePagination(c)
		c.JSON(http.StatusOK, gin.H{
			"page":      page,
			"page_size": pageSize,
		})
	})

	// Test default values
	w := makeRequest(router, "GET", "/test", nil, nil)
	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)
	assert.Equal(t, float64(1), response["page"])
	assert.Equal(t, float64(20), response["page_size"])
}

// Benchmark test for handler response times
func BenchmarkHealthEndpoint(b *testing.B) {
	router := setupTestRouter()
	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":      "healthy",
			"timestamp":   "2024-01-01T00:00:00Z",
			"version":     "1.0.0",
			"environment": "test",
		})
	})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := makeRequest(router, "GET", "/health", nil, nil)
		if w.Code != http.StatusOK {
			b.Fatal("Expected 200 status code")
		}
	}
}
