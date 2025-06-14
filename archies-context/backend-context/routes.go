// routes.go - API Routes Reference
// This file serves as a reference for the API structure
package reference

/*
import (
	"github.com/gin-gonic/gin"
)

func SetupRoutes(r *gin.Engine) {
	// API v1 group
	v1 := r.Group("/api/v1")

	// Apply rate limiting middleware
	// v1.Use(RateLimitMiddleware())

	// Public routes (no auth required)
	public := v1.Group("")
	{
		// Health check
		// public.GET("/health", HealthCheck)
		// public.GET("/status", SystemStatus)

		// Authentication
		// public.POST("/auth/login", Login)
		// public.POST("/auth/register", Register)
		// public.POST("/auth/forgot-password", ForgotPassword)
		// public.POST("/auth/reset-password", ResetPassword)
		// public.GET("/auth/verify-email/:token", VerifyEmail)
	}

	// Protected routes (auth required)
	protected := v1.Group("")
	// protected.Use(AuthMiddleware())
	{
		// User Management
		// protected.GET("/users/profile", GetUserProfile)
		// protected.PUT("/users/profile", UpdateUserProfile)
		// protected.POST("/users/change-password", ChangePassword)
		// protected.POST("/users/enable-mfa", EnableMFA)
		// protected.POST("/users/verify-mfa", VerifyMFA)
		// protected.POST("/auth/logout", Logout)
		// protected.POST("/auth/refresh", RefreshToken)

		// Document Management
		// protected.POST("/documents", UploadDocument)
		// protected.GET("/documents", ListDocuments)
		// protected.GET("/documents/:id", GetDocument)
		// protected.PUT("/documents/:id", UpdateDocument)
		// protected.DELETE("/documents/:id", DeleteDocument)
		// protected.GET("/documents/:id/content", DownloadDocument)
		// protected.GET("/documents/:id/preview", PreviewDocument)
		// protected.POST("/documents/:id/versions", CreateDocumentVersion)
		// protected.GET("/documents/:id/versions", ListDocumentVersions)
		// protected.GET("/documents/:id/versions/:version", GetDocumentVersion)
		// protected.POST("/documents/bulk", BulkUploadDocuments)
		// protected.DELETE("/documents/bulk", BulkDeleteDocuments)

		// Search & Discovery
		// protected.GET("/search", SearchDocuments)
		// protected.POST("/search/semantic", SemanticSearch)
		// protected.GET("/search/suggestions", GetSearchSuggestions)
		// protected.GET("/search/filters", GetSearchFilters)

		// Folders
		// protected.POST("/folders", CreateFolder)
		// protected.GET("/folders", ListFolders)
		// protected.GET("/folders/:id", GetFolder)
		// protected.PUT("/folders/:id", UpdateFolder)
		// protected.DELETE("/folders/:id", DeleteFolder)
		// protected.GET("/folders/:id/tree", GetFolderTree)
		// protected.POST("/folders/:id/move", MoveFolder)

		// Categories
		// protected.POST("/categories", CreateCategory)
		// protected.GET("/categories", ListCategories)
		// protected.GET("/categories/:id", GetCategory)
		// protected.PUT("/categories/:id", UpdateCategory)
		// protected.DELETE("/categories/:id", DeleteCategory)

		// Tags
		// protected.POST("/tags", CreateTag)
		// protected.GET("/tags", ListTags)
		// protected.GET("/tags/:id", GetTag)
		// protected.PUT("/tags/:id", UpdateTag)
		// protected.DELETE("/tags/:id", DeleteTag)
		// protected.GET("/tags/suggestions", GetTagSuggestions)

		// AI Services
		// protected.POST("/ai/analyze", AnalyzeDocument)
		// protected.POST("/ai/summarize", SummarizeDocument)
		// protected.POST("/ai/extract-entities", ExtractEntities)
		// protected.POST("/ai/classify", ClassifyDocument)
		// protected.GET("/ai/suggestions/:document_id", GetAISuggestions)
		// protected.POST("/ai/bulk-process", BulkAIProcess)
		// protected.GET("/ai/jobs", ListAIJobs)
		// protected.GET("/ai/jobs/:id", GetAIJobStatus)

		// Sharing
		// protected.POST("/shares", CreateShare)
		// protected.GET("/shares", ListShares)
		// protected.GET("/shares/:id", GetShare)
		// protected.PUT("/shares/:id", UpdateShare)
		// protected.DELETE("/shares/:id", RevokeShare)
		// protected.GET("/shares/sent", ListSentShares)
		// protected.GET("/shares/received", ListReceivedShares)

		// Analytics & Reports
		// protected.GET("/analytics/dashboard", GetDashboardAnalytics)
		// protected.GET("/analytics/usage", GetUsageAnalytics)
		// protected.GET("/analytics/documents", GetDocumentAnalytics)
		// protected.GET("/analytics/search", GetSearchAnalytics)
		// protected.GET("/analytics/export", ExportAnalytics)

		// Audit Logs
		// protected.GET("/audit", ListAuditLogs)
		// protected.GET("/audit/export", ExportAuditLogs)

		// Webhooks
		// protected.POST("/webhooks", CreateWebhook)
		// protected.GET("/webhooks", ListWebhooks)
		// protected.GET("/webhooks/:id", GetWebhook)
		// protected.PUT("/webhooks/:id", UpdateWebhook)
		// protected.DELETE("/webhooks/:id", DeleteWebhook)
		// protected.POST("/webhooks/:id/test", TestWebhook)

		// API Keys
		// protected.POST("/api-keys", CreateAPIKey)
		// protected.GET("/api-keys", ListAPIKeys)
		// protected.GET("/api-keys/:id", GetAPIKey)
		// protected.PUT("/api-keys/:id", UpdateAPIKey)
		// protected.DELETE("/api-keys/:id", RevokeAPIKey)
	}

	// Admin routes (admin role required)
	admin := v1.Group("")
	// admin.Use(AuthMiddleware(), AdminMiddleware())
	{
		// Tenant Management
		// admin.POST("/tenants", CreateTenant)
		// admin.GET("/tenants", ListTenants)
		// admin.GET("/tenants/:id", GetTenant)
		// admin.PUT("/tenants/:id", UpdateTenant)
		// admin.DELETE("/tenants/:id", DeleteTenant)
		// admin.PUT("/tenants/:id/activate", ActivateTenant)
		// admin.PUT("/tenants/:id/deactivate", DeactivateTenant)

		// User Management (Admin)
		// admin.GET("/users", ListUsers)
		// admin.GET("/users/:id", GetUser)
		// admin.POST("/users", CreateUser)
		// admin.PUT("/users/:id", UpdateUser)
		// admin.DELETE("/users/:id", DeleteUser)
		// admin.PUT("/users/:id/activate", ActivateUser)
		// admin.PUT("/users/:id/deactivate", DeactivateUser)
		// admin.PUT("/users/:id/role", UpdateUserRole)

		// Retention Policies
		// admin.POST("/retention-policies", CreateRetentionPolicy)
		// admin.GET("/retention-policies", ListRetentionPolicies)
		// admin.GET("/retention-policies/:id", GetRetentionPolicy)
		// admin.PUT("/retention-policies/:id", UpdateRetentionPolicy)
		// admin.DELETE("/retention-policies/:id", DeleteRetentionPolicy)

		// System Configuration
		// admin.GET("/config", GetSystemConfig)
		// admin.PUT("/config", UpdateSystemConfig)
		// admin.GET("/metrics", GetSystemMetrics)
		// admin.POST("/maintenance/cleanup", RunCleanup)
		// admin.POST("/maintenance/reindex", ReindexDocuments)
	}

	// Public share access (token-based auth)
	shareAccess := v1.Group("/shared")
	{
		// shareAccess.GET("/:token", AccessSharedDocument)
		// shareAccess.GET("/:token/download", DownloadSharedDocument)
	}
}
*/

