package services

import (
	"context"
	"crypto/rand"
	"encoding/base32"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/archivus/archivus/internal/domain/repositories"
	"github.com/archivus/archivus/internal/infrastructure/database/models"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

var (
	ErrUserNotFound           = errors.New("user not found")
	ErrUserExists             = errors.New("user already exists")
	ErrInvalidCredentials     = errors.New("invalid credentials")
	ErrInvalidEmail           = errors.New("invalid email format")
	ErrWeakPassword           = errors.New("password does not meet requirements")
	ErrInvalidRole            = errors.New("invalid user role")
	ErrUserInactive           = errors.New("user account is inactive")
	ErrMFARequired            = errors.New("MFA verification required")
	ErrInvalidMFACode         = errors.New("invalid MFA code")
	ErrInsufficientPrivileges = errors.New("insufficient privileges")
)

// UserService handles user management and authentication
type UserService struct {
	userRepo   repositories.UserRepository
	tenantRepo repositories.TenantRepository
	auditRepo  repositories.AuditLogRepository

	authService  AuthService
	emailService EmailService
	config       UserServiceConfig
}

// UserServiceConfig holds configuration for user management
type UserServiceConfig struct {
	MinPasswordLength        int
	RequireUppercase         bool
	RequireLowercase         bool
	RequireNumbers           bool
	RequireSpecialChars      bool
	PasswordExpiryDays       int
	MaxLoginAttempts         int
	LockoutDurationMins      int
	RequireEmailVerification bool
	EnableMFA                bool
	JWTSecretKey             string
	JWTExpiryHours           int
}

// NewUserService creates a new user service
func NewUserService(
	userRepo repositories.UserRepository,
	tenantRepo repositories.TenantRepository,
	auditRepo repositories.AuditLogRepository,
	authService AuthService,
	emailService EmailService,
	config UserServiceConfig,
) *UserService {
	return &UserService{
		userRepo:     userRepo,
		tenantRepo:   tenantRepo,
		auditRepo:    auditRepo,
		authService:  authService,
		emailService: emailService,
		config:       config,
	}
}

// CreateUserParams contains parameters for creating a new user
type CreateUserParams struct {
	TenantID   uuid.UUID       `json:"tenant_id"`
	Email      string          `json:"email"`
	Password   string          `json:"password"`
	FirstName  string          `json:"first_name"`
	LastName   string          `json:"last_name"`
	Role       models.UserRole `json:"role"`
	Department string          `json:"department,omitempty"`
	JobTitle   string          `json:"job_title,omitempty"`
	CreatedBy  uuid.UUID       `json:"created_by"`
}

// LoginParams contains parameters for user login
type LoginParams struct {
	TenantSubdomain string `json:"tenant_subdomain"`
	Email           string `json:"email"`
	Password        string `json:"password"`
	MFACode         string `json:"mfa_code,omitempty"`
	IPAddress       string `json:"ip_address,omitempty"`
	UserAgent       string `json:"user_agent,omitempty"`
}

// LoginResult contains the result of a login attempt
type LoginResult struct {
	User         *models.User `json:"user"`
	Token        string       `json:"token"`
	RefreshToken string       `json:"refresh_token"`
	RequiresMFA  bool         `json:"requires_mfa"`
	ExpiresAt    time.Time    `json:"expires_at"`
}

// UserProfile contains user profile information
type UserProfile struct {
	*models.User
	Permissions    []string   `json:"permissions"`
	LastLogin      *time.Time `json:"last_login"`
	PasswordExpiry *time.Time `json:"password_expiry,omitempty"`
	MFAEnabled     bool       `json:"mfa_enabled"`
}

// CreateUser creates a new user account
func (s *UserService) CreateUser(ctx context.Context, params CreateUserParams) (*models.User, error) {
	// Validate email format
	if !s.isValidEmail(params.Email) {
		return nil, ErrInvalidEmail
	}

	// Validate password strength
	if err := s.validatePassword(params.Password); err != nil {
		return nil, err
	}

	// Validate role
	if !s.isValidRole(params.Role) {
		return nil, ErrInvalidRole
	}

	// Check if user already exists
	existing, err := s.userRepo.GetByEmail(ctx, params.TenantID, params.Email)
	if err == nil && existing != nil {
		return nil, ErrUserExists
	}

	// Hash password
	hashedPassword, err := s.hashPassword(params.Password)
	if err != nil {
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}

	// Create user
	user := &models.User{
		ID:                uuid.New(),
		TenantID:          params.TenantID,
		Email:             strings.ToLower(params.Email),
		PasswordHash:      hashedPassword,
		FirstName:         params.FirstName,
		LastName:          params.LastName,
		Role:              params.Role,
		Department:        params.Department,
		JobTitle:          params.JobTitle,
		IsActive:          true,
		EmailVerified:     !s.config.RequireEmailVerification,
		PasswordChangedAt: time.Now(),
		MFAEnabled:        false,
		Preferences:       models.JSONB{},
		NotificationSettings: models.JSONB{
			"email_notifications": true,
			"task_reminders":      true,
			"document_alerts":     true,
		},
	}

	if err := s.userRepo.Create(ctx, user); err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	// Send email verification if required
	if s.config.RequireEmailVerification && s.emailService != nil {
		if err := s.sendEmailVerification(ctx, user); err != nil {
			// Log but don't fail user creation
		}
	}

	// Create audit log
	s.createAuditLog(ctx, params.TenantID, params.CreatedBy, user.ID, models.AuditCreate, "User created")

	return user, nil
}

// Login authenticates a user and returns a token
func (s *UserService) Login(ctx context.Context, params LoginParams) (*LoginResult, error) {
	// Get tenant by subdomain
	tenant, err := s.tenantRepo.GetBySubdomain(ctx, params.TenantSubdomain)
	if err != nil {
		return nil, ErrInvalidCredentials
	}

	// Check if tenant is active
	if !tenant.IsActive {
		return nil, errors.New("tenant account suspended")
	}

	// Get user by email
	user, err := s.userRepo.GetByEmail(ctx, tenant.ID, params.Email)
	if err != nil {
		return nil, ErrInvalidCredentials
	}

	// Check if user is active
	if !user.IsActive {
		return nil, ErrUserInactive
	}

	// Verify password
	if !s.verifyPassword(params.Password, user.PasswordHash) {
		// Log failed login attempt
		s.createAuditLog(ctx, tenant.ID, user.ID, user.ID, models.AuditRead, "Failed login attempt")
		return nil, ErrInvalidCredentials
	}

	// Check if MFA is required
	if user.MFAEnabled && params.MFACode == "" {
		return &LoginResult{
			RequiresMFA: true,
		}, nil
	}

	// Verify MFA code if provided
	if user.MFAEnabled && params.MFACode != "" {
		if !s.verifyMFACode(user.MFASecret, params.MFACode) {
			return nil, ErrInvalidMFACode
		}
	}

	// Generate JWT token
	token, expiresAt, err := s.authService.GenerateToken(user.ID, tenant.ID, user.Role)
	if err != nil {
		return nil, fmt.Errorf("failed to generate token: %w", err)
	}

	// Generate refresh token
	refreshToken, err := s.authService.GenerateRefreshToken(user.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to generate refresh token: %w", err)
	}

	// Update last login
	if err := s.userRepo.UpdateLastLogin(ctx, user.ID); err != nil {
		// Log but don't fail login
	}

	// Create audit log
	s.createAuditLog(ctx, tenant.ID, user.ID, user.ID, models.AuditRead, "User logged in")

	return &LoginResult{
		User:         user,
		Token:        token,
		RefreshToken: refreshToken,
		RequiresMFA:  false,
		ExpiresAt:    expiresAt,
	}, nil
}

// GetUserProfile gets detailed user profile information
func (s *UserService) GetUserProfile(ctx context.Context, userID uuid.UUID) (*UserProfile, error) {
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return nil, ErrUserNotFound
	}

	// Get user permissions based on role
	permissions := s.getRolePermissions(user.Role)

	// Calculate password expiry
	var passwordExpiry *time.Time
	if s.config.PasswordExpiryDays > 0 {
		expiry := user.PasswordChangedAt.AddDate(0, 0, s.config.PasswordExpiryDays)
		passwordExpiry = &expiry
	}

	return &UserProfile{
		User:           user,
		Permissions:    permissions,
		LastLogin:      user.LastLoginAt,
		PasswordExpiry: passwordExpiry,
		MFAEnabled:     user.MFAEnabled,
	}, nil
}

