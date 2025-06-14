package reference

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/pgvector/pgvector-go"
)

// Custom Types
type DocStatus string
type UserRole string
type SubscriptionTier string
type ProcessingStatus string
type AuditAction string

const (
	// Document Status
	DocStatusPending    DocStatus = "pending"
	DocStatusProcessing DocStatus = "processing"
	DocStatusCompleted  DocStatus = "completed"
	DocStatusError      DocStatus = "error"
	DocStatusArchived   DocStatus = "archived"

	// User Roles
	UserRoleAdmin   UserRole = "admin"
	UserRoleManager UserRole = "manager"
	UserRoleUser    UserRole = "user"
	UserRoleViewer  UserRole = "viewer"

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
)

// Core Models
type Tenant struct {
	ID               uuid.UUID        `json:"id" gorm:"type:uuid;primary_key;default:uuid_generate_v4()"`
	Name             string           `json:"name" gorm:"type:varchar(255);not null"`
	Subdomain        string           `json:"subdomain" gorm:"type:varchar(100);unique;not null"`
	SubscriptionTier SubscriptionTier `json:"subscription_tier" gorm:"type:subscription_tier;not null;default:'starter'"`
	StorageQuota     int64            `json:"storage_quota" gorm:"not null;default:5368709120"`
	StorageUsed      int64            `json:"storage_used" gorm:"not null;default:0"`
	APIQuota         int              `json:"api_quota" gorm:"not null;default:1000"`
	APIUsed          int              `json:"api_used" gorm:"not null;default:0"`
	Settings         JSONB            `json:"settings" gorm:"type:jsonb;default:'{}'"`
	IsActive         bool             `json:"is_active" gorm:"not null;default:true"`
	TrialEndsAt      *time.Time       `json:"trial_ends_at"`
	CreatedAt        time.Time        `json:"created_at" gorm:"not null;default:now()"`
	UpdatedAt        time.Time        `json:"updated_at" gorm:"not null;default:now()"`
}

type User struct {
	ID                uuid.UUID  `json:"id" gorm:"type:uuid;primary_key;default:uuid_generate_v4()"`
	TenantID          uuid.UUID  `json:"tenant_id" gorm:"type:uuid;not null"`
	Email             string     `json:"email" gorm:"type:varchar(320);not null"`
	PasswordHash      string     `json:"-" gorm:"type:varchar(255);not null"`
	FirstName         string     `json:"first_name" gorm:"type:varchar(100);not null"`
	LastName          string     `json:"last_name" gorm:"type:varchar(100);not null"`
	Role              UserRole   `json:"role" gorm:"type:user_role;not null;default:'user'"`
	IsActive          bool       `json:"is_active" gorm:"not null;default:true"`
	EmailVerified     bool       `json:"email_verified" gorm:"not null;default:false"`
	LastLoginAt       *time.Time `json:"last_login_at"`
	PasswordChangedAt time.Time  `json:"password_changed_at" gorm:"not null;default:now()"`
	MFAEnabled        bool       `json:"mfa_enabled" gorm:"not null;default:false"`
	MFASecret         string     `json:"-" gorm:"type:varchar(32)"`
	CreatedAt         time.Time  `json:"created_at" gorm:"not null;default:now()"`
	UpdatedAt         time.Time  `json:"updated_at" gorm:"not null;default:now()"`
}

type Document struct {
	ID                 uuid.UUID       `json:"id" gorm:"type:uuid;primary_key;default:uuid_generate_v4()"`
	TenantID           uuid.UUID       `json:"tenant_id" gorm:"type:uuid;not null;index"`
	FolderID           *uuid.UUID      `json:"folder_id" gorm:"type:uuid"`
	FileName           string          `json:"file_name" gorm:"type:varchar(255);not null"`
	OriginalName       string          `json:"original_name" gorm:"type:varchar(255);not null"`
	ContentType        string          `json:"content_type" gorm:"type:varchar(100);not null"`
	FileSize           int64           `json:"file_size" gorm:"not null"`
	StoragePath        string          `json:"storage_path" gorm:"type:varchar(500);not null"`
	ExtractedText      string          `json:"extracted_text" gorm:"type:text"`
	ContentHash        string          `json:"content_hash" gorm:"type:varchar(64);not null;index"`
	OCRText            string          `json:"ocr_text" gorm:"type:text"`
	Summary            string          `json:"summary" gorm:"type:text"`
	AIConfidence       float64         `json:"ai_confidence" gorm:"type:decimal(3,2)"`
	Embedding          pgvector.Vector `json:"embedding" gorm:"type:vector(1536)"`
	Title              string          `json:"title" gorm:"type:varchar(255)"`
	Description        string          `json:"description" gorm:"type:text"`
	Status             DocStatus       `json:"status" gorm:"type:doc_status;not null;default:'pending'"`
	Version            int             `json:"version" gorm:"not null;default:1"`
	Language           string          `json:"language" gorm:"type:varchar(10);default:'en'"`
	CreatedBy          uuid.UUID       `json:"created_by" gorm:"type:uuid;not null"`
	UpdatedBy          *uuid.UUID      `json:"updated_by" gorm:"type:uuid"`
	CreatedAt          time.Time       `json:"created_at" gorm:"not null;default:now()"`
	UpdatedAt          time.Time       `json:"updated_at" gorm:"not null;default:now()"`
	Author             string          `json:"author" gorm:"type:varchar(255)"`
	Subject            string          `json:"subject" gorm:"type:varchar(255)"`
	Keywords           string          `json:"keywords" gorm:"type:text"`
	DocumentCreatedAt  *time.Time      `json:"document_created_at"`
	DocumentModifiedAt *time.Time      `json:"document_modified_at"`
	Tags               []Tag           `json:"tags" gorm:"many2many:document_tags"`
	Categories         []Category      `json:"categories" gorm:"many2many:document_categories"`
}

