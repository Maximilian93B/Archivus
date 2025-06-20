package models

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/pgvector/pgvector-go"
)

// Enhanced Custom Types
type DocStatus string
type UserRole string
type SubscriptionTier string
type ProcessingStatus string
type AuditAction string
type DocumentType string
type WorkflowStatus string
type NotificationChannel string
type ComplianceStatus string

const (
	// Document Status
	DocStatusPending     DocStatus = "pending"
	DocStatusProcessing  DocStatus = "processing"
	DocStatusCompleted   DocStatus = "completed"
	DocStatusError       DocStatus = "error"
	DocStatusArchived    DocStatus = "archived"
	DocStatusExpired     DocStatus = "expired"
	DocStatusUnderReview DocStatus = "under_review"

	// User Roles
	UserRoleAdmin      UserRole = "admin"
	UserRoleManager    UserRole = "manager"
	UserRoleUser       UserRole = "user"
	UserRoleViewer     UserRole = "viewer"
	UserRoleAccountant UserRole = "accountant"
	UserRoleCompliance UserRole = "compliance"

	// Subscription Tiers
	SubscriptionStarter      SubscriptionTier = "starter"
	SubscriptionProfessional SubscriptionTier = "professional"
	SubscriptionEnterprise   SubscriptionTier = "enterprise"

	// Processing Status
	ProcessingQueued     ProcessingStatus = "queued"
	ProcessingInProgress ProcessingStatus = "processing"
	ProcessingCompleted  ProcessingStatus = "completed"
	ProcessingFailed     ProcessingStatus = "failed"

	// Audit Actions
	AuditCreate   AuditAction = "create"
	AuditRead     AuditAction = "read"
	AuditUpdate   AuditAction = "update"
	AuditDelete   AuditAction = "delete"
	AuditDownload AuditAction = "download"
	AuditShare    AuditAction = "share"
	AuditApprove  AuditAction = "approve"
	AuditReject   AuditAction = "reject"

	// Document Types for SMB
	DocTypeInvoice       DocumentType = "invoice"
	DocTypeReceipt       DocumentType = "receipt"
	DocTypeContract      DocumentType = "contract"
	DocTypeSpreadsheet   DocumentType = "spreadsheet"
	DocTypePresentationn DocumentType = "presentation"
	DocTypeReport        DocumentType = "report"
	DocTypeTaxDocument   DocumentType = "tax_document"
	DocTypePayroll       DocumentType = "payroll"
	DocTypeBankStatement DocumentType = "bank_statement"
	DocTypeInsurance     DocumentType = "insurance"
	DocTypeLegal         DocumentType = "legal"
	DocTypeHR            DocumentType = "hr"
	DocTypeMarketing     DocumentType = "marketing"
	DocTypeGeneral       DocumentType = "general"

	// Workflow Status
	WorkflowPending   WorkflowStatus = "pending"
	WorkflowApproved  WorkflowStatus = "approved"
	WorkflowRejected  WorkflowStatus = "rejected"
	WorkflowEscalated WorkflowStatus = "escalated"

	// Notification Channels
	NotifyEmail   NotificationChannel = "email"
	NotifySlack   NotificationChannel = "slack"
	NotifyWebhook NotificationChannel = "webhook"
	NotifyInApp   NotificationChannel = "in_app"

	// Compliance Status
	ComplianceCompliant    ComplianceStatus = "compliant"
	ComplianceNonCompliant ComplianceStatus = "non_compliant"
	CompliancePending      ComplianceStatus = "pending"
	ComplianceExempt       ComplianceStatus = "exempt"
)

// JSONB type for PostgreSQL jsonb columns
type JSONB map[string]interface{}

func (j JSONB) Value() (driver.Value, error) {
	return json.Marshal(j)
}

func (j *JSONB) Scan(value interface{}) error {
	bytes, ok := value.([]byte)
	if !ok {
		return errors.New("type assertion to []byte failed")
	}
	return json.Unmarshal(bytes, j)
}

