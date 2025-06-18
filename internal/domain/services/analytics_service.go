package services

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/archivus/archivus/internal/domain/repositories"
	"github.com/google/uuid"
)

var (
	ErrInvalidDateRange = errors.New("invalid date range")
	ErrInvalidPeriod    = errors.New("invalid period specified")
	ErrNoDataAvailable  = errors.New("no data available for the specified period")
)

// AnalyticsService provides business intelligence and insights
type AnalyticsService struct {
	analyticsRepo repositories.AnalyticsRepository
	documentRepo  repositories.DocumentRepository
	userRepo      repositories.UserRepository
	tenantRepo    repositories.TenantRepository
	auditRepo     repositories.AuditLogRepository

	config AnalyticsServiceConfig
}

// AnalyticsServiceConfig holds configuration for analytics
type AnalyticsServiceConfig struct {
	DefaultCacheTTL       time.Duration
	MaxDataPointsPerChart int
	EnableRealTimeUpdates bool
	RetentionDays         int
}

// NewAnalyticsService creates a new analytics service
func NewAnalyticsService(
	analyticsRepo repositories.AnalyticsRepository,
	documentRepo repositories.DocumentRepository,
	userRepo repositories.UserRepository,
	tenantRepo repositories.TenantRepository,
	auditRepo repositories.AuditLogRepository,
	config AnalyticsServiceConfig,
) *AnalyticsService {
	return &AnalyticsService{
		analyticsRepo: analyticsRepo,
		documentRepo:  documentRepo,
		userRepo:      userRepo,
		tenantRepo:    tenantRepo,
		auditRepo:     auditRepo,
		config:        config,
	}
}

// DashboardData contains comprehensive dashboard information
type DashboardData struct {
	Summary           *DashboardSummary  `json:"summary"`
	DocumentTrends    *DocumentTrends    `json:"document_trends"`
	UserActivity      *UserActivityData  `json:"user_activity"`
	StorageAnalytics  *StorageAnalytics  `json:"storage_analytics"`
	WorkflowMetrics   *WorkflowMetrics   `json:"workflow_metrics"`
	ComplianceStatus  *ComplianceMetrics `json:"compliance_status"`
	FinancialInsights *FinancialInsights `json:"financial_insights"`
	AIProcessingStats *AIProcessingStats `json:"ai_processing_stats"`
	RecentActivity    []ActivitySummary  `json:"recent_activity"`
	Alerts            []SystemAlert      `json:"alerts"`
}

// DashboardSummary provides high-level KPIs
type DashboardSummary struct {
	TotalDocuments     int64   `json:"total_documents"`
	DocumentsThisMonth int64   `json:"documents_this_month"`
	StorageUsed        int64   `json:"storage_used_bytes"`
	StorageQuota       int64   `json:"storage_quota_bytes"`
	ActiveUsers        int64   `json:"active_users"`
	PendingTasks       int64   `json:"pending_tasks"`
	ProcessingJobs     int64   `json:"processing_jobs"`
	GrowthRate         float64 `json:"growth_rate_percent"`
}

// DocumentTrends shows document creation and processing trends
type DocumentTrends struct {
	Period            string                `json:"period"`
	TotalDocuments    int64                 `json:"total_documents"`
	DocumentsByType   map[string]int64      `json:"documents_by_type"`
	DocumentsByStatus map[string]int64      `json:"documents_by_status"`
	CreationTrend     []TimeSeriesDataPoint `json:"creation_trend"`
	ProcessingTrend   []TimeSeriesDataPoint `json:"processing_trend"`
	PopularCategories []CategoryUsage       `json:"popular_categories"`
	TopTags           []TagUsage            `json:"top_tags"`
}