type Folder struct {
	ID          uuid.UUID  `json:"id" gorm:"type:uuid;primary_key;default:uuid_generate_v4()"`
	TenantID    uuid.UUID  `json:"tenant_id" gorm:"type:uuid;not null;index"`
	ParentID    *uuid.UUID `json:"parent_id" gorm:"type:uuid"`
	Name        string     `json:"name" gorm:"type:varchar(255);not null"`
	Description string     `json:"description" gorm:"type:text"`
	Path        string     `json:"path" gorm:"type:varchar(2048);not null"`
	Level       int        `json:"level" gorm:"not null;default:0"`
	IsSystem    bool       `json:"is_system" gorm:"not null;default:false"`
	CreatedBy   uuid.UUID  `json:"created_by" gorm:"type:uuid;not null"`
	CreatedAt   time.Time  `json:"created_at" gorm:"not null;default:now()"`
	UpdatedAt   time.Time  `json:"updated_at" gorm:"not null;default:now()"`
}

type Category struct {
	ID          uuid.UUID `json:"id" gorm:"type:uuid;primary_key;default:uuid_generate_v4()"`
	TenantID    uuid.UUID `json:"tenant_id" gorm:"type:uuid;not null;index"`
	Name        string    `json:"name" gorm:"type:varchar(100);not null"`
	Description string    `json:"description" gorm:"type:text"`
	Color       string    `json:"color" gorm:"type:varchar(7);default:'#6B7280'"`
	Icon        string    `json:"icon" gorm:"type:varchar(50)"`
	IsSystem    bool      `json:"is_system" gorm:"not null;default:false"`
	CreatedAt   time.Time `json:"created_at" gorm:"not null;default:now()"`
}

type Tag struct {
	ID            uuid.UUID `json:"id" gorm:"type:uuid;primary_key;default:uuid_generate_v4()"`
	TenantID      uuid.UUID `json:"tenant_id" gorm:"type:uuid;not null;index"`
	Name          string    `json:"name" gorm:"type:varchar(50);not null"`
	Color         string    `json:"color" gorm:"type:varchar(7);default:'#6B7280'"`
	IsAIGenerated bool      `json:"is_ai_generated" gorm:"not null;default:false"`
	UsageCount    int       `json:"usage_count" gorm:"not null;default:0"`
	CreatedAt     time.Time `json:"created_at" gorm:"not null;default:now()"`
}

type AIProcessingJob struct {
	ID               uuid.UUID        `json:"id" gorm:"type:uuid;primary_key;default:uuid_generate_v4()"`
	TenantID         uuid.UUID        `json:"tenant_id" gorm:"type:uuid;not null"`
	DocumentID       uuid.UUID        `json:"document_id" gorm:"type:uuid;not null"`
	JobType          string           `json:"job_type" gorm:"type:varchar(50);not null"`
	Status           ProcessingStatus `json:"status" gorm:"type:processing_status;not null;default:'queued'"`
	Priority         int              `json:"priority" gorm:"not null;default:5"`
	Attempts         int              `json:"attempts" gorm:"not null;default:0"`
	MaxAttempts      int              `json:"max_attempts" gorm:"not null;default:3"`
	ErrorMessage     string           `json:"error_message" gorm:"type:text"`
	Result           JSONB            `json:"result" gorm:"type:jsonb"`
	ProcessingTimeMs int              `json:"processing_time_ms"`
	CreatedAt        time.Time        `json:"created_at" gorm:"not null;default:now()"`
	StartedAt        *time.Time       `json:"started_at"`
	CompletedAt      *time.Time       `json:"completed_at"`
}

// Request/Response DTOs
type LoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=8"`
}

type RegisterRequest struct {
	Email     string `json:"email" binding:"required,email"`
	Password  string `json:"password" binding:"required,min=8"`
	FirstName string `json:"first_name" binding:"required"`
	LastName  string `json:"last_name" binding:"required"`
	TenantID  string `json:"tenant_id" binding:"required,uuid"`
}

type DocumentUploadRequest struct {
	FolderID    string   `form:"folder_id" binding:"omitempty,uuid"`
	Title       string   `form:"title" binding:"omitempty,max=255"`
	Description string   `form:"description" binding:"omitempty"`
	Tags        []string `form:"tags" binding:"omitempty"`
	Categories  []string `form:"categories" binding:"omitempty"`
}

type DocumentSearchRequest struct {
	Query      string   `json:"query" binding:"required"`
	FolderID   string   `json:"folder_id" binding:"omitempty,uuid"`
	Tags       []string `json:"tags" binding:"omitempty"`
	Categories []string `json:"categories" binding:"omitempty"`
	DateFrom   string   `json:"date_from" binding:"omitempty"`
	DateTo     string   `json:"date_to" binding:"omitempty"`
	Page       int      `json:"page" binding:"omitempty,min=1"`
	PageSize   int      `json:"page_size" binding:"omitempty,min=1,max=100"`
}

type AIAnalyzeRequest struct {
	DocumentID string   `json:"document_id" binding:"required,uuid"`
	Operations []string `json:"operations" binding:"required,min=1"`
}

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