// Enhanced Core Models
type Tenant struct {
	ID               uuid.UUID        `json:"id" gorm:"type:uuid;primary_key;default:uuid_generate_v4()"`
	Name             string           `json:"name" gorm:"type:varchar(255);not null"`
	Subdomain        string           `json:"subdomain" gorm:"type:varchar(100);unique;not null"`
	SubscriptionTier SubscriptionTier `json:"subscription_tier" gorm:"type:varchar(20);not null;default:'starter'"`
	StorageQuota     int64            `json:"storage_quota" gorm:"not null;default:5368709120"` // 5GB default
	StorageUsed      int64            `json:"storage_used" gorm:"not null;default:0"`
	APIQuota         int              `json:"api_quota" gorm:"not null;default:1000"`
	APIUsed          int              `json:"api_used" gorm:"not null;default:0"`
	Settings         JSONB            `json:"settings" gorm:"type:jsonb;default:'{}'"`
	IsActive         bool             `json:"is_active" gorm:"not null;default:true"`
	TrialEndsAt      *time.Time       `json:"trial_ends_at"`

	// Business Information
	BusinessType string `json:"business_type" gorm:"type:varchar(100)"`
	Industry     string `json:"industry" gorm:"type:varchar(100)"`
	CompanySize  string `json:"company_size" gorm:"type:varchar(50)"`
	TaxID        string `json:"tax_id" gorm:"type:varchar(50)"`
	Address      JSONB  `json:"address" gorm:"type:jsonb"`

	// Compliance & Retention
	RetentionPolicy JSONB `json:"retention_policy" gorm:"type:jsonb"`
	ComplianceRules JSONB `json:"compliance_rules" gorm:"type:jsonb"`

	CreatedAt time.Time `json:"created_at" gorm:"not null;default:now()"`
	UpdatedAt time.Time `json:"updated_at" gorm:"not null;default:now()"`

	// Relationships
	Users      []User             `json:"users,omitempty" gorm:"foreignKey:TenantID"`
	Documents  []Document         `json:"documents,omitempty" gorm:"foreignKey:TenantID"`
	Folders    []Folder           `json:"folders,omitempty" gorm:"foreignKey:TenantID"`
	Tags       []Tag              `json:"tags,omitempty" gorm:"foreignKey:TenantID"`
	Categories []Category         `json:"categories,omitempty" gorm:"foreignKey:TenantID"`
	AIJobs     []AIProcessingJob  `json:"ai_jobs,omitempty" gorm:"foreignKey:TenantID"`
	Workflows  []Workflow         `json:"workflows,omitempty" gorm:"foreignKey:TenantID"`
	Templates  []DocumentTemplate `json:"templates,omitempty" gorm:"foreignKey:TenantID"`
}

