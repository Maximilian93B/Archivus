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

type AnalyticsRepository struct {
	db *database.DB
}

func NewAnalyticsRepository(db *database.DB) repositories.AnalyticsRepository {
	return &AnalyticsRepository{db: db}
}

func (r *AnalyticsRepository) CreateDocumentAnalytics(ctx context.Context, analytics *models.DocumentAnalytics) error {
	if err := r.db.WithContext(ctx).Create(analytics).Error; err != nil {
		return fmt.Errorf("failed to create document analytics: %w", err)
	}
	return nil
}

func (r *AnalyticsRepository) UpdateDocumentView(ctx context.Context, documentID uuid.UUID) error {
	// First, try to find existing analytics for this document
	var analytics models.DocumentAnalytics
	err := r.db.WithContext(ctx).Where("document_id = ?", documentID).First(&analytics).Error

	now := time.Now()
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// Create new analytics record
			var doc models.Document
			if err := r.db.WithContext(ctx).Select("tenant_id").Where("id = ?", documentID).First(&doc).Error; err != nil {
				return fmt.Errorf("failed to find document: %w", err)
			}

			analytics = models.DocumentAnalytics{
				TenantID:       doc.TenantID,
				DocumentID:     documentID,
				ViewCount:      1,
				LastAccessedAt: &now,
			}
			return r.CreateDocumentAnalytics(ctx, &analytics)
		}
		return fmt.Errorf("failed to get document analytics: %w", err)
	}

	// Update existing record
	result := r.db.WithContext(ctx).Model(&analytics).Updates(map[string]interface{}{
		"view_count":       gorm.Expr("view_count + 1"),
		"last_accessed_at": now,
	})

	if result.Error != nil {
		return fmt.Errorf("failed to update document view count: %w", result.Error)
	}
	return nil
}

func (r *AnalyticsRepository) UpdateDocumentDownload(ctx context.Context, documentID uuid.UUID) error {
	result := r.db.WithContext(ctx).Model(&models.DocumentAnalytics{}).
		Where("document_id = ?", documentID).
		Update("download_count", gorm.Expr("download_count + 1"))

	if result.Error != nil {
		return fmt.Errorf("failed to update document download count: %w", result.Error)
	}
	return nil
}

func (r *AnalyticsRepository) UpdateDocumentShare(ctx context.Context, documentID uuid.UUID) error {
	result := r.db.WithContext(ctx).Model(&models.DocumentAnalytics{}).
		Where("document_id = ?", documentID).
		Update("share_count", gorm.Expr("share_count + 1"))

	if result.Error != nil {
		return fmt.Errorf("failed to update document share count: %w", result.Error)
	}
	return nil
}

func (r *AnalyticsRepository) GetDocumentStats(ctx context.Context, documentID uuid.UUID) (*models.DocumentAnalytics, error) {
	var analytics models.DocumentAnalytics
	err := r.db.WithContext(ctx).Where("document_id = ?", documentID).First(&analytics).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("document analytics not found")
		}
		return nil, fmt.Errorf("failed to get document analytics: %w", err)
	}
	return &analytics, nil
}