// UserActivityData shows user engagement metrics
type UserActivityData struct {
	ActiveUsersToday int64                 `json:"active_users_today"`
	ActiveUsersWeek  int64                 `json:"active_users_week"`
	ActiveUsersMonth int64                 `json:"active_users_month"`
	TopUsers         []UserActivitySummary `json:"top_users"`
	ActivityByHour   []HourlyActivity      `json:"activity_by_hour"`
	ActivityByDay    []DailyActivity       `json:"activity_by_day"`
	LoginTrends      []TimeSeriesDataPoint `json:"login_trends"`
	UserGrowth       []TimeSeriesDataPoint `json:"user_growth"`
}

// StorageAnalytics provides storage usage insights
type StorageAnalytics struct {
	TotalSize        int64                 `json:"total_size_bytes"`
	UsedSize         int64                 `json:"used_size_bytes"`
	AvailableSize    int64                 `json:"available_size_bytes"`
	UsagePercent     float64               `json:"usage_percent"`
	SizeByDocType    map[string]int64      `json:"size_by_document_type"`
	SizeByCategory   map[string]int64      `json:"size_by_category"`
	GrowthTrend      []TimeSeriesDataPoint `json:"growth_trend"`
	LargestDocuments []DocumentSizeInfo    `json:"largest_documents"`
	StorageHotspots  []StorageHotspot      `json:"storage_hotspots"`
	PredictedFull    *time.Time            `json:"predicted_full_date,omitempty"`
}

// WorkflowMetrics shows workflow and task performance
type WorkflowMetrics struct {
	TotalWorkflows     int64                `json:"total_workflows"`
	ActiveWorkflows    int64                `json:"active_workflows"`
	PendingTasks       int64                `json:"pending_tasks"`
	CompletedTasks     int64                `json:"completed_tasks"`
	AverageTaskTime    float64              `json:"average_task_time_hours"`
	TasksByStatus      map[string]int64     `json:"tasks_by_status"`
	TasksByType        map[string]int64     `json:"tasks_by_type"`
	WorkflowEfficiency []WorkflowEfficiency `json:"workflow_efficiency"`
	OverdueTasks       []OverdueTaskSummary `json:"overdue_tasks"`
	TaskCompletionRate float64              `json:"task_completion_rate"`
}

// ComplianceMetrics shows compliance and audit status
type ComplianceMetrics struct {
	ComplianceScore     float64                     `json:"compliance_score"`
	CompliantDocuments  int64                       `json:"compliant_documents"`
	NonCompliantDocs    int64                       `json:"non_compliant_documents"`
	PendingReview       int64                       `json:"pending_review"`
	RetentionViolations int64                       `json:"retention_violations"`
	AuditEvents         []AuditEventSummary         `json:"recent_audit_events"`
	ComplianceByType    map[string]ComplianceStatus `json:"compliance_by_type"`
	SecurityAlerts      []SecurityAlert             `json:"security_alerts"`
}

// FinancialInsights shows financial document analytics
type FinancialInsights struct {
	TotalInvoices     int64                 `json:"total_invoices"`
	TotalAmount       float64               `json:"total_amount"`
	AverageAmount     float64               `json:"average_amount"`
	Currency          string                `json:"primary_currency"`
	AmountByMonth     []MonthlyFinancial    `json:"amount_by_month"`
	TopVendors        []VendorSummary       `json:"top_vendors"`
	TopCustomers      []CustomerSummary     `json:"top_customers"`
	PaymentTrends     []TimeSeriesDataPoint `json:"payment_trends"`
	ExpiringDocuments []ExpiringDocument    `json:"expiring_documents"`
	TaxAnalysis       *TaxAnalysis          `json:"tax_analysis"`
}

// AIProcessingStats shows AI processing metrics
type AIProcessingStats struct {
	TotalJobsProcessed    int64                 `json:"total_jobs_processed"`
	SuccessfulJobs        int64                 `json:"successful_jobs"`
	FailedJobs            int64                 `json:"failed_jobs"`
	AverageProcessingTime float64               `json:"average_processing_time_ms"`
	JobsByType            map[string]int64      `json:"jobs_by_type"`
	ProcessingTrend       []TimeSeriesDataPoint `json:"processing_trend"`
	AccuracyMetrics       map[string]float64    `json:"accuracy_metrics"`
	CostAnalysis          *AIProcessingCost     `json:"cost_analysis"`
}

