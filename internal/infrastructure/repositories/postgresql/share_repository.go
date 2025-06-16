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

type ShareRepository struct {
	db *database.DB
}

func NewShareRepository(db *database.DB) repositories.ShareRepository {
	return &ShareRepository{db: db}
}

func (r *ShareRepository) Create(ctx context.Context, share *models.Share) error {
	if err := r.db.WithContext(ctx).Create(share).Error; err != nil {
		return fmt.Errorf("failed to create share: %w", err)
	}
	return nil
}

func (r *ShareRepository) GetByToken(ctx context.Context, token string) (*models.Share, error) {
	var share models.Share
	err := r.db.WithContext(ctx).Preload("Document").Preload("Creator").
		Where("token = ? AND is_active = ?", token, true).First(&share).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("share not found or inactive")
		}
		return nil, fmt.Errorf("failed to get share: %w", err)
	}
	return &share, nil
}

func (r *ShareRepository) GetByDocument(ctx context.Context, documentID uuid.UUID) ([]models.Share, error) {
	var shares []models.Share
	err := r.db.WithContext(ctx).Preload("Creator").
		Where("document_id = ?", documentID).
		Order("created_at DESC").Find(&shares).Error
	if err != nil {
		return nil, fmt.Errorf("failed to get shares by document: %w", err)
	}
	return shares, nil
}

func (r *ShareRepository) Update(ctx context.Context, share *models.Share) error {
	result := r.db.WithContext(ctx).Save(share)
	if result.Error != nil {
		return fmt.Errorf("failed to update share: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("share not found")
	}
	return nil
}

func (r *ShareRepository) IncrementDownload(ctx context.Context, shareID uuid.UUID) error {
	result := r.db.WithContext(ctx).Model(&models.Share{}).
		Where("id = ?", shareID).
		Update("download_count", gorm.Expr("download_count + 1"))

	if result.Error != nil {
		return fmt.Errorf("failed to increment download count: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("share not found")
	}
	return nil
}

func (r *ShareRepository) ExpireShare(ctx context.Context, shareID uuid.UUID) error {
	result := r.db.WithContext(ctx).Model(&models.Share{}).
		Where("id = ?", shareID).
		Update("is_active", false)

	if result.Error != nil {
		return fmt.Errorf("failed to expire share: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("share not found")
	}
	return nil
}

func (r *ShareRepository) ListByCreator(ctx context.Context, creatorID uuid.UUID) ([]models.Share, error) {
	var shares []models.Share
	err := r.db.WithContext(ctx).Preload("Document").
		Where("created_by = ?", creatorID).
		Order("created_at DESC").Find(&shares).Error
	if err != nil {
		return nil, fmt.Errorf("failed to list shares by creator: %w", err)
	}
	return shares, nil
}

func (r *ShareRepository) Delete(ctx context.Context, id uuid.UUID) error {
	result := r.db.WithContext(ctx).Delete(&models.Share{}, id)
	if result.Error != nil {
		return fmt.Errorf("failed to delete share: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("share not found")
	}
	return nil
}
