package main

import (
	"fmt"
	"os"

	"github.com/archivus/archivus/internal/app/config"
	"github.com/archivus/archivus/internal/app/server"
	"github.com/archivus/archivus/internal/infrastructure/database"
	"github.com/archivus/archivus/internal/infrastructure/database/models"
	"github.com/archivus/archivus/internal/infrastructure/repositories/postgresql"
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

	repos := initializeRepositories(db, log)
	log.Info("Database and repositories initialized successfully", "repository_count", 13)

	// Verify repositories are properly initialized
	if repos == nil {
		log.Error("Failed to initialize repositories")
		os.Exit(1)
	}

	// For Phase 1, create basic services structure
	// Full service wiring will be completed in Phase 2
	services := &server.Services{
		UserService:      nil, // To be implemented in Phase 2
		TenantService:    nil, // To be implemented in Phase 2
		DocumentService:  nil, // To be implemented in Phase 2
		WorkflowService:  nil, // To be implemented in Phase 2
		AIService:        nil, // To be implemented in Phase 3
		AnalyticsService: nil, // To be implemented in Phase 4
	}

	// Create HTTP server
	srv := server.NewServer(cfg, services, log)

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
