package postgresql

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/archivus/archivus/internal/domain/repositories"
	"github.com/archivus/archivus/internal/infrastructure/database"
	"github.com/archivus/archivus/internal/infrastructure/database/models"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type WorkflowTaskRepository struct {
	db *database.DB
}

func NewWorkflowTaskRepository(db *database.DB) repositories.WorkflowTaskRepository {
	return &WorkflowTaskRepository{db: db}
}

func (r *WorkflowTaskRepository) Create(ctx context.Context, task *models.WorkflowTask) error {
	if err := r.db.WithContext(ctx).Create(task).Error; err != nil {
		return fmt.Errorf("failed to create workflow task: %w", err)
	}
	return nil
}

func (r *WorkflowTaskRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.WorkflowTask, error) {
	var task models.WorkflowTask
	// Use selective preloading to optimize performance
	err := r.db.WithContext(ctx).
		Preload("Workflow", func(db *gorm.DB) *gorm.DB {
			return db.Select("id", "name", "description", "doc_type", "tenant_id")
		}).
		Preload("Document", func(db *gorm.DB) *gorm.DB {
			return db.Select("id", "title", "file_name", "document_type", "status", "tenant_id")
		}).
		Preload("Assignee", func(db *gorm.DB) *gorm.DB {
			return db.Select("id", "first_name", "last_name", "email", "role")
		}).
		Where("id = ?", id).First(&task).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("workflow task not found")
		}
		return nil, fmt.Errorf("failed to get workflow task: %w", err)
	}
	return &task, nil
}