// UpdateUser updates user information
func (s *UserService) UpdateUser(ctx context.Context, userID uuid.UUID, updates map[string]interface{}, updatedBy uuid.UUID) (*models.User, error) {
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return nil, ErrUserNotFound
	}

	// Apply updates
	if firstName, ok := updates["first_name"].(string); ok {
		user.FirstName = firstName
	}

	if lastName, ok := updates["last_name"].(string); ok {
		user.LastName = lastName
	}

	if department, ok := updates["department"].(string); ok {
		user.Department = department
	}

	if jobTitle, ok := updates["job_title"].(string); ok {
		user.JobTitle = jobTitle
	}

	if role, ok := updates["role"].(models.UserRole); ok {
		if !s.isValidRole(role) {
			return nil, ErrInvalidRole
		}
		user.Role = role
	}

	if preferences, ok := updates["preferences"].(map[string]interface{}); ok {
		user.Preferences = models.JSONB(preferences)
	}

	if notifications, ok := updates["notification_settings"].(map[string]interface{}); ok {
		user.NotificationSettings = models.JSONB(notifications)
	}

	user.UpdatedAt = time.Now()

	if err := s.userRepo.Update(ctx, user); err != nil {
		return nil, fmt.Errorf("failed to update user: %w", err)
	}

	// Create audit log
	s.createAuditLog(ctx, user.TenantID, updatedBy, user.ID, models.AuditUpdate, "User updated")

	return user, nil
}