type User struct {
	ID                uuid.UUID  `json:"id" gorm:"type:uuid;primary_key;default:uuid_generate_v4()"`
	TenantID          uuid.UUID  `json:"tenant_id" gorm:"type:uuid;not null;index"`
	Email             string     `json:"email" gorm:"type:varchar(320);not null;index"`
	PasswordHash      string     `json:"-" gorm:"type:varchar(255);not null"`
	FirstName         string     `json:"first_name" gorm:"type:varchar(100);not null"`
	LastName          string     `json:"last_name" gorm:"type:varchar(100);not null"`
	Role              UserRole   `json:"role" gorm:"type:varchar(20);not null;default:'user'"`
	Department        string     `json:"department" gorm:"type:varchar(100)"`
	JobTitle          string     `json:"job_title" gorm:"type:varchar(100)"`
	IsActive          bool       `json:"is_active" gorm:"not null;default:true"`
	EmailVerified     bool       `json:"email_verified" gorm:"not null;default:false"`
	LastLoginAt       *time.Time `json:"last_login_at"`
	PasswordChangedAt time.Time  `json:"password_changed_at" gorm:"not null;default:now()"`
	MFAEnabled        bool       `json:"mfa_enabled" gorm:"not null;default:false"`
	MFASecret         string     `json:"-" gorm:"type:varchar(32)"`

	// User Preferences
	Preferences          JSONB `json:"preferences" gorm:"type:jsonb;default:'{}'"`
	NotificationSettings JSONB `json:"notification_settings" gorm:"type:jsonb;default:'{}'"`

	CreatedAt time.Time `json:"created_at" gorm:"not null;default:now()"`
	UpdatedAt time.Time `json:"updated_at" gorm:"not null;default:now()"`

	// Relationships
	Tenant           Tenant         `json:"tenant,omitempty" gorm:"foreignKey:TenantID"`
	CreatedDocuments []Document     `json:"created_documents,omitempty" gorm:"foreignKey:CreatedBy"`
	UpdatedDocuments []Document     `json:"updated_documents,omitempty" gorm:"foreignKey:UpdatedBy"`
	CreatedFolders   []Folder       `json:"created_folders,omitempty" gorm:"foreignKey:CreatedBy"`
	WorkflowTasks    []WorkflowTask `json:"workflow_tasks,omitempty" gorm:"foreignKey:AssignedTo"`
}

