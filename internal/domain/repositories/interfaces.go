package repositories

import (
	"context"
	"time"

	"github.com/archivus/archivus/internal/infrastructure/database/models"
	"github.com/google/uuid"
)

// Core repository interfaces for clean architecture

type TenantRepository interface {
	Create(ctx context.Context, tenant *models.Tenant) error
	GetByID(ctx context.Context, id uuid.UUID) (*models.Tenant, error)
	GetBySubdomain(ctx context.Context, subdomain string) (*models.Tenant, error)
	Update(ctx context.Context, tenant *models.Tenant) error
	UpdateUsage(ctx context.Context, tenantID uuid.UUID, storageUsed int64, apiUsed int) error
	CheckQuotaLimits(ctx context.Context, tenantID uuid.UUID) (*QuotaStatus, error)
	List(ctx context.Context, params ListParams) ([]models.Tenant, int64, error)
	Delete(ctx context.Context, id uuid.UUID) error
}

type UserRepository interface {
	Create(ctx context.Context, user *models.User) error
	GetByID(ctx context.Context, id uuid.UUID) (*models.User, error)
	GetByEmail(ctx context.Context, tenantID uuid.UUID, email string) (*models.User, error)
	Update(ctx context.Context, user *models.User) error
	UpdateLastLogin(ctx context.Context, userID uuid.UUID) error
	ListByTenant(ctx context.Context, tenantID uuid.UUID, params ListParams) ([]models.User, int64, error)
	SetMFA(ctx context.Context, userID uuid.UUID, enabled bool, secret string) error
	Delete(ctx context.Context, id uuid.UUID) error
}

type DocumentRepository interface {
	Create(ctx context.Context, document *models.Document) error
	GetByID(ctx context.Context, id uuid.UUID) (*models.Document, error)
	GetByContentHash(ctx context.Context, tenantID uuid.UUID, hash string) (*models.Document, error)
	Update(ctx context.Context, document *models.Document) error
	List(ctx context.Context, tenantID uuid.UUID, filters DocumentFilters) ([]models.Document, int64, error)
	Search(ctx context.Context, tenantID uuid.UUID, query SearchQuery) ([]models.Document, error)
	SemanticSearch(ctx context.Context, tenantID uuid.UUID, embedding []float32, limit int) ([]models.Document, error)
	GetByFolder(ctx context.Context, folderID uuid.UUID, params ListParams) ([]models.Document, int64, error)
	GetByTags(ctx context.Context, tenantID uuid.UUID, tagIDs []uuid.UUID) ([]models.Document, error)
	GetByCategories(ctx context.Context, tenantID uuid.UUID, categoryIDs []uuid.UUID) ([]models.Document, error)
	GetDuplicates(ctx context.Context, tenantID uuid.UUID, threshold float64) ([]DocumentDuplicate, error)
	GetExpiring(ctx context.Context, tenantID uuid.UUID, days int) ([]models.Document, error)
	GetFinancialDocuments(ctx context.Context, tenantID uuid.UUID, filters FinancialFilters) ([]models.Document, error)
	UpdateStatus(ctx context.Context, id uuid.UUID, status models.DocStatus) error
	AssociateTags(ctx context.Context, documentID uuid.UUID, tagIDs []uuid.UUID) error
	AssociateCategories(ctx context.Context, documentID uuid.UUID, categoryIDs []uuid.UUID) error
	SoftDelete(ctx context.Context, id uuid.UUID, deletedBy uuid.UUID) error
	Delete(ctx context.Context, id uuid.UUID) error
}

type FolderRepository interface {
	Create(ctx context.Context, folder *models.Folder) error
	GetByID(ctx context.Context, id uuid.UUID) (*models.Folder, error)
	GetByPath(ctx context.Context, tenantID uuid.UUID, path string) (*models.Folder, error)
	Update(ctx context.Context, folder *models.Folder) error
	GetChildren(ctx context.Context, parentID uuid.UUID) ([]models.Folder, error)
	GetTree(ctx context.Context, tenantID uuid.UUID) ([]FolderNode, error)
	GetDocumentCount(ctx context.Context, folderID uuid.UUID) (int64, error)
	Move(ctx context.Context, folderID, newParentID uuid.UUID) error
	Delete(ctx context.Context, id uuid.UUID) error
}

type TagRepository interface {
	Create(ctx context.Context, tag *models.Tag) error
	GetByID(ctx context.Context, id uuid.UUID) (*models.Tag, error)
	GetByName(ctx context.Context, tenantID uuid.UUID, name string) (*models.Tag, error)
	Update(ctx context.Context, tag *models.Tag) error
	ListByTenant(ctx context.Context, tenantID uuid.UUID) ([]models.Tag, error)
	GetPopular(ctx context.Context, tenantID uuid.UUID, limit int) ([]models.Tag, error)
	IncrementUsage(ctx context.Context, tagID uuid.UUID) error
	BulkCreate(ctx context.Context, tags []models.Tag) error
	Delete(ctx context.Context, id uuid.UUID) error
}