func (r *AnalyticsRepository) GetTenantDashboard(ctx context.Context, tenantID uuid.UUID, period string) (*repositories.DashboardStats, error) {
	var stats repositories.DashboardStats

	// Calculate date range based on period
	var startDate time.Time
	now := time.Now()
	switch period {
	case "day":
		startDate = now.AddDate(0, 0, -1)
	case "week":
		startDate = now.AddDate(0, 0, -7)
	case "month":
		startDate = now.AddDate(0, -1, 0)
	case "year":
		startDate = now.AddDate(-1, 0, 0)
	default:
		startDate = now.AddDate(0, -1, 0) // Default to month
	}

	// Get total documents
	var totalDocs int64
	if err := r.db.WithContext(ctx).Model(&models.Document{}).
		Where("tenant_id = ?", tenantID).Count(&totalDocs).Error; err != nil {
		return nil, fmt.Errorf("failed to count total documents: %w", err)
	}
	stats.TotalDocuments = totalDocs

	// Get storage used
	var storageUsed struct {
		Total int64
	}
	if err := r.db.WithContext(ctx).Model(&models.Document{}).
		Select("COALESCE(SUM(file_size), 0) as total").
		Where("tenant_id = ?", tenantID).Scan(&storageUsed).Error; err != nil {
		return nil, fmt.Errorf("failed to calculate storage used: %w", err)
	}
	stats.StorageUsed = storageUsed.Total

	// Get processing jobs count
	var processingJobs int64
	if err := r.db.WithContext(ctx).Model(&models.AIProcessingJob{}).
		Where("tenant_id = ? AND status IN ?", tenantID, []models.ProcessingStatus{
			models.ProcessingQueued, models.ProcessingInProgress,
		}).Count(&processingJobs).Error; err != nil {
		return nil, fmt.Errorf("failed to count processing jobs: %w", err)
	}
	stats.ProcessingJobs = processingJobs

	// Get pending tasks count
	var pendingTasks int64
	if err := r.db.WithContext(ctx).Model(&models.WorkflowTask{}).
		Joins("JOIN workflows ON workflow_tasks.workflow_id = workflows.id").
		Where("workflows.tenant_id = ? AND workflow_tasks.status = ?", tenantID, models.WorkflowPending).
		Count(&pendingTasks).Error; err != nil {
		return nil, fmt.Errorf("failed to count pending tasks: %w", err)
	}
	stats.PendingTasks = pendingTasks

	// Get documents by type
	var docsByType []struct {
		DocumentType string `json:"document_type"`
		Count        int64  `json:"count"`
	}
	if err := r.db.WithContext(ctx).Model(&models.Document{}).
		Select("document_type, COUNT(*) as count").
		Where("tenant_id = ?", tenantID).
		Group("document_type").Scan(&docsByType).Error; err != nil {
		return nil, fmt.Errorf("failed to get documents by type: %w", err)
	}

	stats.DocumentsByType = make(map[string]int64)
	for _, item := range docsByType {
		stats.DocumentsByType[item.DocumentType] = item.Count
	}

	// Get documents by status
	var docsByStatus []struct {
		Status string `json:"status"`
		Count  int64  `json:"count"`
	}
	if err := r.db.WithContext(ctx).Model(&models.Document{}).
		Select("status, COUNT(*) as count").
		Where("tenant_id = ?", tenantID).
		Group("status").Scan(&docsByStatus).Error; err != nil {
		return nil, fmt.Errorf("failed to get documents by status: %w", err)
	}

	stats.DocumentsByStatus = make(map[string]int64)
	for _, item := range docsByStatus {
		stats.DocumentsByStatus[item.Status] = item.Count
	}

	// Get recent activity (simplified - could be enhanced)
	var recentDocs []models.Document
	if err := r.db.WithContext(ctx).Select("id, title, created_at").
		Where("tenant_id = ? AND created_at >= ?", tenantID, startDate).
		Order("created_at DESC").Limit(10).Find(&recentDocs).Error; err != nil {
		return nil, fmt.Errorf("failed to get recent activity: %w", err)
	}

	stats.RecentActivity = make([]repositories.ActivityItem, 0)
	for _, doc := range recentDocs {
		stats.RecentActivity = append(stats.RecentActivity, repositories.ActivityItem{
			Type:        "document_created",
			Description: fmt.Sprintf("Document '%s' was created", doc.Title),
			CreatedAt:   doc.CreatedAt,
		})
	}

	// Get top users by document count
	var topUsers []repositories.UserStats
	if err := r.db.WithContext(ctx).Model(&models.Document{}).
		Select("created_by as user_id, users.first_name || ' ' || users.last_name as user_name, COUNT(*) as doc_count, COALESCE(SUM(file_size), 0) as storage_used").
		Joins("JOIN users ON documents.created_by = users.id").
		Where("documents.tenant_id = ?", tenantID).
		Group("created_by, users.first_name, users.last_name").
		Order("doc_count DESC").Limit(5).Scan(&topUsers).Error; err != nil {
		return nil, fmt.Errorf("failed to get top users: %w", err)
	}
	stats.TopUsers = topUsers

	return &stats, nil
}