// Enhanced Document Model - The Beast!
type Document struct {
	ID       uuid.UUID  `json:"id" gorm:"type:uuid;primary_key;default:uuid_generate_v4()"`
	TenantID uuid.UUID  `json:"tenant_id" gorm:"type:uuid;not null;index"`
	FolderID *uuid.UUID `json:"folder_id" gorm:"type:uuid;index"`

	// Basic File Info
	FileName      string `json:"file_name" gorm:"type:varchar(255);not null"`
	OriginalName  string `json:"original_name" gorm:"type:varchar(255);not null"`
	ContentType   string `json:"content_type" gorm:"type:varchar(100);not null"`
	FileSize      int64  `json:"file_size" gorm:"not null"`
	StoragePath   string `json:"storage_path" gorm:"type:varchar(500);not null"`
	ThumbnailPath string `json:"thumbnail_path" gorm:"type:varchar(500)"`
	PreviewPath   string `json:"preview_path" gorm:"type:varchar(500)"`

	// Content Analysis
	ExtractedText string          `json:"extracted_text" gorm:"type:text"`
	ContentHash   string          `json:"content_hash" gorm:"type:varchar(64);not null;index"`
	OCRText       string          `json:"ocr_text" gorm:"type:text"`
	Summary       string          `json:"summary" gorm:"type:text"`
	AIConfidence  float64         `json:"ai_confidence" gorm:"type:decimal(3,2)"`
	Embedding     pgvector.Vector `json:"-" gorm:"type:vector(1536)"`

	// Document Metadata
	Title        string       `json:"title" gorm:"type:varchar(255)"`
	Description  string       `json:"description" gorm:"type:text"`
	DocumentType DocumentType `json:"document_type" gorm:"type:varchar(50);index"`
	Status       DocStatus    `json:"status" gorm:"type:varchar(20);not null;default:'pending'"`
	Version      int          `json:"version" gorm:"not null;default:1"`
	Language     string       `json:"language" gorm:"type:varchar(10);default:'en'"`

	// Business Document Fields
	DocumentNumber  string `json:"document_number" gorm:"type:varchar(100);index"`
	ReferenceNumber string `json:"reference_number" gorm:"type:varchar(100);index"`
	ExternalID      string `json:"external_id" gorm:"type:varchar(100);index"`

	// Financial Data (for invoices, receipts, etc.)
	Amount       *float64 `json:"amount" gorm:"type:decimal(15,2)"`
	Currency     string   `json:"currency" gorm:"type:varchar(3);default:'USD'"`
	TaxAmount    *float64 `json:"tax_amount" gorm:"type:decimal(15,2)"`
	VendorName   string   `json:"vendor_name" gorm:"type:varchar(255);index"`
	CustomerName string   `json:"customer_name" gorm:"type:varchar(255);index"`

	// Dates
	DocumentDate *time.Time `json:"document_date" gorm:"index"`
	DueDate      *time.Time `json:"due_date" gorm:"index"`
	ExpiryDate   *time.Time `json:"expiry_date" gorm:"index"`

	// Compliance & Legal
	ComplianceStatus ComplianceStatus `json:"compliance_status" gorm:"type:varchar(20);default:'pending'"`
	RetentionDate    *time.Time       `json:"retention_date" gorm:"index"`
	LegalHold        bool             `json:"legal_hold" gorm:"not null;default:false"`

	// Structured Data Extraction
	ExtractedData JSONB `json:"extracted_data" gorm:"type:jsonb"` // AI-extracted structured data
	CustomFields  JSONB `json:"custom_fields" gorm:"type:jsonb"`  // Tenant-specific fields

	// System Fields
	CreatedBy uuid.UUID  `json:"created_by" gorm:"type:uuid;not null;index"`
	UpdatedBy *uuid.UUID `json:"updated_by" gorm:"type:uuid;index"`
	CreatedAt time.Time  `json:"created_at" gorm:"not null;default:now()"`
	UpdatedAt time.Time  `json:"updated_at" gorm:"not null;default:now()"`

	// Legacy Fields (keeping for compatibility)
	Author             string     `json:"author" gorm:"type:varchar(255)"`
	Subject            string     `json:"subject" gorm:"type:varchar(255)"`
	Keywords           string     `json:"keywords" gorm:"type:text"`
	DocumentCreatedAt  *time.Time `json:"document_created_at"`
	DocumentModifiedAt *time.Time `json:"document_modified_at"`

	// Relationships
	Tenant        Tenant            `json:"tenant,omitempty" gorm:"foreignKey:TenantID"`
	Folder        *Folder           `json:"folder,omitempty" gorm:"foreignKey:FolderID"`
	Creator       User              `json:"creator,omitempty" gorm:"foreignKey:CreatedBy"`
	Updater       *User             `json:"updater,omitempty" gorm:"foreignKey:UpdatedBy"`
	Tags          []Tag             `json:"tags,omitempty" gorm:"many2many:document_tags"`
	Categories    []Category        `json:"categories,omitempty" gorm:"many2many:document_categories"`
	AIJobs        []AIProcessingJob `json:"ai_jobs,omitempty" gorm:"foreignKey:DocumentID"`
	Versions      []DocumentVersion `json:"versions,omitempty" gorm:"foreignKey:DocumentID"`
	WorkflowTasks []WorkflowTask    `json:"workflow_tasks,omitempty" gorm:"foreignKey:DocumentID"`
	Comments      []DocumentComment `json:"comments,omitempty" gorm:"foreignKey:DocumentID"`
}

// New Models for Enhanced Functionality

// Document Versioning
type DocumentVersion struct {
	ID            uuid.UUID `json:"id" gorm:"type:uuid;primary_key;default:uuid_generate_v4()"`
	DocumentID    uuid.UUID `json:"document_id" gorm:"type:uuid;not null;index"`
	VersionNumber int       `json:"version_number" gorm:"not null"`
	StoragePath   string    `json:"storage_path" gorm:"type:varchar(500);not null"`
	FileSize      int64     `json:"file_size" gorm:"not null"`
	ContentHash   string    `json:"content_hash" gorm:"type:varchar(64);not null"`
	Changes       string    `json:"changes" gorm:"type:text"`
	CreatedBy     uuid.UUID `json:"created_by" gorm:"type:uuid;not null"`
	CreatedAt     time.Time `json:"created_at" gorm:"not null;default:now()"`

	// Relationships
	Document Document `json:"document,omitempty" gorm:"foreignKey:DocumentID"`
	Creator  User     `json:"creator,omitempty" gorm:"foreignKey:CreatedBy"`
}