func (r *WorkflowTaskRepository) Update(ctx context.Context, task *models.WorkflowTask) error {
	result := r.db.WithContext(ctx).Save(task)
	if result.Error != nil {
		return fmt.Errorf("failed to update workflow task: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("workflow task not found")
	}
	return nil
}

func (r *WorkflowTaskRepository) ListByAssignee(ctx context.Context, userID uuid.UUID, status models.WorkflowStatus) ([]models.WorkflowTask, error) {
	var tasks []models.WorkflowTask
	// Use selective preloading and field selection to optimize performance
	query := r.db.WithContext(ctx).
		Preload("Workflow", func(db *gorm.DB) *gorm.DB {
			return db.Select("id", "name", "doc_type")
		}).
		Preload("Document", func(db *gorm.DB) *gorm.DB {
			return db.Select("id", "title", "file_name", "document_type", "status")
		}).
		Select("id", "workflow_id", "document_id", "assigned_to", "task_type", "status", "priority", "due_date", "comments", "created_at").
		Where("assigned_to = ?", userID)

	// Filter by status if provided
	if status != "" {
		query = query.Where("status = ?", status)
	}

	err := query.Order("priority ASC, due_date ASC, created_at ASC").Find(&tasks).Error
	if err != nil {
		return nil, fmt.Errorf("failed to list workflow tasks by assignee: %w", err)
	}
	return tasks, nil
}

func (r *WorkflowTaskRepository) ListByDocument(ctx context.Context, documentID uuid.UUID) ([]models.WorkflowTask, error) {
	var tasks []models.WorkflowTask
	// Use selective preloading to optimize performance
	err := r.db.WithContext(ctx).
		Preload("Workflow", func(db *gorm.DB) *gorm.DB {
			return db.Select("id", "name", "doc_type")
		}).
		Preload("Assignee", func(db *gorm.DB) *gorm.DB {
			return db.Select("id", "first_name", "last_name", "email", "role")
		}).
		Select("id", "workflow_id", "document_id", "assigned_to", "task_type", "status", "priority", "due_date", "comments", "created_at", "completed_at").
		Where("document_id = ?", documentID).
		Order("created_at DESC").Find(&tasks).Error
	if err != nil {
		return nil, fmt.Errorf("failed to list workflow tasks by document: %w", err)
	}
	return tasks, nil
}

func (r *WorkflowTaskRepository) GetPendingTasks(ctx context.Context, tenantID uuid.UUID) ([]models.WorkflowTask, error) {
	var tasks []models.WorkflowTask
	// Use selective preloading to optimize performance
	err := r.db.WithContext(ctx).
		Preload("Workflow", func(db *gorm.DB) *gorm.DB {
			return db.Select("id", "name", "doc_type")
		}).
		Preload("Document", func(db *gorm.DB) *gorm.DB {
			return db.Select("id", "title", "file_name", "document_type", "status")
		}).
		Preload("Assignee", func(db *gorm.DB) *gorm.DB {
			return db.Select("id", "first_name", "last_name", "email")
		}).
		Select("workflow_tasks.id", "workflow_tasks.workflow_id", "workflow_tasks.document_id", "workflow_tasks.assigned_to", "workflow_tasks.task_type", "workflow_tasks.status", "workflow_tasks.priority", "workflow_tasks.due_date", "workflow_tasks.created_at").
		Joins("JOIN workflows ON workflow_tasks.workflow_id = workflows.id").
		Where("workflows.tenant_id = ? AND workflow_tasks.status = ?", tenantID, models.WorkflowPending).
		Order("workflow_tasks.priority ASC, workflow_tasks.due_date ASC").Find(&tasks).Error
	if err != nil {
		return nil, fmt.Errorf("failed to get pending tasks: %w", err)
	}
	return tasks, nil
}

func (r *WorkflowTaskRepository) GetOverdueTasks(ctx context.Context, tenantID uuid.UUID) ([]models.WorkflowTask, error) {
	var tasks []models.WorkflowTask
	now := time.Now()
	// Use selective preloading to optimize performance
	err := r.db.WithContext(ctx).
		Preload("Workflow", func(db *gorm.DB) *gorm.DB {
			return db.Select("id", "name", "doc_type")
		}).
		Preload("Document", func(db *gorm.DB) *gorm.DB {
			return db.Select("id", "title", "file_name", "document_type", "status")
		}).
		Preload("Assignee", func(db *gorm.DB) *gorm.DB {
			return db.Select("id", "first_name", "last_name", "email")
		}).
		Select("workflow_tasks.id", "workflow_tasks.workflow_id", "workflow_tasks.document_id", "workflow_tasks.assigned_to", "workflow_tasks.task_type", "workflow_tasks.status", "workflow_tasks.priority", "workflow_tasks.due_date", "workflow_tasks.created_at").
		Joins("JOIN workflows ON workflow_tasks.workflow_id = workflows.id").
		Where("workflows.tenant_id = ? AND workflow_tasks.status = ? AND workflow_tasks.due_date < ?",
			tenantID, models.WorkflowPending, now).
		Order("workflow_tasks.due_date ASC").Find(&tasks).Error
	if err != nil {
		return nil, fmt.Errorf("failed to get overdue tasks: %w", err)
	}
	return tasks, nil
}

func (r *WorkflowTaskRepository) Complete(ctx context.Context, taskID uuid.UUID, completedBy uuid.UUID, comments string) error {
	now := time.Now()
	updates := map[string]interface{}{
		"status":       models.WorkflowApproved, // Default to approved - could be parameterized
		"comments":     comments,
		"completed_at": now,
	}

	result := r.db.WithContext(ctx).Model(&models.WorkflowTask{}).
		Where("id = ? AND status = ?", taskID, models.WorkflowPending).
		Updates(updates)

	if result.Error != nil {
		return fmt.Errorf("failed to complete workflow task: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("workflow task not found or not in pending status")
	}
	return nil
}

func (r *WorkflowTaskRepository) Delete(ctx context.Context, id uuid.UUID) error {
	// Check if task is in a state that can be deleted
	var task models.WorkflowTask
	err := r.db.WithContext(ctx).Select("status").Where("id = ?", id).First(&task).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("workflow task not found")
		}
		return fmt.Errorf("failed to check workflow task status: %w", err)
	}

	// Don't allow deletion of pending tasks
	if task.Status == models.WorkflowPending {
		return fmt.Errorf("cannot delete pending workflow tasks")
	}

	result := r.db.WithContext(ctx).Delete(&models.WorkflowTask{}, id)
	if result.Error != nil {
		return fmt.Errorf("failed to delete workflow task: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("workflow task not found")
	}
	return nil
}