type CategoryRepository interface {
	Create(ctx context.Context, category *models.Category) error
	GetByID(ctx context.Context, id uuid.UUID) (*models.Category, error)
	GetByName(ctx context.Context, tenantID uuid.UUID, name string) (*models.Category, error)
	Update(ctx context.Context, category *models.Category) error
	ListByTenant(ctx context.Context, tenantID uuid.UUID) ([]models.Category, error)
	GetDocumentCount(ctx context.Context, categoryID uuid.UUID) (int64, error)
	Delete(ctx context.Context, id uuid.UUID) error
}

type WorkflowRepository interface {
	Create(ctx context.Context, workflow *models.Workflow) error
	GetByID(ctx context.Context, id uuid.UUID) (*models.Workflow, error)
	Update(ctx context.Context, workflow *models.Workflow) error
	ListByTenant(ctx context.Context, tenantID uuid.UUID) ([]models.Workflow, error)
	GetByDocumentType(ctx context.Context, tenantID uuid.UUID, docType models.DocumentType) ([]models.Workflow, error)
	Delete(ctx context.Context, id uuid.UUID) error
}

type WorkflowTaskRepository interface {
	Create(ctx context.Context, task *models.WorkflowTask) error
	GetByID(ctx context.Context, id uuid.UUID) (*models.WorkflowTask, error)
	Update(ctx context.Context, task *models.WorkflowTask) error
	ListByAssignee(ctx context.Context, userID uuid.UUID, status models.WorkflowStatus) ([]models.WorkflowTask, error)
	ListByDocument(ctx context.Context, documentID uuid.UUID) ([]models.WorkflowTask, error)
	GetPendingTasks(ctx context.Context, tenantID uuid.UUID) ([]models.WorkflowTask, error)
	GetOverdueTasks(ctx context.Context, tenantID uuid.UUID) ([]models.WorkflowTask, error)
	Complete(ctx context.Context, taskID uuid.UUID, completedBy uuid.UUID, comments string) error
	Delete(ctx context.Context, id uuid.UUID) error
}

type AIProcessingJobRepository interface {
	Create(ctx context.Context, job *models.AIProcessingJob) error
	GetByID(ctx context.Context, id uuid.UUID) (*models.AIProcessingJob, error)
	GetNextJob(ctx context.Context) (*models.AIProcessingJob, error)
	Update(ctx context.Context, job *models.AIProcessingJob) error
	UpdateStatus(ctx context.Context, jobID uuid.UUID, status models.ProcessingStatus) error
	ListByDocument(ctx context.Context, documentID uuid.UUID) ([]models.AIProcessingJob, error)
	GetFailedJobs(ctx context.Context, tenantID uuid.UUID) ([]models.AIProcessingJob, error)
	RetryJob(ctx context.Context, jobID uuid.UUID) error
	Delete(ctx context.Context, id uuid.UUID) error
}

type AuditLogRepository interface {
	Create(ctx context.Context, log *models.AuditLog) error
	ListByResource(ctx context.Context, resourceID uuid.UUID, resourceType string, params ListParams) ([]models.AuditLog, int64, error)
	ListByUser(ctx context.Context, userID uuid.UUID, params ListParams) ([]models.AuditLog, int64, error)
	ListByTenant(ctx context.Context, tenantID uuid.UUID, params ListParams) ([]models.AuditLog, int64, error)
	GetSecurityEvents(ctx context.Context, tenantID uuid.UUID, since time.Time) ([]models.AuditLog, error)
}

type ShareRepository interface {
	Create(ctx context.Context, share *models.Share) error
	GetByToken(ctx context.Context, token string) (*models.Share, error)
	GetByDocument(ctx context.Context, documentID uuid.UUID) ([]models.Share, error)
	Update(ctx context.Context, share *models.Share) error
	IncrementDownload(ctx context.Context, shareID uuid.UUID) error
	ExpireShare(ctx context.Context, shareID uuid.UUID) error
	ListByCreator(ctx context.Context, creatorID uuid.UUID) ([]models.Share, error)
	Delete(ctx context.Context, id uuid.UUID) error
}

type AnalyticsRepository interface {
	CreateDocumentAnalytics(ctx context.Context, analytics *models.DocumentAnalytics) error
	UpdateDocumentView(ctx context.Context, documentID uuid.UUID) error
	UpdateDocumentDownload(ctx context.Context, documentID uuid.UUID) error
	UpdateDocumentShare(ctx context.Context, documentID uuid.UUID) error
	GetDocumentStats(ctx context.Context, documentID uuid.UUID) (*models.DocumentAnalytics, error)
	GetTenantDashboard(ctx context.Context, tenantID uuid.UUID, period string) (*DashboardStats, error)
	GetStorageAnalytics(ctx context.Context, tenantID uuid.UUID) (*StorageAnalytics, error)
	GetUserActivity(ctx context.Context, tenantID uuid.UUID, days int) ([]UserActivityStats, error)
}

type NotificationRepository interface {
	Create(ctx context.Context, notification *models.Notification) error
	GetByID(ctx context.Context, id uuid.UUID) (*models.Notification, error)
	ListByUser(ctx context.Context, userID uuid.UUID, params ListParams) ([]models.Notification, int64, error)
	ListByTenant(ctx context.Context, tenantID uuid.UUID, params ListParams) ([]models.Notification, int64, error)
	MarkAsRead(ctx context.Context, notificationID uuid.UUID) error
	MarkAllAsRead(ctx context.Context, userID uuid.UUID) error
	Delete(ctx context.Context, id uuid.UUID) error
}