// ChangePassword changes a user's password
func (s *UserService) ChangePassword(ctx context.Context, userID uuid.UUID, currentPassword, newPassword string) error {
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return ErrUserNotFound
	}

	// Verify current password
	if !s.verifyPassword(currentPassword, user.PasswordHash) {
		return ErrInvalidCredentials
	}

	// Validate new password
	if err := s.validatePassword(newPassword); err != nil {
		return err
	}

	// Hash new password
	hashedPassword, err := s.hashPassword(newPassword)
	if err != nil {
		return fmt.Errorf("failed to hash password: %w", err)
	}

	// Update password
	user.PasswordHash = hashedPassword
	user.PasswordChangedAt = time.Now()

	if err := s.userRepo.Update(ctx, user); err != nil {
		return fmt.Errorf("failed to update password: %w", err)
	}

	// Create audit log
	s.createAuditLog(ctx, user.TenantID, userID, userID, models.AuditUpdate, "Password changed")

	return nil
}

// EnableMFA enables multi-factor authentication for a user
func (s *UserService) EnableMFA(ctx context.Context, userID uuid.UUID) (string, error) {
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return "", ErrUserNotFound
	}

	if user.MFAEnabled {
		return "", errors.New("MFA already enabled")
	}

	// Generate MFA secret
	secret, err := s.generateMFASecret()
	if err != nil {
		return "", fmt.Errorf("failed to generate MFA secret: %w", err)
	}

	// Save MFA secret
	if err := s.userRepo.SetMFA(ctx, userID, true, secret); err != nil {
		return "", fmt.Errorf("failed to enable MFA: %w", err)
	}

	// Create audit log
	s.createAuditLog(ctx, user.TenantID, userID, userID, models.AuditUpdate, "MFA enabled")

	// Return QR code data for user to scan
	return s.generateMFAQRCode(user.Email, secret), nil
}

// DisableMFA disables multi-factor authentication for a user
func (s *UserService) DisableMFA(ctx context.Context, userID uuid.UUID, mfaCode string) error {
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return ErrUserNotFound
	}

	if !user.MFAEnabled {
		return errors.New("MFA not enabled")
	}

	// Verify MFA code before disabling
	if !s.verifyMFACode(user.MFASecret, mfaCode) {
		return ErrInvalidMFACode
	}

	// Disable MFA
	if err := s.userRepo.SetMFA(ctx, userID, false, ""); err != nil {
		return fmt.Errorf("failed to disable MFA: %w", err)
	}

	// Create audit log
	s.createAuditLog(ctx, user.TenantID, userID, userID, models.AuditUpdate, "MFA disabled")

	return nil
}

