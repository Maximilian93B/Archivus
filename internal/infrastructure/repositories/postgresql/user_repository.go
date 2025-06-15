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

type UserRepository struct {
	db *database.DB
}

func NewUserRepository(db *database.DB) repositories.UserRepository {
	return &UserRepository{db: db}
}

func (r *UserRepository) Create(ctx context.Context, user *models.User) error {
	if err := r.db.WithContext(ctx).Create(user).Error; err != nil {
		if isDuplicateKeyError(err) {
			return fmt.Errorf("user with email '%s' already exists", user.Email)
		}
		return fmt.Errorf("failed to create user: %w", err)
	}
	return nil
}

func (r *UserRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.User, error) {
	var user models.User
	err := r.db.WithContext(ctx).Preload("Tenant").Where("id = ?", id).First(&user).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("user not found")
		}
		return nil, fmt.Errorf("failed to get user: %w", err)
	}
	return &user, nil
}

func (r *UserRepository) GetByEmail(ctx context.Context, tenantID uuid.UUID, email string) (*models.User, error) {
	var user models.User
	err := r.db.WithContext(ctx).Preload("Tenant").
		Where("tenant_id = ? AND email = ?", tenantID, email).First(&user).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("user not found")
		}
		return nil, fmt.Errorf("failed to get user: %w", err)
	}
	return &user, nil
}

func (r *UserRepository) Update(ctx context.Context, user *models.User) error {
	result := r.db.WithContext(ctx).Save(user)
	if result.Error != nil {
		return fmt.Errorf("failed to update user: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("user not found")
	}
	return nil
}

func (r *UserRepository) UpdateLastLogin(ctx context.Context, userID uuid.UUID) error {
	result := r.db.WithContext(ctx).Model(&models.User{}).
		Where("id = ?", userID).
		Update("last_login_at", gorm.Expr("NOW()"))

	if result.Error != nil {
		return fmt.Errorf("failed to update last login: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("user not found")
	}
	return nil
}

func (r *UserRepository) ListByTenant(ctx context.Context, tenantID uuid.UUID, params repositories.ListParams) ([]models.User, int64, error) {
	var users []models.User
	var total int64

	query := r.db.WithContext(ctx).Model(&models.User{}).Where("tenant_id = ?", tenantID)

	// Apply search filter
	if params.Search != "" {
		query = query.Where("first_name ILIKE ? OR last_name ILIKE ? OR email ILIKE ?",
			"%"+params.Search+"%", "%"+params.Search+"%", "%"+params.Search+"%")
	}

	// Get total count
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to count users: %w", err)
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

	err := query.Order(orderBy).Offset(offset).Limit(params.PageSize).Find(&users).Error
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list users: %w", err)
	}

	return users, total, nil
}

func (r *UserRepository) SetMFA(ctx context.Context, userID uuid.UUID, enabled bool, secret string) error {
	updates := map[string]interface{}{
		"mfa_enabled": enabled,
		"mfa_secret":  secret,
	}

	result := r.db.WithContext(ctx).Model(&models.User{}).
		Where("id = ?", userID).Updates(updates)

	if result.Error != nil {
		return fmt.Errorf("failed to update MFA settings: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("user not found")
	}
	return nil
}

func (r *UserRepository) Delete(ctx context.Context, id uuid.UUID) error {
	result := r.db.WithContext(ctx).Delete(&models.User{}, id)
	if result.Error != nil {
		return fmt.Errorf("failed to delete user: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("user not found")
	}
	return nil
}
