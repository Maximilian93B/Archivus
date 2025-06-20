package main

import (
	"fmt"
	"os"
	"time"

	"github.com/archivus/archivus/internal/app/config"
	"github.com/archivus/archivus/internal/app/server"
	appservices "github.com/archivus/archivus/internal/app/services"
	"github.com/archivus/archivus/internal/domain/services"
	"github.com/archivus/archivus/internal/infrastructure/auth/supabase"
	"github.com/archivus/archivus/internal/infrastructure/database"
	"github.com/archivus/archivus/internal/infrastructure/database/models"
	"github.com/archivus/archivus/internal/infrastructure/repositories/postgresql"
	"github.com/archivus/archivus/internal/infrastructure/storage/local"
	"github.com/archivus/archivus/pkg/logger"
)

func main() {
	// Initialize logger
	log := logger.New()

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Error("Failed to load configuration", "error", err)
		os.Exit(1)
	}

	log.Info("Starting Archivus DMS", "environment", cfg.Environment, "port", cfg.Server.Port)

	// Initialize database and repositories
	db, err := initializeDatabase(cfg, log)
	if err != nil {
		log.Error("Failed to initialize database", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	// Initialize service manager with Redis
	serviceManager, err := appservices.NewServiceManager(cfg, db)
	if err != nil {
		log.Error("Failed to initialize service manager", "error", err)
		os.Exit(1)
	}
	defer serviceManager.Close()

	log.Info("Service manager initialized successfully with Redis",
		"redis_url", cfg.Redis.URL,
		"database_initialized", true)

	// Health check to verify all services are working
	if err := serviceManager.HealthCheck(); err != nil {
		log.Error("Service health check failed", "error", err)
		os.Exit(1)
	}
	log.Info("All services healthy - Database and Redis connected")

	repos := serviceManager.Repositories
	log.Info("Database and repositories initialized successfully", "repository_count", 13)

	// Initialize external services
	storageService := initializeStorageService(cfg, log)
	authService := initializeAuthService(cfg, log)

	// Initialize business services with Redis cache
	businessServices := initializeBusinessServices(repos, storageService, authService, cfg, serviceManager.CacheService, log)

	// Create HTTP server
	srv := server.NewServer(cfg, businessServices, log)

	// Check for TLS configuration and start appropriate server
	certFile := os.Getenv("TLS_CERT_FILE")
	keyFile := os.Getenv("TLS_KEY_FILE")

	if cfg.IsProduction() && certFile != "" && keyFile != "" {
		// Production HTTPS
		log.Info("Starting HTTPS server", "cert", certFile)
		if err := srv.StartTLS(certFile, keyFile); err != nil {
			log.Error("HTTPS server failed", "error", err)
			os.Exit(1)
		}
	} else {
		// Development HTTP or production with reverse proxy
		if cfg.IsProduction() {
			log.Info("Starting HTTP server (ensure reverse proxy handles TLS)")
		} else {
			log.Info("Starting HTTP server in development mode")
		}

		if err := srv.Start(); err != nil {
			log.Error("HTTP server failed", "error", err)
			os.Exit(1)
		}
	}
}

// Database initialization function
func initializeDatabase(cfg *config.Config, log *logger.Logger) (*database.DB, error) {
	databaseURL := cfg.GetDatabaseURL()
	if databaseURL == "" && cfg.IsDevelopment() {
		// Use SQLite for development if no PostgreSQL URL provided
		databaseURL = "file:./archivus.db?cache=shared&_fk=1"
		log.Info("Using SQLite database for development", "path", "./archivus.db")
	}

	if databaseURL == "" {
		return nil, fmt.Errorf("database URL is required")
	}

	log.Info("Connecting to database", "url", databaseURL)
	db, err := database.New(databaseURL)
	if err != nil {
		return nil, err
	}

	// Test the connection
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	// Run auto-migrations
	log.Info("Running database migrations...")
	if err := db.AutoMigrate(models.GetAllModels()...); err != nil {
		return nil, fmt.Errorf("failed to run migrations: %w", err)
	}

	log.Info("Database initialized successfully")
	return db, nil
}

// Repository initialization function
func initializeRepositories(db *database.DB, log *logger.Logger) *postgresql.Repositories {
	log.Info("Initializing all 13 repositories...")
	repos := postgresql.NewRepositories(db)
	log.Info("All repositories initialized successfully",
		"tenant", repos.TenantRepo != nil,
		"user", repos.UserRepo != nil,
		"document", repos.DocumentRepo != nil,
		"folder", repos.FolderRepo != nil,
		"tag", repos.TagRepo != nil,
		"category", repos.CategoryRepo != nil,
		"workflow", repos.WorkflowRepo != nil,
		"workflow_task", repos.WorkflowTaskRepo != nil,
		"ai_job", repos.AIJobRepo != nil,
		"audit", repos.AuditRepo != nil,
		"share", repos.ShareRepo != nil,
		"analytics", repos.AnalyticsRepo != nil,
		"notification", repos.NotificationRepo != nil,
	)
	return repos
}

// Storage service initialization
func initializeStorageService(cfg *config.Config, log *logger.Logger) *local.StorageService {
	log.Info("Initializing local storage service", "path", cfg.Storage.Path)
	return local.NewStorageService(cfg.Storage.Path)
}

// Auth service initialization
func initializeAuthService(cfg *config.Config, log *logger.Logger) *supabase.AuthService {
	log.Info("Initializing Supabase auth service")

	if cfg.Supabase.URL == "" || cfg.Supabase.APIKey == "" {
		log.Error("Supabase credentials are required for authentication")
		os.Exit(1)
	}

	authService, err := supabase.NewAuthService(supabase.Config{
		URL:        cfg.Supabase.URL,
		APIKey:     cfg.Supabase.APIKey,
		ServiceKey: cfg.Supabase.ServiceKey,
		JWTSecret:  cfg.Supabase.JWTSecret,
	})
	if err != nil {
		log.Error("Failed to initialize auth service", "error", err)
		os.Exit(1)
	}

	log.Info("Supabase auth service initialized successfully")
	return authService
}

// Business services initialization - THE BIG ONE!
func initializeBusinessServices(
	repos *postgresql.Repositories,
	storageService *local.StorageService,
	authService *supabase.AuthService,
	cfg *config.Config,
	cacheService services.CacheService,
	log *logger.Logger,
) *server.Services {
	log.Info("Initializing business services with complete repository wiring...")

	// Configure UserService
	userServiceConfig := services.UserServiceConfig{
		MinPasswordLength:        8,
		RequireUppercase:         true,
		RequireLowercase:         true,
		RequireNumbers:           true,
		RequireSpecialChars:      false,
		PasswordExpiryDays:       90,
		MaxLoginAttempts:         5,
		LockoutDurationMins:      30,
		RequireEmailVerification: true,
		EnableMFA:                false,
	}

	// Configure TenantService
	tenantServiceConfig := services.TenantServiceConfig{
		DefaultTrialDays:      30,
		MinSubdomainLength:    3,
		MaxSubdomainLength:    20,
		ReservedSubdomains:    []string{"api", "www", "admin", "support", "mail", "ftp"},
		DefaultStorageQuota:   5 * 1024 * 1024 * 1024, // 5GB
		DefaultAPIQuota:       1000,
		RequireBusinessInfo:   false,
		EnableCompliance:      true,
		SupportedIndustries:   []string{"technology", "finance", "healthcare", "legal", "manufacturing", "retail"},
		SupportedCompanySizes: []string{"1-10", "11-50", "51-200", "201-500", "500+"},
	}

	// Configure DocumentService
	documentServiceConfig := services.DocumentServiceConfig{
		MaxFileSize:            cfg.Limits.MaxFileSize,
		AllowedMimeTypes:       []string{"application/pdf", "image/", "text/", "application/msword", "application/vnd.openxmlformats"},
		StorageBasePath:        cfg.Storage.Path,
		ThumbnailPath:          cfg.Storage.Path + "/thumbnails",
		PreviewPath:            cfg.Storage.Path + "/previews",
		EnableAIProcessing:     cfg.Features.AIProcessing,
		EnableDuplicateCheck:   true,
		AutoGenerateThumbnails: true,
	}

	// Initialize UserService with full dependencies
	userService := services.NewUserService(
		repos.UserRepo,
		repos.TenantRepo,
		repos.AuditRepo,
		authService,
		nil, // emailService - will be implemented in Phase 3
		userServiceConfig,
		cacheService,
	)

	// Initialize TenantService with full dependencies (including UserService)
	tenantService := services.NewTenantService(
		repos.TenantRepo,
		repos.UserRepo,
		repos.DocumentRepo,
		repos.AuditRepo,
		userService, // UserService for proper admin user creation
		nil,         // subscriptionService - will be implemented in Phase 4
		tenantServiceConfig,
	)

	// Initialize DocumentService with ALL 9 repositories + external services
	documentService := services.NewDocumentService(
		repos.DocumentRepo,  // docRepo
		repos.TenantRepo,    // tenantRepo
		repos.UserRepo,      // userRepo
		repos.FolderRepo,    // folderRepo
		repos.TagRepo,       // tagRepo
		repos.CategoryRepo,  // categoryRepo
		repos.AuditRepo,     // auditRepo
		repos.AIJobRepo,     // aiJobRepo
		repos.AnalyticsRepo, // analyticsRepo
		storageService,      // storageService
		nil,                 // aiService - will be implemented in Phase 3
		documentServiceConfig,
	)

	// Initialize WorkflowService with correct dependencies
	workflowService := services.NewWorkflowService(
		repos.WorkflowRepo,     // workflowRepo
		repos.WorkflowTaskRepo, // taskRepo
		repos.DocumentRepo,     // documentRepo
		repos.UserRepo,         // userRepo
		repos.TenantRepo,       // tenantRepo
		repos.AuditRepo,        // auditRepo
		repos.NotificationRepo, // notificationRepo
		nil,                    // notificationService - will be implemented in Phase 4
	)

	// AnalyticsService configuration with correct fields
	analyticsServiceConfig := services.AnalyticsServiceConfig{
		DefaultCacheTTL:       time.Hour,
		MaxDataPointsPerChart: 100,
		EnableRealTimeUpdates: false,
		RetentionDays:         365,
	}

	// Initialize AnalyticsService with correct signature
	analyticsService := services.NewAnalyticsService(
		repos.AnalyticsRepo, // analyticsRepo
		repos.DocumentRepo,  // documentRepo
		repos.UserRepo,      // userRepo
		repos.TenantRepo,    // tenantRepo
		repos.AuditRepo,     // auditRepo
		analyticsServiceConfig,
	)

	log.Info("🎉 Business services initialized successfully!",
		"user_service", userService != nil,
		"tenant_service", tenantService != nil,
		"document_service", documentService != nil,
		"workflow_service", workflowService != nil,
		"analytics_service", analyticsService != nil,
	)

	return &server.Services{
		UserService:      userService,
		TenantService:    tenantService,
		DocumentService:  documentService,
		WorkflowService:  workflowService,
		AIService:        nil, // Will be implemented in Phase 3
		AnalyticsService: analyticsService,
		AuthService:      authService, // Fixed: Pass the auth service
	}
}
