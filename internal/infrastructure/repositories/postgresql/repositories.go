package postgresql

import (
	"context"
	"fmt"

	"github.com/archivus/archivus/internal/domain/repositories"
	"github.com/archivus/archivus/internal/infrastructure/database"
)

// Repositories holds all repository implementations
type Repositories struct {
	TenantRepo       repositories.TenantRepository
	UserRepo         repositories.UserRepository
	DocumentRepo     repositories.DocumentRepository
	FolderRepo       repositories.FolderRepository
	TagRepo          repositories.TagRepository
	CategoryRepo     repositories.CategoryRepository
	WorkflowRepo     repositories.WorkflowRepository
	WorkflowTaskRepo repositories.WorkflowTaskRepository
	AIJobRepo        repositories.AIProcessingJobRepository
	AuditRepo        repositories.AuditLogRepository
	ShareRepo        repositories.ShareRepository
	AnalyticsRepo    repositories.AnalyticsRepository
	NotificationRepo repositories.NotificationRepository

	// Internal reference to database for health checks
	db *database.DB
}

// NewRepositories creates a new repositories container
func NewRepositories(db *database.DB) *Repositories {
	return &Repositories{
		TenantRepo:       NewTenantRepository(db),
		UserRepo:         NewUserRepository(db),
		DocumentRepo:     NewDocumentRepository(db),
		FolderRepo:       NewFolderRepository(db),
		TagRepo:          NewTagRepository(db),
		CategoryRepo:     NewCategoryRepository(db),
		WorkflowRepo:     NewWorkflowRepository(db),
		WorkflowTaskRepo: NewWorkflowTaskRepository(db),
		AIJobRepo:        NewAIProcessingJobRepository(db),
		AuditRepo:        NewAuditLogRepository(db),
		ShareRepo:        NewShareRepository(db),
		AnalyticsRepo:    NewAnalyticsRepository(db),
		NotificationRepo: NewNotificationRepository(db),
		db:               db,
	}
}

// HealthCheck verifies database connectivity
func (r *Repositories) HealthCheck(ctx context.Context) error {
	sqlDB, err := r.db.DB.DB()
	if err != nil {
		return fmt.Errorf("failed to get underlying sql.DB: %w", err)
	}

	if err := sqlDB.PingContext(ctx); err != nil {
		return fmt.Errorf("database ping failed: %w", err)
	}

	return nil
}