func (r *AnalyticsRepository) GetStorageAnalytics(ctx context.Context, tenantID uuid.UUID) (*repositories.StorageAnalytics, error) {
	var analytics repositories.StorageAnalytics

	// Get total size and document count
	var summary struct {
		TotalSize     int64
		DocumentCount int64
	}
	if err := r.db.WithContext(ctx).Model(&models.Document{}).
		Select("COALESCE(SUM(file_size), 0) as total_size, COUNT(*) as document_count").
		Where("tenant_id = ?", tenantID).Scan(&summary).Error; err != nil {
		return nil, fmt.Errorf("failed to get storage summary: %w", err)
	}

	analytics.TotalSize = summary.TotalSize
	analytics.DocumentCount = summary.DocumentCount
	if summary.DocumentCount > 0 {
		analytics.AverageSize = summary.TotalSize / summary.DocumentCount
	}

	// Get size by document type
	var sizeByType []struct {
		DocumentType string `json:"document_type"`
		TotalSize    int64  `json:"total_size"`
	}
	if err := r.db.WithContext(ctx).Model(&models.Document{}).
		Select("document_type, COALESCE(SUM(file_size), 0) as total_size").
		Where("tenant_id = ?", tenantID).
		Group("document_type").Scan(&sizeByType).Error; err != nil {
		return nil, fmt.Errorf("failed to get size by type: %w", err)
	}

	analytics.SizeByType = make(map[string]int64)
	for _, item := range sizeByType {
		analytics.SizeByType[item.DocumentType] = item.TotalSize
	}

	// Get largest documents
	var largestDocs []repositories.DocumentSizeInfo
	if err := r.db.WithContext(ctx).Model(&models.Document{}).
		Select("id as document_id, title as document_name, file_size as size").
		Where("tenant_id = ?", tenantID).
		Order("file_size DESC").Limit(10).Scan(&largestDocs).Error; err != nil {
		return nil, fmt.Errorf("failed to get largest documents: %w", err)
	}
	analytics.LargestDocuments = largestDocs

	// TODO: Implement growth trend analysis
	// This would require historical data tracking
	analytics.GrowthTrend = make([]repositories.StoragePoint, 0)

	return &analytics, nil
}

func (r *AnalyticsRepository) GetUserActivity(ctx context.Context, tenantID uuid.UUID, days int) ([]repositories.UserActivityStats, error) {
	var activities []repositories.UserActivityStats
	startDate := time.Now().AddDate(0, 0, -days)

	err := r.db.WithContext(ctx).Model(&models.User{}).
		Select(`
			users.id as user_id,
			users.first_name || ' ' || users.last_name as user_name,
			COALESCE(doc_counts.documents_created, 0) as documents_created,
			COALESCE(view_counts.documents_viewed, 0) as documents_viewed,
			COALESCE(users.last_login_at, users.created_at) as last_activity
		`).
		Joins(`LEFT JOIN (
			SELECT created_by, COUNT(*) as documents_created 
			FROM documents 
			WHERE created_at >= ? 
			GROUP BY created_by
		) doc_counts ON users.id = doc_counts.created_by`, startDate).
		Joins(`LEFT JOIN (
			SELECT 
				documents.created_by,
				COALESCE(SUM(document_analytics.view_count), 0) as documents_viewed
			FROM documents
			LEFT JOIN document_analytics ON documents.id = document_analytics.document_id
			WHERE documents.created_at >= ?
			GROUP BY documents.created_by
		) view_counts ON users.id = view_counts.created_by`, startDate).
		Where("users.tenant_id = ?", tenantID).
		Scan(&activities).Error

	if err != nil {
		return nil, fmt.Errorf("failed to get user activity: %w", err)
	}

	return activities, nil
}
