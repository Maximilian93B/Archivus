package main

import (
	"os"

	"github.com/archivus/archivus/internal/app/config"
	"github.com/archivus/archivus/internal/app/server"
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

	// TODO: Initialize database and repositories
	// This would be done in a future iteration
	// db, err := database.New(cfg.GetDatabaseURL())
	// repos := initializeRepositories(db)

	// TODO: Initialize external services (storage, AI, email, etc.)
	// This would be done when implementing infrastructure layer
	// storageService := initializeStorageService(cfg)
	// aiService := initializeAIService(cfg)

	// TODO: Initialize business services with real dependencies
	// For now, we'll create placeholder services
	services := &server.Services{
		// These would be initialized with real dependencies
		UserService:      nil, // services.NewUserService(repos.UserRepo, ...)
		TenantService:    nil, // services.NewTenantService(repos.TenantRepo, ...)
		DocumentService:  nil, // services.NewDocumentService(repos.DocumentRepo, ...)
		WorkflowService:  nil, // services.NewWorkflowService(repos.WorkflowRepo, ...)
		AIService:        nil, // services.NewAIService(repos.AIJobRepo, ...)
		AnalyticsService: nil, // services.NewAnalyticsService(repos.AnalyticsRepo, ...)
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

// TODO: These initialization functions would be implemented when we have the infrastructure layer

// func initializeRepositories(db *database.DB) *repositories.Repositories {
//     return &repositories.Repositories{
//         UserRepo:     postgresql.NewUserRepository(db),
//         TenantRepo:   postgresql.NewTenantRepository(db),
//         DocumentRepo: postgresql.NewDocumentRepository(db),
//         // ... other repos
//     }
// }

// func initializeStorageService(cfg *config.Config) services.StorageService {
//     switch cfg.Storage.Type {
//     case "s3":
//         return s3.NewStorageService(cfg.Storage)
//     case "local":
//         return local.NewStorageService(cfg.Storage.Path)
//     default:
//         return local.NewStorageService("./uploads")
//     }
// }

// func initializeAIService(cfg *config.Config) services.AIService {
//     if !cfg.AI.Enabled {
//         return &services.NoOpAIService{}
//     }
//
//     return openai.NewAIService(cfg.AI.OpenAI)
// }