// Document Templates for SMB
type DocumentTemplate struct {
	ID          uuid.UUID    `json:"id" gorm:"type:uuid;primary_key;default:uuid_generate_v4()"`
	TenantID    uuid.UUID    `json:"tenant_id" gorm:"type:uuid;not null;index"`
	Name        string       `json:"name" gorm:"type:varchar(255);not null"`
	Description string       `json:"description" gorm:"type:text"`
	DocType     DocumentType `json:"document_type" gorm:"type:varchar(50);not null"`
	Template    JSONB        `json:"template" gorm:"type:jsonb;not null"`
	IsActive    bool         `json:"is_active" gorm:"not null;default:true"`
	CreatedBy   uuid.UUID    `json:"created_by" gorm:"type:uuid;not null"`
	CreatedAt   time.Time    `json:"created_at" gorm:"not null;default:now()"`
	UpdatedAt   time.Time    `json:"updated_at" gorm:"not null;default:now()"`

	// Relationships
	Tenant  Tenant `json:"tenant,omitempty" gorm:"foreignKey:TenantID"`
	Creator User   `json:"creator,omitempty" gorm:"foreignKey:CreatedBy"`
}

// Workflow System for Document Approval
type Workflow struct {
	ID          uuid.UUID    `json:"id" gorm:"type:uuid;primary_key;default:uuid_generate_v4()"`
	TenantID    uuid.UUID    `json:"tenant_id" gorm:"type:uuid;not null;index"`
	Name        string       `json:"name" gorm:"type:varchar(255);not null"`
	Description string       `json:"description" gorm:"type:text"`
	DocType     DocumentType `json:"document_type" gorm:"type:varchar(50);not null"`
	Rules       JSONB        `json:"rules" gorm:"type:jsonb;not null"`
	IsActive    bool         `json:"is_active" gorm:"not null;default:true"`
	CreatedBy   uuid.UUID    `json:"created_by" gorm:"type:uuid;not null"`
	CreatedAt   time.Time    `json:"created_at" gorm:"not null;default:now()"`
	UpdatedAt   time.Time    `json:"updated_at" gorm:"not null;default:now()"`

	// Relationships
	Tenant  Tenant         `json:"tenant,omitempty" gorm:"foreignKey:TenantID"`
	Creator User           `json:"creator,omitempty" gorm:"foreignKey:CreatedBy"`
	Tasks   []WorkflowTask `json:"tasks,omitempty" gorm:"foreignKey:WorkflowID"`
}

type WorkflowTask struct {
	ID          uuid.UUID      `json:"id" gorm:"type:uuid;primary_key;default:uuid_generate_v4()"`
	WorkflowID  uuid.UUID      `json:"workflow_id" gorm:"type:uuid;not null;index"`
	DocumentID  uuid.UUID      `json:"document_id" gorm:"type:uuid;not null;index"`
	AssignedTo  uuid.UUID      `json:"assigned_to" gorm:"type:uuid;not null;index"`
	TaskType    string         `json:"task_type" gorm:"type:varchar(50);not null"`
	Status      WorkflowStatus `json:"status" gorm:"type:varchar(20);not null;default:'pending'"`
	Priority    int            `json:"priority" gorm:"not null;default:5"`
	DueDate     *time.Time     `json:"due_date"`
	Comments    string         `json:"comments" gorm:"type:text"`
	CompletedAt *time.Time     `json:"completed_at"`
	CreatedAt   time.Time      `json:"created_at" gorm:"not null;default:now()"`
	UpdatedAt   time.Time      `json:"updated_at" gorm:"not null;default:now()"`

	// Relationships
	Workflow Workflow `json:"workflow,omitempty" gorm:"foreignKey:WorkflowID"`
	Document Document `json:"document,omitempty" gorm:"foreignKey:DocumentID"`
	Assignee User     `json:"assignee,omitempty" gorm:"foreignKey:AssignedTo"`
}