// API Routes Reference:
//
// Public Routes:
// GET  /api/v1/health
// GET  /api/v1/status
// POST /api/v1/auth/login
// POST /api/v1/auth/register
// POST /api/v1/auth/forgot-password
// POST /api/v1/auth/reset-password
// GET  /api/v1/auth/verify-email/:token
//
// Protected Routes (Auth Required):
// User Management:
// GET  /api/v1/users/profile
// PUT  /api/v1/users/profile
// POST /api/v1/users/change-password
// POST /api/v1/users/enable-mfa
// POST /api/v1/users/verify-mfa
// POST /api/v1/auth/logout
// POST /api/v1/auth/refresh
//
// Document Management:
// POST   /api/v1/documents
// GET    /api/v1/documents
// GET    /api/v1/documents/:id
// PUT    /api/v1/documents/:id
// DELETE /api/v1/documents/:id
// GET    /api/v1/documents/:id/content
// GET    /api/v1/documents/:id/preview
// POST   /api/v1/documents/:id/versions
// GET    /api/v1/documents/:id/versions
// GET    /api/v1/documents/:id/versions/:version
// POST   /api/v1/documents/bulk
// DELETE /api/v1/documents/bulk
//
// Search & Discovery:
// GET  /api/v1/search
// POST /api/v1/search/semantic
// GET  /api/v1/search/suggestions
// GET  /api/v1/search/filters
//
// Folders:
// POST   /api/v1/folders
// GET    /api/v1/folders
// GET    /api/v1/folders/:id
// PUT    /api/v1/folders/:id
// DELETE /api/v1/folders/:id
// GET    /api/v1/folders/:id/tree
// POST   /api/v1/folders/:id/move
//
// Categories:
// POST   /api/v1/categories
// GET    /api/v1/categories
// GET    /api/v1/categories/:id
// PUT    /api/v1/categories/:id
// DELETE /api/v1/categories/:id
//
// Tags:
// POST   /api/v1/tags
// GET    /api/v1/tags
// GET    /api/v1/tags/:id
// PUT    /api/v1/tags/:id
// DELETE /api/v1/tags/:id
// GET    /api/v1/tags/suggestions
//
// AI Services:
// POST /api/v1/ai/analyze
// POST /api/v1/ai/summarize
// POST /api/v1/ai/extract-entities
// POST /api/v1/ai/classify
// GET  /api/v1/ai/suggestions/:document_id
// POST /api/v1/ai/bulk-process
// GET  /api/v1/ai/jobs
// GET  /api/v1/ai/jobs/:id
//
// Sharing:
// POST   /api/v1/shares
// GET    /api/v1/shares
// GET    /api/v1/shares/:id
// PUT    /api/v1/shares/:id
// DELETE /api/v1/shares/:id
// GET    /api/v1/shares/sent
// GET    /api/v1/shares/received
//
// Analytics & Reports:
// GET /api/v1/analytics/dashboard
// GET /api/v1/analytics/usage
// GET /api/v1/analytics/documents
// GET /api/v1/analytics/search
// GET /api/v1/analytics/export
//
// Audit Logs:
// GET /api/v1/audit
// GET /api/v1/audit/export
//
// Webhooks:
// POST   /api/v1/webhooks
// GET    /api/v1/webhooks
// GET    /api/v1/webhooks/:id
// PUT    /api/v1/webhooks/:id
// DELETE /api/v1/webhooks/:id
// POST   /api/v1/webhooks/:id/test
//
// API Keys:
// POST   /api/v1/api-keys
// GET    /api/v1/api-keys
// GET    /api/v1/api-keys/:id
// PUT    /api/v1/api-keys/:id
// DELETE /api/v1/api-keys/:id
//
// Admin Routes (Admin Role Required):
// Tenant Management:
// POST   /api/v1/tenants
// GET    /api/v1/tenants
// GET    /api/v1/tenants/:id
// PUT    /api/v1/tenants/:id
// DELETE /api/v1/tenants/:id
// PUT    /api/v1/tenants/:id/activate
// PUT    /api/v1/tenants/:id/deactivate
//
// User Management (Admin):
// GET    /api/v1/users
// GET    /api/v1/users/:id
// POST   /api/v1/users
// PUT    /api/v1/users/:id
// DELETE /api/v1/users/:id
// PUT    /api/v1/users/:id/activate
// PUT    /api/v1/users/:id/deactivate
// PUT    /api/v1/users/:id/role
//
// Retention Policies:
// POST   /api/v1/retention-policies
// GET    /api/v1/retention-policies
// GET    /api/v1/retention-policies/:id
// PUT    /api/v1/retention-policies/:id
// DELETE /api/v1/retention-policies/:id
//
// System Configuration:
// GET  /api/v1/config
// PUT  /api/v1/config
// GET  /api/v1/metrics
// POST /api/v1/maintenance/cleanup
// POST /api/v1/maintenance/reindex
//
// Public Share Access:
// GET /api/v1/shared/:token
// GET /api/v1/shared/:token/download
