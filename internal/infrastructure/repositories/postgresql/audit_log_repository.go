package postgresql

import (
	"context"
	"fmt"
	"time"

	"github.com/archivus/archivus/internal/domain/repositories"
	"github.com/archivus/archivus/internal/infrastructure/database"
	"github.com/archivus/archivus/internal/infrastructure/database/models"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type AuditLogRepository struct {
	db *database.DB
}

func NewAuditLogRepository(db *database.DB) repositories.AuditLogRepository {
	return &AuditLogRepository{db: db}
}

func (r *AuditLogRepository) Create(ctx context.Context, log *models.AuditLog) error {
	if err := r.db.WithContext(ctx).Create(log).Error; err != nil {
		return fmt.Errorf("failed to create audit log: %w", err)
	}
	return nil
}

func (r *AuditLogRepository) ListByResource(ctx context.Context, resourceID uuid.UUID, resourceType string, params repositories.ListParams) ([]models.AuditLog, int64, error) {
	var logs []models.AuditLog
	var total int64

	query := r.db.WithContext(ctx).Model(&models.AuditLog{}).
		Where("resource_id = ? AND resource_type = ?", resourceID, resourceType)

	// Apply search filter if provided
	if params.Search != "" {
		query = query.Where("action ILIKE ? OR details::text ILIKE ?",
			"%"+params.Search+"%", "%"+params.Search+"%")
	}

	// Get total count
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to count audit logs: %w", err)
	}

	// Apply pagination and sorting
	offset := (params.Page - 1) * params.PageSize
	orderBy := "created_at DESC"
	if params.SortBy != "" {
		direction := "ASC"
		if params.SortDesc {
			direction = "DESC"
		}
		orderBy = fmt.Sprintf("%s %s", params.SortBy, direction)
	}

	// Load audit logs with user information - use selective preloading to optimize performance
	err := query.
		Preload("User", func(db *gorm.DB) *gorm.DB {
			return db.Select("id", "first_name", "last_name", "email", "role")
		}).
		Preload("Tenant", func(db *gorm.DB) *gorm.DB {
			return db.Select("id", "name", "subdomain")
		}).
		Select("id", "tenant_id", "user_id", "resource_id", "action", "resource_type", "ip_address", "user_agent", "details", "created_at").
		Order(orderBy).Offset(offset).Limit(params.PageSize).Find(&logs).Error
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list audit logs by resource: %w", err)
	}

	return logs, total, nil
}

func (r *AuditLogRepository) ListByUser(ctx context.Context, userID uuid.UUID, params repositories.ListParams) ([]models.AuditLog, int64, error) {
	var logs []models.AuditLog
	var total int64

	query := r.db.WithContext(ctx).Model(&models.AuditLog{}).
		Where("user_id = ?", userID)

	// Apply search filter if provided
	if params.Search != "" {
		query = query.Where("action ILIKE ? OR resource_type ILIKE ? OR details::text ILIKE ?",
			"%"+params.Search+"%", "%"+params.Search+"%", "%"+params.Search+"%")
	}

	// Get total count
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to count audit logs: %w", err)
	}

	// Apply pagination and sorting
	offset := (params.Page - 1) * params.PageSize
	orderBy := "created_at DESC"
	if params.SortBy != "" {
		direction := "ASC"
		if params.SortDesc {
			direction = "DESC"
		}
		orderBy = fmt.Sprintf("%s %s", params.SortBy, direction)
	}

	// Load audit logs with tenant information - use selective preloading to optimize performance
	err := query.
		Preload("Tenant", func(db *gorm.DB) *gorm.DB {
			return db.Select("id", "name", "subdomain")
		}).
		Select("id", "tenant_id", "user_id", "resource_id", "action", "resource_type", "ip_address", "user_agent", "details", "created_at").
		Order(orderBy).Offset(offset).Limit(params.PageSize).Find(&logs).Error
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list audit logs by user: %w", err)
	}

	return logs, total, nil
}

func (r *AuditLogRepository) ListByTenant(ctx context.Context, tenantID uuid.UUID, params repositories.ListParams) ([]models.AuditLog, int64, error) {
	var logs []models.AuditLog
	var total int64

	query := r.db.WithContext(ctx).Model(&models.AuditLog{}).
		Where("tenant_id = ?", tenantID)

	// Apply search filter if provided
	if params.Search != "" {
		query = query.Where("action ILIKE ? OR resource_type ILIKE ? OR user_agent ILIKE ? OR details::text ILIKE ?",
			"%"+params.Search+"%", "%"+params.Search+"%", "%"+params.Search+"%", "%"+params.Search+"%")
	}

	// Get total count
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to count audit logs: %w", err)
	}

	// Apply pagination and sorting
	offset := (params.Page - 1) * params.PageSize
	orderBy := "created_at DESC"
	if params.SortBy != "" {
		direction := "ASC"
		if params.SortDesc {
			direction = "DESC"
		}
		orderBy = fmt.Sprintf("%s %s", params.SortBy, direction)
	}

	// Load audit logs with user information - use selective preloading to optimize performance
	err := query.
		Preload("User", func(db *gorm.DB) *gorm.DB {
			return db.Select("id", "first_name", "last_name", "email", "role")
		}).
		Select("id", "tenant_id", "user_id", "resource_id", "action", "resource_type", "ip_address", "user_agent", "details", "created_at").
		Order(orderBy).Offset(offset).Limit(params.PageSize).Find(&logs).Error
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list audit logs by tenant: %w", err)
	}

	return logs, total, nil
}

func (r *AuditLogRepository) GetSecurityEvents(ctx context.Context, tenantID uuid.UUID, since time.Time) ([]models.AuditLog, error) {
	var logs []models.AuditLog

	// Define security-relevant actions
	securityActions := []models.AuditAction{
		models.AuditCreate,  // User creation
		models.AuditDelete,  // Resource deletion
		models.AuditShare,   // Document sharing
		models.AuditApprove, // Workflow approvals
		models.AuditReject,  // Workflow rejections
	}

	// Use selective preloading to optimize performance
	err := r.db.WithContext(ctx).
		Preload("User", func(db *gorm.DB) *gorm.DB {
			return db.Select("id", "first_name", "last_name", "email", "role")
		}).
		Select("id", "tenant_id", "user_id", "resource_id", "action", "resource_type", "ip_address", "user_agent", "details", "created_at").
		Where("tenant_id = ? AND created_at >= ? AND action IN ?", tenantID, since, securityActions).
		Order("created_at DESC").Find(&logs).Error
	if err != nil {
		return nil, fmt.Errorf("failed to get security events: %w", err)
	}

	return logs, nil
}