// Document Comments/Collaboration
type DocumentComment struct {
	ID         uuid.UUID `json:"id" gorm:"type:uuid;primary_key;default:uuid_generate_v4()"`
	DocumentID uuid.UUID `json:"document_id" gorm:"type:uuid;not null;index"`
	UserID     uuid.UUID `json:"user_id" gorm:"type:uuid;not null;index"`
	Content    string    `json:"content" gorm:"type:text;not null"`
	IsResolved bool      `json:"is_resolved" gorm:"not null;default:false"`
	CreatedAt  time.Time `json:"created_at" gorm:"not null;default:now()"`
	UpdatedAt  time.Time `json:"updated_at" gorm:"not null;default:now()"`

	// Relationships
	Document Document `json:"document,omitempty" gorm:"foreignKey:DocumentID"`
	User     User     `json:"user,omitempty" gorm:"foreignKey:UserID"`
}

// Business Intelligence & Analytics
type DocumentAnalytics struct {
	ID             uuid.UUID  `json:"id" gorm:"type:uuid;primary_key;default:uuid_generate_v4()"`
	TenantID       uuid.UUID  `json:"tenant_id" gorm:"type:uuid;not null;index"`
	DocumentID     uuid.UUID  `json:"document_id" gorm:"type:uuid;not null;index"`
	ViewCount      int        `json:"view_count" gorm:"not null;default:0"`
	DownloadCount  int        `json:"download_count" gorm:"not null;default:0"`
	ShareCount     int        `json:"share_count" gorm:"not null;default:0"`
	LastAccessedAt *time.Time `json:"last_accessed_at"`
	ProcessingTime int        `json:"processing_time_ms"`
	StorageCost    *float64   `json:"storage_cost" gorm:"type:decimal(10,4)"`
	CreatedAt      time.Time  `json:"created_at" gorm:"not null;default:now()"`
	UpdatedAt      time.Time  `json:"updated_at" gorm:"not null;default:now()"`

	// Relationships
	Tenant   Tenant   `json:"tenant,omitempty" gorm:"foreignKey:TenantID"`
	Document Document `json:"document,omitempty" gorm:"foreignKey:DocumentID"`
}

// Notification System
type Notification struct {
	ID        uuid.UUID           `json:"id" gorm:"type:uuid;primary_key;default:uuid_generate_v4()"`
	TenantID  uuid.UUID           `json:"tenant_id" gorm:"type:uuid;not null;index"`
	UserID    uuid.UUID           `json:"user_id" gorm:"type:uuid;not null;index"`
	Type      string              `json:"type" gorm:"type:varchar(50);not null"`
	Title     string              `json:"title" gorm:"type:varchar(255);not null"`
	Message   string              `json:"message" gorm:"type:text;not null"`
	Channel   NotificationChannel `json:"channel" gorm:"type:varchar(20);not null"`
	IsRead    bool                `json:"is_read" gorm:"not null;default:false"`
	Data      JSONB               `json:"data" gorm:"type:jsonb"`
	CreatedAt time.Time           `json:"created_at" gorm:"not null;default:now()"`

	// Relationships
	Tenant Tenant `json:"tenant,omitempty" gorm:"foreignKey:TenantID"`
	User   User   `json:"user,omitempty" gorm:"foreignKey:UserID"`
}

