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

type NotificationRepository struct {
	db *database.DB
}

func NewNotificationRepository(db *database.DB) repositories.NotificationRepository {
	return &NotificationRepository{db: db}
}

func (r *NotificationRepository) Create(ctx context.Context, notification *models.Notification) error {
	if err := r.db.WithContext(ctx).Create(notification).Error; err != nil {
		return fmt.Errorf("failed to create notification: %w", err)
	}
	return nil
}

func (r *NotificationRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.Notification, error) {
	var notification models.Notification
	err := r.db.WithContext(ctx).Preload("Tenant").Preload("User").
		Where("id = ?", id).First(&notification).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("notification not found")
		}
		return nil, fmt.Errorf("failed to get notification: %w", err)
	}
	return &notification, nil
}

func (r *NotificationRepository) ListByUser(ctx context.Context, userID uuid.UUID, params repositories.ListParams) ([]models.Notification, int64, error) {
	var notifications []models.Notification
	var total int64

	query := r.db.WithContext(ctx).Model(&models.Notification{}).
		Where("user_id = ?", userID)

	// Apply search filter if provided
	if params.Search != "" {
		query = query.Where("title ILIKE ? OR message ILIKE ? OR type ILIKE ?",
			"%"+params.Search+"%", "%"+params.Search+"%", "%"+params.Search+"%")
	}

	// Get total count
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to count notifications: %w", err)
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

	err := query.Order(orderBy).Offset(offset).Limit(params.PageSize).Find(&notifications).Error
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list notifications by user: %w", err)
	}

	return notifications, total, nil
}

func (r *NotificationRepository) ListByTenant(ctx context.Context, tenantID uuid.UUID, params repositories.ListParams) ([]models.Notification, int64, error) {
	var notifications []models.Notification
	var total int64

	query := r.db.WithContext(ctx).Model(&models.Notification{}).
		Where("tenant_id = ?", tenantID)

	// Apply search filter if provided
	if params.Search != "" {
		query = query.Where("title ILIKE ? OR message ILIKE ? OR type ILIKE ?",
			"%"+params.Search+"%", "%"+params.Search+"%", "%"+params.Search+"%")
	}

	// Get total count
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to count notifications: %w", err)
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

	// Load notifications with user information
	err := query.Preload("User").
		Order(orderBy).Offset(offset).Limit(params.PageSize).Find(&notifications).Error
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list notifications by tenant: %w", err)
	}

	return notifications, total, nil
}

func (r *NotificationRepository) MarkAsRead(ctx context.Context, notificationID uuid.UUID) error {
	result := r.db.WithContext(ctx).Model(&models.Notification{}).
		Where("id = ?", notificationID).
		Update("is_read", true)

	if result.Error != nil {
		return fmt.Errorf("failed to mark notification as read: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("notification not found")
	}
	return nil
}

func (r *NotificationRepository) MarkAllAsRead(ctx context.Context, userID uuid.UUID) error {
	result := r.db.WithContext(ctx).Model(&models.Notification{}).
		Where("user_id = ? AND is_read = ?", userID, false).
		Update("is_read", true)

	if result.Error != nil {
		return fmt.Errorf("failed to mark all notifications as read: %w", result.Error)
	}
	return nil
}

func (r *NotificationRepository) Delete(ctx context.Context, id uuid.UUID) error {
	result := r.db.WithContext(ctx).Delete(&models.Notification{}, id)
	if result.Error != nil {
		return fmt.Errorf("failed to delete notification: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("notification not found")
	}
	return nil
}
