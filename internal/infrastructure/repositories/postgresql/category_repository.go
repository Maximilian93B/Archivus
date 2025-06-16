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

type CategoryRepository struct {
	db *database.DB
}

func NewCategoryRepository(db *database.DB) repositories.CategoryRepository {
	return &CategoryRepository{db: db}
}

func (r *CategoryRepository) Create(ctx context.Context, category *models.Category) error {
	if err := r.db.WithContext(ctx).Create(category).Error; err != nil {
		if isDuplicateKeyError(err) {
			return fmt.Errorf("category with name '%s' already exists", category.Name)
		}
		return fmt.Errorf("failed to create category: %w", err)
	}
	return nil
}

func (r *CategoryRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.Category, error) {
	var category models.Category
	err := r.db.WithContext(ctx).Preload("Tenant").Where("id = ?", id).First(&category).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("category not found")
		}
		return nil, fmt.Errorf("failed to get category: %w", err)
	}
	return &category, nil
}

func (r *CategoryRepository) GetByName(ctx context.Context, tenantID uuid.UUID, name string) (*models.Category, error) {
	var category models.Category
	err := r.db.WithContext(ctx).Where("tenant_id = ? AND name = ?", tenantID, name).First(&category).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("category not found")
		}
		return nil, fmt.Errorf("failed to get category by name: %w", err)
	}
	return &category, nil
}

func (r *CategoryRepository) Update(ctx context.Context, category *models.Category) error {
	result := r.db.WithContext(ctx).Save(category)
	if result.Error != nil {
		if isDuplicateKeyError(result.Error) {
			return fmt.Errorf("category with name '%s' already exists", category.Name)
		}
		return fmt.Errorf("failed to update category: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("category not found")
	}
	return nil
}

func (r *CategoryRepository) ListByTenant(ctx context.Context, tenantID uuid.UUID) ([]models.Category, error) {
	var categories []models.Category
	err := r.db.WithContext(ctx).Where("tenant_id = ?", tenantID).
		Order("sort_order ASC, name ASC").Find(&categories).Error
	if err != nil {
		return nil, fmt.Errorf("failed to list categories: %w", err)
	}
	return categories, nil
}

func (r *CategoryRepository) GetDocumentCount(ctx context.Context, categoryID uuid.UUID) (int64, error) {
	var count int64

	// Count documents in this category through the many-to-many relationship
	err := r.db.WithContext(ctx).
		Table("document_categories").
		Where("category_id = ?", categoryID).
		Count(&count).Error

	if err != nil {
		return 0, fmt.Errorf("failed to count documents in category: %w", err)
	}
	return count, nil
}

func (r *CategoryRepository) Delete(ctx context.Context, id uuid.UUID) error {
	// Start transaction to handle document associations
	tx := r.db.WithContext(ctx).Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// Check if this is a system category that shouldn't be deleted
	var category models.Category
	if err := tx.Select("is_system").Where("id = ?", id).First(&category).Error; err != nil {
		tx.Rollback()
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("category not found")
		}
		return fmt.Errorf("failed to check category: %w", err)
	}

	if category.IsSystem {
		tx.Rollback()
		return fmt.Errorf("cannot delete system category")
	}

	// Remove category associations from documents
	if err := tx.Exec("DELETE FROM document_categories WHERE category_id = ?", id).Error; err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to remove category associations: %w", err)
	}

	// Delete the category
	result := tx.Delete(&models.Category{}, id)
	if result.Error != nil {
		tx.Rollback()
		return fmt.Errorf("failed to delete category: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		tx.Rollback()
		return fmt.Errorf("category not found")
	}

	return tx.Commit().Error
}
