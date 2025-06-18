package services

import (
	"context"
	"io"
	"time"

	"github.com/archivus/archivus/internal/infrastructure/database/models"
	"github.com/google/uuid"
)

// External service interfaces that our domain services depend on

// StorageService interface for file storage operations (Supabase Storage compatible)
type StorageService interface {
	Store(ctx context.Context, params StorageParams) (string, error)
	Get(ctx context.Context, path string) (io.ReadCloser, error)
	Delete(ctx context.Context, path string) error
	GeneratePresignedURL(ctx context.Context, path string, expiry time.Duration) (string, error)
	GetPublicURL(bucketName, filePath string) string
}

// StorageParams contains parameters for storing files
type StorageParams struct {
	TenantID    uuid.UUID
	FileReader  io.Reader
	Filename    string
	ContentType string
	Size        int64
	BucketName  string // Supabase bucket name
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

// EmailService interface for email operations
type EmailService interface {
	SendEmailVerification(ctx context.Context, email, token string) error
	SendPasswordReset(ctx context.Context, email, token string) error
	SendWelcomeEmail(ctx context.Context, email, name string) error
}

// SupabaseAuthService interface for Supabase authentication operations
type SupabaseAuthService interface {
	// User management
	SignUpWithEmail(email, password string, metadata map[string]interface{}) (*SupabaseUser, error)
	SignInWithEmail(email, password string) (*SupabaseAuthResponse, error)
	SignOut(accessToken string) error

	// Token management
	ValidateToken(accessToken string) (*SupabaseUser, error)
	RefreshSession(refreshToken string) (*SupabaseAuthResponse, error)

	// Password management
	ResetPasswordForEmail(email string) error
	UpdatePassword(accessToken, newPassword string) error

	// User profile
	GetUser(accessToken string) (*SupabaseUser, error)
	UpdateUser(accessToken string, updates map[string]interface{}) (*SupabaseUser, error)

	// Admin operations (using service key)
	AdminCreateUser(email, password string, metadata map[string]interface{}, emailConfirmed bool) (*SupabaseUser, error)
	AdminGetUser(userID string) (*SupabaseUser, error)
	AdminUpdateUser(userID string, updates map[string]interface{}) (*SupabaseUser, error)
	AdminDeleteUser(userID string) error
}

// SupabaseUser represents a user from Supabase Auth
type SupabaseUser struct {
	ID               uuid.UUID              `json:"id"`
	Email            string                 `json:"email"`
	EmailConfirmedAt *time.Time             `json:"email_confirmed_at"`
	Phone            string                 `json:"phone,omitempty"`
	PhoneConfirmedAt *time.Time             `json:"phone_confirmed_at,omitempty"`
	UserMetadata     map[string]interface{} `json:"user_metadata"`
	AppMetadata      map[string]interface{} `json:"app_metadata"`
	CreatedAt        time.Time              `json:"created_at"`
	UpdatedAt        time.Time              `json:"updated_at"`
	LastSignInAt     *time.Time             `json:"last_sign_in_at"`
}

// SupabaseAuthResponse represents Supabase auth response
type SupabaseAuthResponse struct {
	User         *SupabaseUser `json:"user"`
	Session      *Session      `json:"session"`
	AccessToken  string        `json:"access_token"`
	RefreshToken string        `json:"refresh_token"`
	ExpiresAt    time.Time     `json:"expires_at"`
}

// Session represents a Supabase session
type Session struct {
	AccessToken  string        `json:"access_token"`
	RefreshToken string        `json:"refresh_token"`
	ExpiresAt    time.Time     `json:"expires_at"`
	TokenType    string        `json:"token_type"`
	User         *SupabaseUser `json:"user"`
}
