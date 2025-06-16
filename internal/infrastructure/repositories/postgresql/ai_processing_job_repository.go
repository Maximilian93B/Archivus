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

type AIProcessingJobRepository struct {
	db *database.DB
}

func NewAIProcessingJobRepository(db *database.DB) repositories.AIProcessingJobRepository {
	return &AIProcessingJobRepository{db: db}
}

func (r *AIProcessingJobRepository) Create(ctx context.Context, job *models.AIProcessingJob) error {
	if err := r.db.WithContext(ctx).Create(job).Error; err != nil {
		return fmt.Errorf("failed to create AI processing job: %w", err)
	}
	return nil
}

func (r *AIProcessingJobRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.AIProcessingJob, error) {
	var job models.AIProcessingJob
	err := r.db.WithContext(ctx).Preload("Tenant").Preload("Document").
		Where("id = ?", id).First(&job).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("AI processing job not found")
		}
		return nil, fmt.Errorf("failed to get AI processing job: %w", err)
	}
	return &job, nil
}

func (r *AIProcessingJobRepository) GetNextJob(ctx context.Context) (*models.AIProcessingJob, error) {
	var job models.AIProcessingJob

	// Get the next job with highest priority that is queued and hasn't exceeded max attempts
	err := r.db.WithContext(ctx).Preload("Document").
		Where("status = ? AND attempts < max_attempts", models.ProcessingQueued).
		Order("priority ASC, created_at ASC").
		First(&job).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil // No jobs available
		}
		return nil, fmt.Errorf("failed to get next AI processing job: %w", err)
	}
	return &job, nil
}

func (r *AIProcessingJobRepository) Update(ctx context.Context, job *models.AIProcessingJob) error {
	result := r.db.WithContext(ctx).Save(job)
	if result.Error != nil {
		return fmt.Errorf("failed to update AI processing job: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("AI processing job not found")
	}
	return nil
}

func (r *AIProcessingJobRepository) UpdateStatus(ctx context.Context, jobID uuid.UUID, status models.ProcessingStatus) error {
	updates := map[string]interface{}{
		"status": status,
	}

	// Set timestamps based on status
	now := time.Now()
	switch status {
	case models.ProcessingInProgress:
		updates["started_at"] = now
	case models.ProcessingCompleted, models.ProcessingFailed:
		updates["completed_at"] = now
	}

	result := r.db.WithContext(ctx).Model(&models.AIProcessingJob{}).
		Where("id = ?", jobID).Updates(updates)

	if result.Error != nil {
		return fmt.Errorf("failed to update AI processing job status: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("AI processing job not found")
	}
	return nil
}

func (r *AIProcessingJobRepository) ListByDocument(ctx context.Context, documentID uuid.UUID) ([]models.AIProcessingJob, error) {
	var jobs []models.AIProcessingJob
	err := r.db.WithContext(ctx).Where("document_id = ?", documentID).
		Order("created_at DESC").Find(&jobs).Error
	if err != nil {
		return nil, fmt.Errorf("failed to list AI processing jobs by document: %w", err)
	}
	return jobs, nil
}

func (r *AIProcessingJobRepository) GetFailedJobs(ctx context.Context, tenantID uuid.UUID) ([]models.AIProcessingJob, error) {
	var jobs []models.AIProcessingJob
	err := r.db.WithContext(ctx).Preload("Document").
		Where("tenant_id = ? AND status = ?", tenantID, models.ProcessingFailed).
		Order("created_at DESC").Find(&jobs).Error
	if err != nil {
		return nil, fmt.Errorf("failed to get failed AI processing jobs: %w", err)
	}
	return jobs, nil
}

func (r *AIProcessingJobRepository) RetryJob(ctx context.Context, jobID uuid.UUID) error {
	// Check if job exists and can be retried
	var job models.AIProcessingJob
	err := r.db.WithContext(ctx).Where("id = ?", jobID).First(&job).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("AI processing job not found")
		}
		return fmt.Errorf("failed to get AI processing job: %w", err)
	}

	// Check if job can be retried
	if job.Status != models.ProcessingFailed {
		return fmt.Errorf("only failed jobs can be retried")
	}

	if job.Attempts >= job.MaxAttempts {
		return fmt.Errorf("job has exceeded maximum retry attempts")
	}

	// Reset job status for retry
	updates := map[string]interface{}{
		"status":        models.ProcessingQueued,
		"attempts":      job.Attempts + 1,
		"error_message": "", // Clear previous error
		"started_at":    nil,
		"completed_at":  nil,
	}

	result := r.db.WithContext(ctx).Model(&models.AIProcessingJob{}).
		Where("id = ?", jobID).Updates(updates)

	if result.Error != nil {
		return fmt.Errorf("failed to retry AI processing job: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("AI processing job not found")
	}
	return nil
}

func (r *AIProcessingJobRepository) Delete(ctx context.Context, id uuid.UUID) error {
	// Only allow deletion of completed or failed jobs
	var job models.AIProcessingJob
	err := r.db.WithContext(ctx).Select("status").Where("id = ?", id).First(&job).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("AI processing job not found")
		}
		return fmt.Errorf("failed to check AI processing job status: %w", err)
	}

	if job.Status == models.ProcessingInProgress || job.Status == models.ProcessingQueued {
		return fmt.Errorf("cannot delete active or queued AI processing jobs")
	}

	result := r.db.WithContext(ctx).Delete(&models.AIProcessingJob{}, id)
	if result.Error != nil {
		return fmt.Errorf("failed to delete AI processing job: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("AI processing job not found")
	}
	return nil
}
