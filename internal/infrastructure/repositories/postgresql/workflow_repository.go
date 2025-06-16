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

type WorkflowRepository struct {
	db *database.DB
}

func NewWorkflowRepository(db *database.DB) repositories.WorkflowRepository {
	return &WorkflowRepository{db: db}
}

func (r *WorkflowRepository) Create(ctx context.Context, workflow *models.Workflow) error {
	if err := r.db.WithContext(ctx).Create(workflow).Error; err != nil {
		return fmt.Errorf("failed to create workflow: %w", err)
	}
	return nil
}

func (r *WorkflowRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.Workflow, error) {
	var workflow models.Workflow
	err := r.db.WithContext(ctx).Preload("Tenant").Preload("Creator").
		Where("id = ?", id).First(&workflow).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("workflow not found")
		}
		return nil, fmt.Errorf("failed to get workflow: %w", err)
	}
	return &workflow, nil
}

func (r *WorkflowRepository) Update(ctx context.Context, workflow *models.Workflow) error {
	result := r.db.WithContext(ctx).Save(workflow)
	if result.Error != nil {
		return fmt.Errorf("failed to update workflow: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("workflow not found")
	}
	return nil
}

func (r *WorkflowRepository) ListByTenant(ctx context.Context, tenantID uuid.UUID) ([]models.Workflow, error) {
	var workflows []models.Workflow
	err := r.db.WithContext(ctx).Preload("Creator").
		Where("tenant_id = ?", tenantID).
		Order("name ASC").Find(&workflows).Error
	if err != nil {
		return nil, fmt.Errorf("failed to list workflows: %w", err)
	}
	return workflows, nil
}

func (r *WorkflowRepository) GetByDocumentType(ctx context.Context, tenantID uuid.UUID, docType models.DocumentType) ([]models.Workflow, error) {
	var workflows []models.Workflow
	err := r.db.WithContext(ctx).Preload("Creator").
		Where("tenant_id = ? AND doc_type = ? AND is_active = ?", tenantID, docType, true).
		Order("name ASC").Find(&workflows).Error
	if err != nil {
		return nil, fmt.Errorf("failed to get workflows by document type: %w", err)
	}
	return workflows, nil
}

func (r *WorkflowRepository) Delete(ctx context.Context, id uuid.UUID) error {
	// Start transaction to handle cascade deletion
	tx := r.db.WithContext(ctx).Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// Check for active workflow tasks
	var taskCount int64
	if err := tx.Model(&models.WorkflowTask{}).
		Where("workflow_id = ? AND status IN ?", id, []models.WorkflowStatus{
			models.WorkflowPending,
		}).Count(&taskCount).Error; err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to check for active tasks: %w", err)
	}

	if taskCount > 0 {
		tx.Rollback()
		return fmt.Errorf("cannot delete workflow with active tasks")
	}

	// Delete associated workflow tasks
	if err := tx.Where("workflow_id = ?", id).Delete(&models.WorkflowTask{}).Error; err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to delete workflow tasks: %w", err)
	}

	// Delete the workflow
	result := tx.Delete(&models.Workflow{}, id)
	if result.Error != nil {
		tx.Rollback()
		return fmt.Errorf("failed to delete workflow: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		tx.Rollback()
		return fmt.Errorf("workflow not found")
	}

	return tx.Commit().Error
}
