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

type TagRepository struct {
	db *database.DB
}

func NewTagRepository(db *database.DB) repositories.TagRepository {
	return &TagRepository{db: db}
}

func (r *TagRepository) Create(ctx context.Context, tag *models.Tag) error {
	if err := r.db.WithContext(ctx).Create(tag).Error; err != nil {
		if isDuplicateKeyError(err) {
			return fmt.Errorf("tag with name '%s' already exists", tag.Name)
		}
		return fmt.Errorf("failed to create tag: %w", err)
	}
	return nil
}

func (r *TagRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.Tag, error) {
	var tag models.Tag
	err := r.db.WithContext(ctx).Preload("Tenant").Where("id = ?", id).First(&tag).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("tag not found")
		}
		return nil, fmt.Errorf("failed to get tag: %w", err)
	}
	return &tag, nil
}

func (r *TagRepository) GetByName(ctx context.Context, tenantID uuid.UUID, name string) (*models.Tag, error) {
	var tag models.Tag
	err := r.db.WithContext(ctx).Where("tenant_id = ? AND name = ?", tenantID, name).First(&tag).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("tag not found")
		}
		return nil, fmt.Errorf("failed to get tag by name: %w", err)
	}
	return &tag, nil
}

func (r *TagRepository) Update(ctx context.Context, tag *models.Tag) error {
	result := r.db.WithContext(ctx).Save(tag)
	if result.Error != nil {
		if isDuplicateKeyError(result.Error) {
			return fmt.Errorf("tag with name '%s' already exists", tag.Name)
		}
		return fmt.Errorf("failed to update tag: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("tag not found")
	}
	return nil
}

func (r *TagRepository) ListByTenant(ctx context.Context, tenantID uuid.UUID) ([]models.Tag, error) {
	var tags []models.Tag
	err := r.db.WithContext(ctx).Where("tenant_id = ?", tenantID).
		Order("usage_count DESC, name ASC").Find(&tags).Error
	if err != nil {
		return nil, fmt.Errorf("failed to list tags: %w", err)
	}
	return tags, nil
}

func (r *TagRepository) GetPopular(ctx context.Context, tenantID uuid.UUID, limit int) ([]models.Tag, error) {
	var tags []models.Tag
	err := r.db.WithContext(ctx).Where("tenant_id = ?", tenantID).
		Order("usage_count DESC, name ASC").Limit(limit).Find(&tags).Error
	if err != nil {
		return nil, fmt.Errorf("failed to get popular tags: %w", err)
	}
	return tags, nil
}

func (r *TagRepository) IncrementUsage(ctx context.Context, tagID uuid.UUID) error {
	result := r.db.WithContext(ctx).Model(&models.Tag{}).
		Where("id = ?", tagID).
		Update("usage_count", gorm.Expr("usage_count + 1"))

	if result.Error != nil {
		return fmt.Errorf("failed to increment tag usage: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("tag not found")
	}
	return nil
}

func (r *TagRepository) BulkCreate(ctx context.Context, tags []models.Tag) error {
	if len(tags) == 0 {
		return nil
	}

	// Use transaction for bulk operations
	tx := r.db.WithContext(ctx).Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// Create tags in batches
	batchSize := 100
	for i := 0; i < len(tags); i += batchSize {
		end := i + batchSize
		if end > len(tags) {
			end = len(tags)
		}

		batch := tags[i:end]
		if err := tx.Create(&batch).Error; err != nil {
			tx.Rollback()
			return fmt.Errorf("failed to bulk create tags: %w", err)
		}
	}

	return tx.Commit().Error
}

func (r *TagRepository) Delete(ctx context.Context, id uuid.UUID) error {
	// Start transaction to handle document associations
	tx := r.db.WithContext(ctx).Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// Remove tag associations from documents
	if err := tx.Exec("DELETE FROM document_tags WHERE tag_id = ?", id).Error; err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to remove tag associations: %w", err)
	}

	// Delete the tag
	result := tx.Delete(&models.Tag{}, id)
	if result.Error != nil {
		tx.Rollback()
		return fmt.Errorf("failed to delete tag: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		tx.Rollback()
		return fmt.Errorf("tag not found")
	}

	return tx.Commit().Error
}