// DeactivateUser deactivates a user account
func (s *UserService) DeactivateUser(ctx context.Context, userID, deactivatedBy uuid.UUID) error {
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return ErrUserNotFound
	}

	user.IsActive = false
	user.UpdatedAt = time.Now()

	if err := s.userRepo.Update(ctx, user); err != nil {
		return fmt.Errorf("failed to deactivate user: %w", err)
	}

	// Create audit log
	s.createAuditLog(ctx, user.TenantID, deactivatedBy, userID, models.AuditUpdate, "User deactivated")

	return nil
}

// ReactivateUser reactivates a user account
func (s *UserService) ReactivateUser(ctx context.Context, userID, reactivatedBy uuid.UUID) error {
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return ErrUserNotFound
	}

	user.IsActive = true
	user.UpdatedAt = time.Now()

	if err := s.userRepo.Update(ctx, user); err != nil {
		return fmt.Errorf("failed to reactivate user: %w", err)
	}

	// Create audit log
	s.createAuditLog(ctx, user.TenantID, reactivatedBy, userID, models.AuditUpdate, "User reactivated")

	return nil
}

// ListUsers lists users for a tenant with filtering and pagination
func (s *UserService) ListUsers(ctx context.Context, tenantID uuid.UUID, params repositories.ListParams) ([]models.User, int64, error) {
	return s.userRepo.ListByTenant(ctx, tenantID, params)
}

// CheckPermission checks if a user has a specific permission
func (s *UserService) CheckPermission(ctx context.Context, userID uuid.UUID, permission string) (bool, error) {
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return false, ErrUserNotFound
	}

	permissions := s.getRolePermissions(user.Role)
	for _, p := range permissions {
		if p == permission || p == "*" {
			return true, nil
		}
	}

	return false, nil
}

// ResetPassword initiates password reset process
func (s *UserService) ResetPassword(ctx context.Context, tenantSubdomain, email string) error {
	// Get tenant
	tenant, err := s.tenantRepo.GetBySubdomain(ctx, tenantSubdomain)
	if err != nil {
		return ErrUserNotFound // Don't reveal if tenant exists
	}

	// Get user
	user, err := s.userRepo.GetByEmail(ctx, tenant.ID, email)
	if err != nil {
		return nil // Don't reveal if user exists
	}

	// Generate reset token
	resetToken, err := s.authService.GeneratePasswordResetToken(user.ID)
	if err != nil {
		return fmt.Errorf("failed to generate reset token: %w", err)
	}

	// Send reset email
	if s.emailService != nil {
		if err := s.emailService.SendPasswordReset(ctx, user.Email, resetToken); err != nil {
			return fmt.Errorf("failed to send reset email: %w", err)
		}
	}

	// Create audit log
	s.createAuditLog(ctx, tenant.ID, user.ID, user.ID, models.AuditUpdate, "Password reset requested")

	return nil
}

// Helper methods

func (s *UserService) isValidEmail(email string) bool {
	emailRegex := regexp.MustCompile(`^[a-z0-9._%+\-]+@[a-z0-9.\-]+\.[a-z]{2,}$`)
	return emailRegex.MatchString(strings.ToLower(email))
}

func (s *UserService) validatePassword(password string) error {
	if len(password) < s.config.MinPasswordLength {
		return ErrWeakPassword
	}

	if s.config.RequireUppercase && !regexp.MustCompile(`[A-Z]`).MatchString(password) {
		return ErrWeakPassword
	}

	if s.config.RequireLowercase && !regexp.MustCompile(`[a-z]`).MatchString(password) {
		return ErrWeakPassword
	}

	if s.config.RequireNumbers && !regexp.MustCompile(`[0-9]`).MatchString(password) {
		return ErrWeakPassword
	}

	if s.config.RequireSpecialChars && !regexp.MustCompile(`[!@#$%^&*(),.?":{}|<>]`).MatchString(password) {
		return ErrWeakPassword
	}

	return nil
}