// Supporting types for analytics data

type TimeSeriesDataPoint struct {
	Timestamp time.Time `json:"timestamp"`
	Value     float64   `json:"value"`
	Label     string    `json:"label,omitempty"`
}

type CategoryUsage struct {
	CategoryName  string  `json:"category_name"`
	DocumentCount int64   `json:"document_count"`
	UsagePercent  float64 `json:"usage_percent"`
}

type TagUsage struct {
	TagName       string `json:"tag_name"`
	UsageCount    int64  `json:"usage_count"`
	IsAIGenerated bool   `json:"is_ai_generated"`
}

type UserActivitySummary struct {
	UserID           uuid.UUID `json:"user_id"`
	UserName         string    `json:"user_name"`
	DocumentsCreated int64     `json:"documents_created"`
	DocumentsViewed  int64     `json:"documents_viewed"`
	TasksCompleted   int64     `json:"tasks_completed"`
	LastActivity     time.Time `json:"last_activity"`
}

type HourlyActivity struct {
	Hour    int   `json:"hour"`
	Actions int64 `json:"actions"`
}

type DailyActivity struct {
	Date    time.Time `json:"date"`
	Actions int64     `json:"actions"`
}

type DocumentSizeInfo struct {
	DocumentID   uuid.UUID `json:"document_id"`
	DocumentName string    `json:"document_name"`
	Size         int64     `json:"size_bytes"`
	CreatedAt    time.Time `json:"created_at"`
}

type StorageHotspot struct {
	FolderID      uuid.UUID `json:"folder_id"`
	FolderName    string    `json:"folder_name"`
	Size          int64     `json:"size_bytes"`
	DocumentCount int64     `json:"document_count"`
}

type WorkflowEfficiency struct {
	WorkflowID     uuid.UUID `json:"workflow_id"`
	WorkflowName   string    `json:"workflow_name"`
	AverageTime    float64   `json:"average_time_hours"`
	CompletionRate float64   `json:"completion_rate"`
	BottleneckStep string    `json:"bottleneck_step,omitempty"`
}

type OverdueTaskSummary struct {
	TaskID       uuid.UUID `json:"task_id"`
	DocumentName string    `json:"document_name"`
	AssigneeName string    `json:"assignee_name"`
	DueDate      time.Time `json:"due_date"`
	DaysOverdue  int       `json:"days_overdue"`
}

type AuditEventSummary struct {
	EventType    string    `json:"event_type"`
	Count        int64     `json:"count"`
	LastOccurred time.Time `json:"last_occurred"`
}

type ComplianceStatus struct {
	Status     string  `json:"status"`
	Count      int64   `json:"count"`
	Percentage float64 `json:"percentage"`
}

type SecurityAlert struct {
	AlertType   string    `json:"alert_type"`
	Severity    string    `json:"severity"`
	Description string    `json:"description"`
	Timestamp   time.Time `json:"timestamp"`
}

type MonthlyFinancial struct {
	Month         time.Time `json:"month"`
	TotalAmount   float64   `json:"total_amount"`
	DocumentCount int64     `json:"document_count"`
}

type VendorSummary struct {
	VendorName    string  `json:"vendor_name"`
	TotalAmount   float64 `json:"total_amount"`
	DocumentCount int64   `json:"document_count"`
}

type CustomerSummary struct {
	CustomerName  string  `json:"customer_name"`
	TotalAmount   float64 `json:"total_amount"`
	DocumentCount int64   `json:"document_count"`
}

type ExpiringDocument struct {
	DocumentID      uuid.UUID `json:"document_id"`
	DocumentName    string    `json:"document_name"`
	ExpiryDate      time.Time `json:"expiry_date"`
	DaysUntilExpiry int       `json:"days_until_expiry"`
}

