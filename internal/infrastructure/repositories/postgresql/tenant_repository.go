package postgresql

import (
	"context"
	"errors"
	"fmt"

	"github.com/archivus/archivus/internal/domain/repositories"
	"github.com/archivus/archivus/internal/infrastructure/database"
	"github.com/archivus/archivus/internal/infrastructure/database/models"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type TenantRepository struct {
	db *database.DB
}

func NewTenantRepository(db *database.DB) repositories.TenantRepository {
	return &TenantRepository{db: db}
}

func (r *TenantRepository) Create(ctx context.Context, tenant *models.Tenant) error {
	if err := r.db.WithContext(ctx).Create(tenant).Error; err != nil {
		if isDuplicateKeyError(err) {
			return fmt.Errorf("tenant with subdomain '%s' already exists", tenant.Subdomain)
		}
		return fmt.Errorf("failed to create tenant: %w", err)
	}
	return nil
}

func (r *TenantRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.Tenant, error) {
	var tenant models.Tenant
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&tenant).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("tenant not found")
		}
		return nil, fmt.Errorf("failed to get tenant: %w", err)
	}
	return &tenant, nil
}

func (r *TenantRepository) GetBySubdomain(ctx context.Context, subdomain string) (*models.Tenant, error) {
	var tenant models.Tenant
	err := r.db.WithContext(ctx).Where("subdomain = ? AND is_active = ?", subdomain, true).First(&tenant).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("tenant not found")
		}
		return nil, fmt.Errorf("failed to get tenant: %w", err)
	}
	return &tenant, nil
}

func (r *TenantRepository) Update(ctx context.Context, tenant *models.Tenant) error {
	result := r.db.WithContext(ctx).Save(tenant)
	if result.Error != nil {
		return fmt.Errorf("failed to update tenant: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("tenant not found")
	}
	return nil
}

func (r *TenantRepository) UpdateUsage(ctx context.Context, tenantID uuid.UUID, storageUsed int64, apiUsed int) error {
	result := r.db.WithContext(ctx).Model(&models.Tenant{}).
		Where("id = ?", tenantID).
		Updates(map[string]interface{}{
			"storage_used": gorm.Expr("storage_used + ?", storageUsed),
			"api_used":     gorm.Expr("api_used + ?", apiUsed),
		})

	if result.Error != nil {
		return fmt.Errorf("failed to update tenant usage: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("tenant not found")
	}
	return nil
}

func (r *TenantRepository) CheckQuotaLimits(ctx context.Context, tenantID uuid.UUID) (*repositories.QuotaStatus, error) {
	var tenant models.Tenant
	err := r.db.WithContext(ctx).Select("storage_used", "storage_quota", "api_used", "api_quota").
		Where("id = ?", tenantID).First(&tenant).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("tenant not found")
		}
		return nil, fmt.Errorf("failed to get tenant quota: %w", err)
	}

	storagePercent := float64(tenant.StorageUsed) / float64(tenant.StorageQuota) * 100
	apiPercent := float64(tenant.APIUsed) / float64(tenant.APIQuota) * 100

	return &repositories.QuotaStatus{
		StorageUsed:    tenant.StorageUsed,
		StorageQuota:   tenant.StorageQuota,
		StoragePercent: storagePercent,
		APIUsed:        tenant.APIUsed,
		APIQuota:       tenant.APIQuota,
		APIPercent:     apiPercent,
		CanUpload:      storagePercent < 95, // 95% limit
		CanProcessAI:   apiPercent < 95,     // 95% limit
	}, nil
}

func (r *TenantRepository) List(ctx context.Context, params repositories.ListParams) ([]models.Tenant, int64, error) {
	var tenants []models.Tenant
	var total int64

	query := r.db.WithContext(ctx).Model(&models.Tenant{})

	// Apply search filter
	if params.Search != "" {
		query = query.Where("name ILIKE ? OR subdomain ILIKE ?", "%"+params.Search+"%", "%"+params.Search+"%")
	}

	// Get total count
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to count tenants: %w", err)
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

	err := query.Order(orderBy).Offset(offset).Limit(params.PageSize).Find(&tenants).Error
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list tenants: %w", err)
	}

	return tenants, total, nil
}

func (r *TenantRepository) Delete(ctx context.Context, id uuid.UUID) error {
	result := r.db.WithContext(ctx).Delete(&models.Tenant{}, id)
	if result.Error != nil {
		return fmt.Errorf("failed to delete tenant: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("tenant not found")
	}
	return nil
}

// Helper function to check for duplicate key errors
func isDuplicateKeyError(err error) bool {
	return err != nil && (
	// PostgreSQL duplicate key error codes
	containsString(err.Error(), "duplicate key") ||
		containsString(err.Error(), "unique constraint") ||
		containsString(err.Error(), "UNIQUE constraint failed"))
}

func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) &&
		(s[:len(substr)] == substr || s[len(s)-len(substr):] == substr ||
			findInString(s, substr)))
}

func findInString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