// Keep existing models with minor enhancements
type Folder struct {
	ID          uuid.UUID  `json:"id" gorm:"type:uuid;primary_key;default:uuid_generate_v4()"`
	TenantID    uuid.UUID  `json:"tenant_id" gorm:"type:uuid;not null;uniqueIndex:idx_tenant_folder_path"`
	ParentID    *uuid.UUID `json:"parent_id" gorm:"type:uuid;index"`
	Name        string     `json:"name" gorm:"type:varchar(255);not null"`
	Description string     `json:"description" gorm:"type:text"`
	Path        string     `json:"path" gorm:"type:varchar(2048);not null;uniqueIndex:idx_tenant_folder_path"`
	Level       int        `json:"level" gorm:"not null;default:0"`
	IsSystem    bool       `json:"is_system" gorm:"not null;default:false"`
	Color       string     `json:"color" gorm:"type:varchar(7);default:'#6B7280'"`
	Icon        string     `json:"icon" gorm:"type:varchar(50)"`
	CreatedBy   uuid.UUID  `json:"created_by" gorm:"type:uuid;not null;index"`
	CreatedAt   time.Time  `json:"created_at" gorm:"not null;default:now()"`
	UpdatedAt   time.Time  `json:"updated_at" gorm:"not null;default:now()"`

	// Relationships
	Tenant    Tenant     `json:"tenant,omitempty" gorm:"foreignKey:TenantID"`
	Parent    *Folder    `json:"parent,omitempty" gorm:"foreignKey:ParentID"`
	Children  []Folder   `json:"children,omitempty" gorm:"foreignKey:ParentID"`
	Creator   User       `json:"creator,omitempty" gorm:"foreignKey:CreatedBy"`
	Documents []Document `json:"documents,omitempty" gorm:"foreignKey:FolderID"`
}

type Category struct {
	ID          uuid.UUID `json:"id" gorm:"type:uuid;primary_key;default:uuid_generate_v4()"`
	TenantID    uuid.UUID `json:"tenant_id" gorm:"type:uuid;not null;index"`
	Name        string    `json:"name" gorm:"type:varchar(100);not null;uniqueIndex:idx_tenant_category_name"`
	Description string    `json:"description" gorm:"type:text"`
	Color       string    `json:"color" gorm:"type:varchar(7);default:'#6B7280'"`
	Icon        string    `json:"icon" gorm:"type:varchar(50)"`
	IsSystem    bool      `json:"is_system" gorm:"not null;default:false"`
	SortOrder   int       `json:"sort_order" gorm:"not null;default:0"`
	CreatedAt   time.Time `json:"created_at" gorm:"not null;default:now()"`

	// Relationships
	Tenant    Tenant     `json:"tenant,omitempty" gorm:"foreignKey:TenantID"`
	Documents []Document `json:"documents,omitempty" gorm:"many2many:document_categories"`
}

type Tag struct {
	ID            uuid.UUID `json:"id" gorm:"type:uuid;primary_key;default:uuid_generate_v4()"`
	TenantID      uuid.UUID `json:"tenant_id" gorm:"type:uuid;not null;index"`
	Name          string    `json:"name" gorm:"type:varchar(50);not null;uniqueIndex:idx_tenant_tag_name"`
	Color         string    `json:"color" gorm:"type:varchar(7);default:'#6B7280'"`
	IsAIGenerated bool      `json:"is_ai_generated" gorm:"not null;default:false"`
	UsageCount    int       `json:"usage_count" gorm:"not null;default:0"`
	CreatedAt     time.Time `json:"created_at" gorm:"not null;default:now()"`

	// Relationships
	Tenant    Tenant     `json:"tenant,omitempty" gorm:"foreignKey:TenantID"`
	Documents []Document `json:"documents,omitempty" gorm:"many2many:document_tags"`
}

