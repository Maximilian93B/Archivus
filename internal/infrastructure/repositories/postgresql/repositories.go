package postgresql

import (
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
	}
}
