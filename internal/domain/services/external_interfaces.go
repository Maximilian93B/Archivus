package services

import (
	"context"
	"io"
	"time"

	"github.com/archivus/archivus/internal/infrastructure/database/models"
	"github.com/google/uuid"
)

// External service interfaces that our domain services depend on

// StorageService interface for file storage operations
type StorageService interface {
	Store(ctx context.Context, params StorageParams) (string, error)
	Get(ctx context.Context, path string) (io.ReadCloser, error)
	Delete(ctx context.Context, path string) error
	GeneratePresignedURL(ctx context.Context, path string, expiry time.Duration) (string, error)
}

// StorageParams contains parameters for storing files
type StorageParams struct {
	TenantID    uuid.UUID
	FileReader  io.Reader
	Filename    string
	ContentType string
	Size        int64
}

// AIService interface for AI/ML operations
type AIService interface {
	ExtractText(ctx context.Context, filePath string) (string, error)
	GenerateEmbedding(ctx context.Context, text string) ([]float32, error)
	GenerateSummary(ctx context.Context, text string) (string, error)
	ExtractEntities(ctx context.Context, text string) (map[string]interface{}, error)
	ClassifyDocument(ctx context.Context, text string) (models.DocumentType, float64, error)
	GenerateTags(ctx context.Context, text string) ([]string, error)
	ExtractFinancialData(ctx context.Context, text string, docType models.DocumentType) (map[string]interface{}, error)
	PerformOCR(ctx context.Context, filePath string) (string, error)
}

// AuthService interface for authentication operations
type AuthService interface {
	GenerateToken(userID, tenantID uuid.UUID, role models.UserRole) (string, time.Time, error)
	GenerateRefreshToken(userID uuid.UUID) (string, error)
	GeneratePasswordResetToken(userID uuid.UUID) (string, error)
	GenerateEmailVerificationToken(userID uuid.UUID) (string, error)
	ValidateToken(token string) (*TokenClaims, error)
	RefreshToken(refreshToken string) (string, time.Time, error)
}

// TokenClaims represents JWT token claims
type TokenClaims struct {
	UserID    uuid.UUID       `json:"user_id"`
	TenantID  uuid.UUID       `json:"tenant_id"`
	Role      models.UserRole `json:"role"`
	IssuedAt  time.Time       `json:"issued_at"`
	ExpiresAt time.Time       `json:"expires_at"`
}

// EmailService interface for email operations
type EmailService interface {
	SendEmailVerification(ctx context.Context, email, token string) error
	SendPasswordReset(ctx context.Context, email, token string) error
	SendWelcomeEmail(ctx context.Context, email, name string) error
}

// NotificationService interface for sending notifications
type NotificationService interface {
	SendTaskAssignment(ctx context.Context, task *models.WorkflowTask, userID uuid.UUID) error
	SendTaskCompletion(ctx context.Context, task *models.WorkflowTask, completedBy uuid.UUID, action string) error
	SendTaskReminder(ctx context.Context, task *models.WorkflowTask) error
	SendTaskEscalation(ctx context.Context, task *models.WorkflowTask, escalatedTo uuid.UUID) error
}

// SubscriptionService interface for subscription management
type SubscriptionService interface {
	InitializeSubscription(ctx context.Context, tenantID uuid.UUID, tier models.SubscriptionTier) error
	UpgradeSubscription(ctx context.Context, tenantID uuid.UUID, newTier models.SubscriptionTier) error
	CancelSubscription(ctx context.Context, tenantID uuid.UUID) error
	GetSubscriptionStatus(ctx context.Context, tenantID uuid.UUID) (string, error)
}