// Supporting types for repository operations

type ListParams struct {
	Page     int    `json:"page"`
	PageSize int    `json:"page_size"`
	SortBy   string `json:"sort_by"`
	SortDesc bool   `json:"sort_desc"`
	Search   string `json:"search"`
}

type DocumentFilters struct {
	Status       []models.DocStatus        `json:"status"`
	DocumentType []models.DocumentType     `json:"document_type"`
	CreatedBy    []uuid.UUID               `json:"created_by"`
	FolderID     *uuid.UUID                `json:"folder_id"`
	TagIDs       []uuid.UUID               `json:"tag_ids"`
	CategoryIDs  []uuid.UUID               `json:"category_ids"`
	DateFrom     *time.Time                `json:"date_from"`
	DateTo       *time.Time                `json:"date_to"`
	MinSize      *int64                    `json:"min_size"`
	MaxSize      *int64                    `json:"max_size"`
	HasAI        *bool                     `json:"has_ai"`
	Compliance   []models.ComplianceStatus `json:"compliance"`
	ListParams
}

type SearchQuery struct {
	Query         string                `json:"query"`
	DocumentTypes []models.DocumentType `json:"document_types"`
	FolderIDs     []uuid.UUID           `json:"folder_ids"`
	TagIDs        []uuid.UUID           `json:"tag_ids"`
	DateFrom      *time.Time            `json:"date_from"`
	DateTo        *time.Time            `json:"date_to"`
	Fuzzy         bool                  `json:"fuzzy"`
	Limit         int                   `json:"limit"`
}

type FinancialFilters struct {
	MinAmount     *float64   `json:"min_amount"`
	MaxAmount     *float64   `json:"max_amount"`
	VendorNames   []string   `json:"vendor_names"`
	CustomerNames []string   `json:"customer_names"`
	Currency      string     `json:"currency"`
	DateFrom      *time.Time `json:"date_from"`
	DateTo        *time.Time `json:"date_to"`
	ListParams
}

type QuotaStatus struct {
	StorageUsed    int64   `json:"storage_used"`
	StorageQuota   int64   `json:"storage_quota"`
	StoragePercent float64 `json:"storage_percent"`
	APIUsed        int     `json:"api_used"`
	APIQuota       int     `json:"api_quota"`
	APIPercent     float64 `json:"api_percent"`
	CanUpload      bool    `json:"can_upload"`
	CanProcessAI   bool    `json:"can_process_ai"`
}

type DocumentDuplicate struct {
	OriginalID   uuid.UUID `json:"original_id"`
	DuplicateID  uuid.UUID `json:"duplicate_id"`
	Similarity   float64   `json:"similarity"`
	ContentMatch bool      `json:"content_match"`
}

type FolderNode struct {
	*models.Folder
	Children      []FolderNode `json:"children"`
	DocumentCount int64        `json:"document_count"`
}

type DashboardStats struct {
	TotalDocuments    int64            `json:"total_documents"`
	StorageUsed       int64            `json:"storage_used"`
	ProcessingJobs    int64            `json:"processing_jobs"`
	PendingTasks      int64            `json:"pending_tasks"`
	DocumentsByType   map[string]int64 `json:"documents_by_type"`
	DocumentsByStatus map[string]int64 `json:"documents_by_status"`
	RecentActivity    []ActivityItem   `json:"recent_activity"`
	TopUsers          []UserStats      `json:"top_users"`
}

type StorageAnalytics struct {
	TotalSize        int64              `json:"total_size"`
	DocumentCount    int64              `json:"document_count"`
	AverageSize      int64              `json:"average_size"`
	SizeByType       map[string]int64   `json:"size_by_type"`
	GrowthTrend      []StoragePoint     `json:"growth_trend"`
	LargestDocuments []DocumentSizeInfo `json:"largest_documents"`
}

type UserActivityStats struct {
	UserID           uuid.UUID `json:"user_id"`
	UserName         string    `json:"user_name"`
	DocumentsCreated int       `json:"documents_created"`
	DocumentsViewed  int       `json:"documents_viewed"`
	LastActivity     time.Time `json:"last_activity"`
}

type ActivityItem struct {
	Type        string    `json:"type"`
	Description string    `json:"description"`
	UserName    string    `json:"user_name"`
	CreatedAt   time.Time `json:"created_at"`
}

type UserStats struct {
	UserID      uuid.UUID `json:"user_id"`
	UserName    string    `json:"user_name"`
	DocCount    int64     `json:"doc_count"`
	StorageUsed int64     `json:"storage_used"`
}

type StoragePoint struct {
	Date time.Time `json:"date"`
	Size int64     `json:"size"`
}

type DocumentSizeInfo struct {
	DocumentID   uuid.UUID `json:"document_id"`
	DocumentName string    `json:"document_name"`
	Size         int64     `json:"size"`
}