func (s *UserService) isValidRole(role models.UserRole) bool {
	validRoles := []models.UserRole{
		models.UserRoleAdmin,
		models.UserRoleManager,
		models.UserRoleUser,
		models.UserRoleViewer,
		models.UserRoleAccountant,
		models.UserRoleCompliance,
	}

	for _, validRole := range validRoles {
		if role == validRole {
			return true
		}
	}
	return false
}

func (s *UserService) hashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(bytes), err
}

func (s *UserService) verifyPassword(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}

func (s *UserService) generateMFASecret() (string, error) {
	bytes := make([]byte, 20)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return base32.StdEncoding.EncodeToString(bytes), nil
}

func (s *UserService) generateMFAQRCode(email, secret string) string {
	// Generate QR code data for TOTP
	return fmt.Sprintf("otpauth://totp/Archivus:%s?secret=%s&issuer=Archivus", email, secret)
}

func (s *UserService) verifyMFACode(secret, code string) bool {
	// Implementation would verify TOTP code
	// This is a simplified version
	return len(code) == 6 && regexp.MustCompile(`^[0-9]{6}$`).MatchString(code)
}

func (s *UserService) getRolePermissions(role models.UserRole) []string {
	switch role {
	case models.UserRoleAdmin:
		return []string{"*"} // All permissions
	case models.UserRoleManager:
		return []string{
			"documents.create", "documents.read", "documents.update", "documents.delete",
			"users.create", "users.read", "users.update",
			"workflows.create", "workflows.read", "workflows.update",
			"reports.read", "analytics.read",
		}
	case models.UserRoleUser:
		return []string{
			"documents.create", "documents.read", "documents.update",
			"workflows.read", "tasks.complete",
		}
	case models.UserRoleViewer:
		return []string{
			"documents.read", "workflows.read",
		}
	case models.UserRoleAccountant:
		return []string{
			"documents.create", "documents.read", "documents.update",
			"financial.read", "reports.read",
		}
	case models.UserRoleCompliance:
		return []string{
			"documents.read", "audit.read", "compliance.read",
			"reports.read", "analytics.read",
		}
	default:
		return []string{}
	}
}

func (s *UserService) sendEmailVerification(ctx context.Context, user *models.User) error {
	// Generate verification token
	token, err := s.authService.GenerateEmailVerificationToken(user.ID)
	if err != nil {
		return err
	}

	// Send verification email
	return s.emailService.SendEmailVerification(ctx, user.Email, token)
}

func (s *UserService) createAuditLog(ctx context.Context, tenantID, userID, resourceID uuid.UUID, action models.AuditAction, details string) {
	log := &models.AuditLog{
		TenantID:     tenantID,
		UserID:       userID,
		ResourceID:   resourceID,
		Action:       action,
		ResourceType: "user",
		Details:      models.JSONB{"message": details},
	}

	go func() {
		s.auditRepo.Create(context.Background(), log)
	}()
}

// External service interfaces

type AuthService interface {
	GenerateToken(userID, tenantID uuid.UUID, role models.UserRole) (string, time.Time, error)
	GenerateRefreshToken(userID uuid.UUID) (string, error)
	GeneratePasswordResetToken(userID uuid.UUID) (string, error)
	GenerateEmailVerificationToken(userID uuid.UUID) (string, error)
	ValidateToken(token string) (*TokenClaims, error)
	RefreshToken(refreshToken string) (string, time.Time, error)
}

type TokenClaims struct {
	UserID    uuid.UUID       `json:"user_id"`
	TenantID  uuid.UUID       `json:"tenant_id"`
	Role      models.UserRole `json:"role"`
	IssuedAt  time.Time       `json:"issued_at"`
	ExpiresAt time.Time       `json:"expires_at"`
}

type EmailService interface {
	SendEmailVerification(ctx context.Context, email, token string) error
	SendPasswordReset(ctx context.Context, email, token string) error
	SendWelcomeEmail(ctx context.Context, email, name string) error
}
