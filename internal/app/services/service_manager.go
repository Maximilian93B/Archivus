package services

import (
	"context"
	"fmt"

	"github.com/archivus/archivus/internal/app/config"
	"github.com/archivus/archivus/internal/domain/services"
	"github.com/archivus/archivus/internal/infrastructure/cache"
	"github.com/archivus/archivus/internal/infrastructure/database"
	"github.com/archivus/archivus/internal/infrastructure/repositories/postgresql"
)

// ServiceManager manages all application services
type ServiceManager struct {
	Config *config.Config

	// Infrastructure
	DB           *database.DB
	Repositories *postgresql.Repositories
	CacheService services.CacheService
}

// NewServiceManager creates a new service manager
func NewServiceManager(cfg *config.Config, db *database.DB) (*ServiceManager, error) {
	// Initialize repositories
	repos := postgresql.NewRepositories(db)

	// Initialize cache service with Redis
	cacheService, err := cache.CreateCacheService(cfg.Redis.URL)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize cache service: %w", err)
	}

	sm := &ServiceManager{
		Config:       cfg,
		DB:           db,
		Repositories: repos,
		CacheService: cacheService,
	}

	return sm, nil
}

// Health check for all services
func (sm *ServiceManager) HealthCheck() error {
	// Check database
	if err := sm.Repositories.HealthCheck(context.Background()); err != nil {
		return fmt.Errorf("database health check failed: %w", err)
	}

	// Check Redis cache
	if err := sm.CacheService.Ping(context.Background()); err != nil {
		return fmt.Errorf("redis health check failed: %w", err)
	}

	return nil
}

// Close gracefully shuts down all services
func (sm *ServiceManager) Close() error {
	// Close cache service
	if err := sm.CacheService.Close(); err != nil {
		return fmt.Errorf("failed to close cache service: %w", err)
	}

	// Close database connection
	if err := sm.DB.Close(); err != nil {
		return fmt.Errorf("failed to close database: %w", err)
	}

	return nil
}