type AIProcessingJob struct {
	ID               uuid.UUID        `json:"id" gorm:"type:uuid;primary_key;default:uuid_generate_v4()"`
	TenantID         uuid.UUID        `json:"tenant_id" gorm:"type:uuid;not null;index"`
	DocumentID       uuid.UUID        `json:"document_id" gorm:"type:uuid;not null;index"`
	JobType          string           `json:"job_type" gorm:"type:varchar(50);not null"`
	Status           ProcessingStatus `json:"status" gorm:"type:varchar(20);not null;default:'queued';index"`
	Priority         int              `json:"priority" gorm:"not null;default:5"`
	Attempts         int              `json:"attempts" gorm:"not null;default:0"`
	MaxAttempts      int              `json:"max_attempts" gorm:"not null;default:3"`
	ErrorMessage     string           `json:"error_message" gorm:"type:text"`
	Result           JSONB            `json:"result" gorm:"type:jsonb"`
	ProcessingTimeMs int              `json:"processing_time_ms"`
	CreatedAt        time.Time        `json:"created_at" gorm:"not null;default:now()"`
	StartedAt        *time.Time       `json:"started_at"`
	CompletedAt      *time.Time       `json:"completed_at"`

	// Relationships
	Tenant   Tenant   `json:"tenant,omitempty" gorm:"foreignKey:TenantID"`
	Document Document `json:"document,omitempty" gorm:"foreignKey:DocumentID"`
}

// Additional models for audit and sharing (keeping existing)
type AuditLog struct {
	ID           uuid.UUID   `json:"id" gorm:"type:uuid;primary_key;default:uuid_generate_v4()"`
	TenantID     uuid.UUID   `json:"tenant_id" gorm:"type:uuid;not null;index"`
	UserID       uuid.UUID   `json:"user_id" gorm:"type:uuid;not null;index"`
	ResourceID   uuid.UUID   `json:"resource_id" gorm:"type:uuid;not null;index"`
	Action       AuditAction `json:"action" gorm:"type:varchar(20);not null"`
	ResourceType string      `json:"resource_type" gorm:"type:varchar(50);not null"`
	IPAddress    string      `json:"ip_address" gorm:"type:varchar(45)"`
	UserAgent    string      `json:"user_agent" gorm:"type:text"`
	Details      JSONB       `json:"details" gorm:"type:jsonb"`
	CreatedAt    time.Time   `json:"created_at" gorm:"not null;default:now()"`

	// Relationships
	Tenant Tenant `json:"tenant,omitempty" gorm:"foreignKey:TenantID"`
	User   User   `json:"user,omitempty" gorm:"foreignKey:UserID"`
}

type Share struct {
	ID            uuid.UUID  `json:"id" gorm:"type:uuid;primary_key;default:uuid_generate_v4()"`
	TenantID      uuid.UUID  `json:"tenant_id" gorm:"type:uuid;not null;index"`
	DocumentID    uuid.UUID  `json:"document_id" gorm:"type:uuid;not null;index"`
	CreatedBy     uuid.UUID  `json:"created_by" gorm:"type:uuid;not null;index"`
	Token         string     `json:"token" gorm:"type:varchar(255);unique;not null"`
	Password      string     `json:"password,omitempty" gorm:"type:varchar(255)"`
	ExpiresAt     *time.Time `json:"expires_at"`
	MaxDownloads  int        `json:"max_downloads" gorm:"default:0"` // 0 = unlimited
	DownloadCount int        `json:"download_count" gorm:"default:0"`
	IsActive      bool       `json:"is_active" gorm:"not null;default:true"`
	CreatedAt     time.Time  `json:"created_at" gorm:"not null;default:now()"`
	UpdatedAt     time.Time  `json:"updated_at" gorm:"not null;default:now()"`

	// Relationships
	Tenant   Tenant   `json:"tenant,omitempty" gorm:"foreignKey:TenantID"`
	Document Document `json:"document,omitempty" gorm:"foreignKey:DocumentID"`
	Creator  User     `json:"creator,omitempty" gorm:"foreignKey:CreatedBy"`
}

// GetAllModels returns all models for migration
func GetAllModels() []interface{} {
	return []interface{}{
		&Tenant{},
		&User{},
		&Folder{},
		&Category{},
		&Tag{},
		&Document{},
		&DocumentVersion{},
		&DocumentTemplate{},
		&DocumentComment{},
		&DocumentAnalytics{},
		&Workflow{},
		&WorkflowTask{},
		&Notification{},
		&AIProcessingJob{},
		&AuditLog{},
		&Share{},
	}
}