type TaxAnalysis struct {
	TotalTaxAmount float64            `json:"total_tax_amount"`
	AverageTaxRate float64            `json:"average_tax_rate"`
	TaxByMonth     []MonthlyFinancial `json:"tax_by_month"`
}

type AIProcessingCost struct {
	TotalCost     float64               `json:"total_cost"`
	CostThisMonth float64               `json:"cost_this_month"`
	CostByJobType map[string]float64    `json:"cost_by_job_type"`
	CostTrend     []TimeSeriesDataPoint `json:"cost_trend"`
}

type ActivitySummary struct {
	Type        string    `json:"type"`
	Description string    `json:"description"`
	UserName    string    `json:"user_name"`
	Timestamp   time.Time `json:"timestamp"`
}

type SystemAlert struct {
	ID          uuid.UUID `json:"id"`
	Type        string    `json:"type"`
	Severity    string    `json:"severity"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Timestamp   time.Time `json:"timestamp"`
	IsRead      bool      `json:"is_read"`
}

// GetDashboard returns comprehensive dashboard data
func (s *AnalyticsService) GetDashboard(ctx context.Context, tenantID uuid.UUID, period string) (*DashboardData, error) {
	// Validate period
	if err := s.validatePeriod(period); err != nil {
		return nil, err
	}

	// Get dashboard stats from repository
	dashboardStats, err := s.analyticsRepo.GetTenantDashboard(ctx, tenantID, period)
	if err != nil {
		return nil, fmt.Errorf("failed to get dashboard stats: %w", err)
	}

	// Build comprehensive dashboard data
	dashboard := &DashboardData{
		Summary:           s.buildDashboardSummary(dashboardStats),
		DocumentTrends:    s.getDocumentTrends(ctx, tenantID, period),
		UserActivity:      s.getUserActivity(ctx, tenantID, period),
		StorageAnalytics:  s.getStorageAnalytics(ctx, tenantID),
		WorkflowMetrics:   s.getWorkflowMetrics(ctx, tenantID, period),
		ComplianceStatus:  s.getComplianceMetrics(ctx, tenantID),
		FinancialInsights: s.getFinancialInsights(ctx, tenantID, period),
		AIProcessingStats: s.getAIProcessingStats(ctx, tenantID, period),
		RecentActivity:    s.getRecentActivity(ctx, tenantID, 50),
		Alerts:            s.getSystemAlerts(ctx, tenantID),
	}

	return dashboard, nil
}

// GetDocumentAnalytics returns detailed document analytics
func (s *AnalyticsService) GetDocumentAnalytics(ctx context.Context, tenantID uuid.UUID, filters AnalyticsFilters) (*DocumentTrends, error) {
	if err := s.validateDateRange(filters.DateFrom, filters.DateTo); err != nil {
		return nil, err
	}

	return s.getDocumentTrends(ctx, tenantID, filters.Period), nil
}

// GetUserAnalytics returns user activity analytics
func (s *AnalyticsService) GetUserAnalytics(ctx context.Context, tenantID uuid.UUID, filters AnalyticsFilters) (*UserActivityData, error) {
	if err := s.validateDateRange(filters.DateFrom, filters.DateTo); err != nil {
		return nil, err
	}

	return s.getUserActivity(ctx, tenantID, filters.Period), nil
}

// GetStorageReport returns detailed storage analytics
func (s *AnalyticsService) GetStorageReport(ctx context.Context, tenantID uuid.UUID) (*StorageAnalytics, error) {
	repoAnalytics, err := s.analyticsRepo.GetStorageAnalytics(ctx, tenantID)
	if err != nil {
		return nil, fmt.Errorf("failed to get storage analytics: %w", err)
	}

	// Convert repository type to service type
	storageAnalytics := &StorageAnalytics{
		TotalSize:     repoAnalytics.TotalSize,
		UsedSize:      repoAnalytics.TotalSize,
		AvailableSize: 0,
		UsagePercent:  0,
		SizeByDocType: make(map[string]int64),
	}

	// Enhance with predictions and insights
	s.enhanceStorageAnalytics(storageAnalytics)

	return storageAnalytics, nil
}

// GetComplianceReport returns compliance and audit metrics
func (s *AnalyticsService) GetComplianceReport(ctx context.Context, tenantID uuid.UUID) (*ComplianceMetrics, error) {
	return s.getComplianceMetrics(ctx, tenantID), nil
}

// GetFinancialReport returns financial document insights
func (s *AnalyticsService) GetFinancialReport(ctx context.Context, tenantID uuid.UUID, filters AnalyticsFilters) (*FinancialInsights, error) {
	if err := s.validateDateRange(filters.DateFrom, filters.DateTo); err != nil {
		return nil, err
	}

	return s.getFinancialInsights(ctx, tenantID, filters.Period), nil
}

// ExportAnalytics exports analytics data in various formats
func (s *AnalyticsService) ExportAnalytics(ctx context.Context, tenantID uuid.UUID, exportType string, filters AnalyticsFilters) ([]byte, error) {
	// Get analytics data
	dashboard, err := s.GetDashboard(ctx, tenantID, filters.Period)
	if err != nil {
		return nil, err
	}

	// Export based on type
	switch exportType {
	case "csv":
		return s.exportCSV(dashboard)
	case "xlsx":
		return s.exportExcel(dashboard)
	case "pdf":
		return s.exportPDF(dashboard)
	default:
		return nil, errors.New("unsupported export format")
	}
}

// Helper methods

func (s *AnalyticsService) validatePeriod(period string) error {
	validPeriods := []string{"day", "week", "month", "quarter", "year"}
	for _, valid := range validPeriods {
		if period == valid {
			return nil
		}
	}
	return ErrInvalidPeriod
}

func (s *AnalyticsService) validateDateRange(from, to *time.Time) error {
	if from != nil && to != nil && from.After(*to) {
		return ErrInvalidDateRange
	}
	return nil
}

func (s *AnalyticsService) buildDashboardSummary(stats *repositories.DashboardStats) *DashboardSummary {
	return &DashboardSummary{
		TotalDocuments:     stats.TotalDocuments,
		DocumentsThisMonth: s.getDocumentsThisMonth(stats),
		StorageUsed:        stats.StorageUsed,
		ActiveUsers:        int64(len(stats.TopUsers)),
		PendingTasks:       stats.PendingTasks,
		ProcessingJobs:     stats.ProcessingJobs,
		GrowthRate:         s.calculateGrowthRate(stats),
	}
}

func (s *AnalyticsService) getDocumentTrends(ctx context.Context, tenantID uuid.UUID, period string) *DocumentTrends {
	// Implementation would query document repository with date filters
	// This is a simplified version
	return &DocumentTrends{
		Period:            period,
		TotalDocuments:    0,
		DocumentsByType:   make(map[string]int64),
		DocumentsByStatus: make(map[string]int64),
		CreationTrend:     []TimeSeriesDataPoint{},
		ProcessingTrend:   []TimeSeriesDataPoint{},
	}
}

func (s *AnalyticsService) getUserActivity(ctx context.Context, tenantID uuid.UUID, period string) *UserActivityData {
	// Implementation would query user activity data
	return &UserActivityData{
		ActiveUsersToday: 0,
		TopUsers:         []UserActivitySummary{},
		ActivityByHour:   []HourlyActivity{},
		LoginTrends:      []TimeSeriesDataPoint{},
	}
}

func (s *AnalyticsService) getStorageAnalytics(ctx context.Context, tenantID uuid.UUID) *StorageAnalytics {
	// Get storage analytics from repository and convert
	repoAnalytics, _ := s.analyticsRepo.GetStorageAnalytics(ctx, tenantID)
	if repoAnalytics == nil {
		return &StorageAnalytics{}
	}

	return &StorageAnalytics{
		TotalSize:     repoAnalytics.TotalSize,
		UsedSize:      repoAnalytics.TotalSize,
		AvailableSize: 0,
		UsagePercent:  0,
		SizeByDocType: make(map[string]int64),
	}
}

func (s *AnalyticsService) getWorkflowMetrics(ctx context.Context, tenantID uuid.UUID, period string) *WorkflowMetrics {
	// Implementation would aggregate workflow and task data
	return &WorkflowMetrics{
		TotalWorkflows: 0,
		PendingTasks:   0,
		TasksByStatus:  make(map[string]int64),
		TasksByType:    make(map[string]int64),
	}
}

func (s *AnalyticsService) getComplianceMetrics(ctx context.Context, tenantID uuid.UUID) *ComplianceMetrics {
	// Implementation would analyze compliance status
	return &ComplianceMetrics{
		ComplianceScore:    0.0,
		CompliantDocuments: 0,
		ComplianceByType:   make(map[string]ComplianceStatus),
	}
}

func (s *AnalyticsService) getFinancialInsights(ctx context.Context, tenantID uuid.UUID, period string) *FinancialInsights {
	// Implementation would analyze financial documents
	return &FinancialInsights{
		TotalInvoices: 0,
		TotalAmount:   0.0,
		Currency:      "CAD",
		TopVendors:    []VendorSummary{},
		TopCustomers:  []CustomerSummary{},
	}
}

func (s *AnalyticsService) getAIProcessingStats(ctx context.Context, tenantID uuid.UUID, period string) *AIProcessingStats {
	// Implementation would analyze AI processing jobs
	return &AIProcessingStats{
		TotalJobsProcessed: 0,
		SuccessfulJobs:     0,
		JobsByType:         make(map[string]int64),
		AccuracyMetrics:    make(map[string]float64),
	}
}

func (s *AnalyticsService) getRecentActivity(ctx context.Context, tenantID uuid.UUID, limit int) []ActivitySummary {
	// Implementation would get recent audit log entries
	return []ActivitySummary{}
}

func (s *AnalyticsService) getSystemAlerts(ctx context.Context, tenantID uuid.UUID) []SystemAlert {
	// Implementation would check for system alerts and warnings
	return []SystemAlert{}
}

func (s *AnalyticsService) enhanceStorageAnalytics(analytics *StorageAnalytics) {
	// Add predictions and insights to storage analytics
	if analytics.UsagePercent > 80 {
		// Predict when storage will be full based on growth trend
		// This is a simplified prediction
	}
}

func (s *AnalyticsService) getDocumentsThisMonth(stats *repositories.DashboardStats) int64 {
	// Calculate documents created this month from stats
	return 0 // Placeholder
}

func (s *AnalyticsService) calculateGrowthRate(stats *repositories.DashboardStats) float64 {
	// Calculate growth rate from historical data
	return 0.0 // Placeholder
}

func (s *AnalyticsService) exportCSV(dashboard *DashboardData) ([]byte, error) {
	// Implementation would convert dashboard data to CSV
	return nil, nil
}

func (s *AnalyticsService) exportExcel(dashboard *DashboardData) ([]byte, error) {
	// Implementation would convert dashboard data to Excel
	return nil, nil
}

func (s *AnalyticsService) exportPDF(dashboard *DashboardData) ([]byte, error) {
	// Implementation would convert dashboard data to PDF report
	return nil, nil
}

// AnalyticsFilters for filtering analytics data
type AnalyticsFilters struct {
	Period       string     `json:"period"`
	DateFrom     *time.Time `json:"date_from,omitempty"`
	DateTo       *time.Time `json:"date_to,omitempty"`
	DocumentType string     `json:"document_type,omitempty"`
	UserID       *uuid.UUID `json:"user_id,omitempty"`
	Department   string     `json:"department,omitempty"`
}
