package services

import (
	"context"
	"crypto/rand"
	"encoding/base32"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/archivus/archivus/internal/domain/repositories"
	"github.com/archivus/archivus/internal/infrastructure/database/models"
	"github.com/google/uuid"
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

// UserService handles user management and authentication with Supabase
type UserService struct {
	userRepo     repositories.UserRepository
	tenantRepo   repositories.TenantRepository
	auditRepo    repositories.AuditLogRepository
	supabaseAuth SupabaseAuthService
	emailService EmailService
	config       UserServiceConfig
	cacheService CacheService
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
}

// NewUserService creates a new user service with Supabase
func NewUserService(
	userRepo repositories.UserRepository,
	tenantRepo repositories.TenantRepository,
	auditRepo repositories.AuditLogRepository,
	supabaseAuth SupabaseAuthService,
	emailService EmailService,
	config UserServiceConfig,
	cacheService CacheService,
) *UserService {
	return &UserService{
		userRepo:     userRepo,
		tenantRepo:   tenantRepo,
		auditRepo:    auditRepo,
		supabaseAuth: supabaseAuth,
		emailService: emailService,
		config:       config,
		cacheService: cacheService,
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
	Permissions    []string       `json:"permissions"`
	LastLogin      *time.Time     `json:"last_login"`
	PasswordExpiry *time.Time     `json:"password_expiry,omitempty"`
	MFAEnabled     bool           `json:"mfa_enabled"`
	Tenant         *models.Tenant `json:"tenant"`
}

// CreateUser creates a new user account with Supabase Auth
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

	// Check if user already exists locally
	existing, err := s.userRepo.GetByEmail(ctx, params.TenantID, params.Email)
	if err == nil && existing != nil {
		return nil, ErrUserExists
	}

	// Create user in Supabase Auth first
	metadata := map[string]interface{}{
		"tenant_id":  params.TenantID.String(),
		"first_name": params.FirstName,
		"last_name":  params.LastName,
		"role":       string(params.Role),
		"department": params.Department,
		"job_title":  params.JobTitle,
	}

	supabaseUser, err := s.supabaseAuth.SignUpWithEmail(params.Email, params.Password, metadata)
	if err != nil {
		return nil, fmt.Errorf("failed to create user in Supabase: %w", err)
	}

	// Create user in local database
	user := &models.User{
		ID:            supabaseUser.ID, // Use Supabase UUID
		TenantID:      params.TenantID,
		Email:         strings.ToLower(params.Email),
		FirstName:     params.FirstName,
		LastName:      params.LastName,
		Role:          params.Role,
		Department:    params.Department,
		JobTitle:      params.JobTitle,
		IsActive:      true,
		EmailVerified: supabaseUser.EmailConfirmedAt != nil,
		MFAEnabled:    false,
		Preferences:   models.JSONB{},
		NotificationSettings: models.JSONB{
			"email_notifications": true,
			"task_reminders":      true,
			"document_alerts":     true,
		},
	}

	if err := s.userRepo.Create(ctx, user); err != nil {
		// Rollback Supabase user creation if local creation fails
		s.supabaseAuth.AdminDeleteUser(supabaseUser.ID.String())
		return nil, fmt.Errorf("failed to create local user: %w", err)
	}

	// Send welcome email
	if s.emailService != nil {
		go func() {
			s.emailService.SendWelcomeEmail(context.Background(), user.Email, user.FirstName)
		}()
	}

	// Create audit log
	s.createAuditLog(ctx, params.TenantID, user.ID, user.ID, models.AuditCreate, "User created")

	return user, nil
}

// Login authenticates a user using Supabase
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

	// Authenticate with Supabase
	authResponse, err := s.supabaseAuth.SignInWithEmail(params.Email, params.Password)
	if err != nil {
		return nil, ErrInvalidCredentials
	}

	// Sync Supabase user with local database
	user, err := s.syncSupabaseUser(ctx, authResponse.User, tenant.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to sync user: %w", err)
	}

	// Check if user is active
	if !user.IsActive {
		return nil, ErrUserInactive
	}

	// Handle MFA if enabled
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

	// Update last login
	now := time.Now()
	user.LastLoginAt = &now
	s.userRepo.Update(ctx, user)

	// Create audit log
	s.createAuditLog(ctx, tenant.ID, user.ID, user.ID, models.AuditRead, "User logged in")

	return &LoginResult{
		User:         user,
		Token:        authResponse.AccessToken,
		RefreshToken: authResponse.RefreshToken,
		RequiresMFA:  false,
		ExpiresAt:    authResponse.ExpiresAt,
	}, nil
}

// ValidateToken validates a Supabase access token
func (s *UserService) ValidateToken(ctx context.Context, token string) (*models.User, error) {
	// Validate token with Supabase
	supabaseUser, err := s.supabaseAuth.ValidateToken(token)
	if err != nil {
		return nil, fmt.Errorf("invalid Supabase token: %w", err)
	}

	// Get local user
	user, err := s.userRepo.GetByID(ctx, supabaseUser.ID)
	if err != nil {
		return nil, fmt.Errorf("user not found: %w", err)
	}

	if !user.IsActive {
		return nil, ErrUserInactive
	}

	return user, nil
}

// GetUserProfile retrieves a user profile with caching
func (s *UserService) GetUserProfile(ctx context.Context, userID uuid.UUID) (*UserProfile, error) {
	// Try to get from cache first
	cacheKey := fmt.Sprintf(UserCacheKeyPattern, userID.String())

	if cached, err := s.cacheService.Get(ctx, cacheKey); err == nil {
		var profile UserProfile
		if json.Unmarshal([]byte(cached), &profile) == nil {
			return &profile, nil
		}
	}

	// If not in cache, get from database
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	// Get tenant information
	tenant, err := s.tenantRepo.GetByID(ctx, user.TenantID)
	if err != nil {
		return nil, fmt.Errorf("failed to get tenant: %w", err)
	}

	// Get user permissions based on role
	permissions := s.getRolePermissions(user.Role)

	// Calculate password expiry
	var passwordExpiry *time.Time
	if s.config.PasswordExpiryDays > 0 {
		expiry := user.PasswordChangedAt.AddDate(0, 0, s.config.PasswordExpiryDays)
		passwordExpiry = &expiry
	}

	profile := &UserProfile{
		User:           user,
		Permissions:    permissions,
		LastLogin:      user.LastLoginAt,
		PasswordExpiry: passwordExpiry,
		MFAEnabled:     user.MFAEnabled,
		Tenant:         tenant,
	}

	// Cache the profile for future requests
	if profileJSON, err := json.Marshal(profile); err == nil {
		s.cacheService.Set(ctx, cacheKey, string(profileJSON), CacheMediumTerm)
	}

	return profile, nil
}

// UpdateUser updates user information and invalidates cache
func (s *UserService) UpdateUser(ctx context.Context, userID uuid.UUID, updates map[string]interface{}, updatedByID uuid.UUID) (*models.User, error) {
	// First get the user
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	// Apply updates to the user model
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

	// Update in database
	if err := s.userRepo.Update(ctx, user); err != nil {
		return nil, fmt.Errorf("failed to update user: %w", err)
	}

	// Invalidate user cache
	cacheKey := fmt.Sprintf(UserCacheKeyPattern, userID.String())
	s.cacheService.Delete(ctx, cacheKey)

	// Update session cache if needed
	sessionKey := fmt.Sprintf(SessionKeyPattern, userID.String())
	if exists, _ := s.cacheService.Exists(ctx, sessionKey); exists {
		// Update session with new user info
		if userJSON, err := json.Marshal(user); err == nil {
			s.cacheService.HSet(ctx, sessionKey, "user", string(userJSON))
		}
	}

	return user, nil
}

// ChangePassword changes a user's password via Supabase
func (s *UserService) ChangePassword(ctx context.Context, userID uuid.UUID, accessToken, newPassword string) error {
	// Validate new password
	if err := s.validatePassword(newPassword); err != nil {
		return err
	}

	// Update password in Supabase
	if err := s.supabaseAuth.UpdatePassword(accessToken, newPassword); err != nil {
		return fmt.Errorf("failed to update password: %w", err)
	}

	// Update local record
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return ErrUserNotFound
	}

	user.PasswordChangedAt = time.Now()
	s.userRepo.Update(ctx, user)

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

// ResetPassword initiates password reset process via Supabase
func (s *UserService) ResetPassword(ctx context.Context, tenantSubdomain, email string) error {
	// Get tenant
	tenant, err := s.tenantRepo.GetBySubdomain(ctx, tenantSubdomain)
	if err != nil {
		return nil // Don't reveal if tenant exists
	}

	// Check if user exists locally
	_, err = s.userRepo.GetByEmail(ctx, tenant.ID, email)
	if err != nil {
		return nil // Don't reveal if user exists
	}

	// Send reset email via Supabase
	if err := s.supabaseAuth.ResetPasswordForEmail(email); err != nil {
		return fmt.Errorf("failed to send reset email: %w", err)
	}

	return nil
}

// SyncSupabaseUser syncs a Supabase user with local database
func (s *UserService) syncSupabaseUser(ctx context.Context, supabaseUser *SupabaseUser, tenantID uuid.UUID) (*models.User, error) {
	// Check if user exists locally
	user, err := s.userRepo.GetByID(ctx, supabaseUser.ID)
	if err != nil {
		// User doesn't exist locally, create them
		user = &models.User{
			ID:            supabaseUser.ID,
			TenantID:      tenantID,
			Email:         supabaseUser.Email,
			FirstName:     s.getStringFromMetadata(supabaseUser.UserMetadata, "first_name"),
			LastName:      s.getStringFromMetadata(supabaseUser.UserMetadata, "last_name"),
			Role:          s.getRoleFromMetadata(supabaseUser.UserMetadata),
			Department:    s.getStringFromMetadata(supabaseUser.UserMetadata, "department"),
			JobTitle:      s.getStringFromMetadata(supabaseUser.UserMetadata, "job_title"),
			IsActive:      true,
			EmailVerified: supabaseUser.EmailConfirmedAt != nil,
			LastLoginAt:   supabaseUser.LastSignInAt,
			Preferences:   models.JSONB{},
			NotificationSettings: models.JSONB{
				"email_notifications": true,
				"task_reminders":      true,
				"document_alerts":     true,
			},
		}

		if err := s.userRepo.Create(ctx, user); err != nil {
			return nil, fmt.Errorf("failed to create local user: %w", err)
		}
	} else {
		// Update existing user with Supabase data
		user.Email = supabaseUser.Email
		user.EmailVerified = supabaseUser.EmailConfirmedAt != nil
		user.LastLoginAt = supabaseUser.LastSignInAt

		if err := s.userRepo.Update(ctx, user); err != nil {
			return nil, fmt.Errorf("failed to update local user: %w", err)
		}
	}

	return user, nil
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

	// Also disable in Supabase (admin operation)
	s.supabaseAuth.AdminUpdateUser(userID.String(), map[string]interface{}{
		"user_disabled": true,
	})

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

	// Also enable in Supabase (admin operation)
	s.supabaseAuth.AdminUpdateUser(userID.String(), map[string]interface{}{
		"user_disabled": false,
	})

	// Create audit log
	s.createAuditLog(ctx, user.TenantID, reactivatedBy, userID, models.AuditUpdate, "User reactivated")

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

func (s *UserService) getStringFromMetadata(metadata map[string]interface{}, key string) string {
	if val, ok := metadata[key]; ok {
		if str, ok := val.(string); ok {
			return str
		}
	}
	return ""
}

func (s *UserService) getRoleFromMetadata(metadata map[string]interface{}) models.UserRole {
	roleStr := s.getStringFromMetadata(metadata, "role")
	switch roleStr {
	case string(models.UserRoleAdmin):
		return models.UserRoleAdmin
	case string(models.UserRoleManager):
		return models.UserRoleManager
	case string(models.UserRoleUser):
		return models.UserRoleUser
	case string(models.UserRoleViewer):
		return models.UserRoleViewer
	case string(models.UserRoleAccountant):
		return models.UserRoleAccountant
	case string(models.UserRoleCompliance):
		return models.UserRoleCompliance
	default:
		return models.UserRoleUser
	}
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

// External service interfaces are now defined in external_interfaces.go
// This avoids duplication and centralizes interface definitions

// CacheUserSession stores user session data in Redis
func (s *UserService) CacheUserSession(ctx context.Context, userID uuid.UUID, sessionToken string, user *models.User) error {
	sessionKey := fmt.Sprintf(SessionKeyPattern, sessionToken)

	// Store session as a hash for easy field updates
	sessionData := map[string]interface{}{
		"user_id":    userID.String(),
		"created_at": time.Now().Unix(),
		"last_seen":  time.Now().Unix(),
	}

	if user != nil {
		if userJSON, err := json.Marshal(user); err == nil {
			sessionData["user"] = string(userJSON)
		}
	}

	// Set each field
	for field, value := range sessionData {
		if err := s.cacheService.HSet(ctx, sessionKey, field, value); err != nil {
			return fmt.Errorf("failed to set session field %s: %w", field, err)
		}
	}

	// Set session expiration
	s.cacheService.Set(ctx, sessionKey+"_ttl", "1", SessionDuration)

	return nil
}

// GetUserSession retrieves user session from Redis
func (s *UserService) GetUserSession(ctx context.Context, sessionToken string) (*models.User, error) {
	sessionKey := fmt.Sprintf(SessionKeyPattern, sessionToken)

	// Check if session exists
	exists, err := s.cacheService.Exists(ctx, sessionKey)
	if err != nil || !exists {
		return nil, fmt.Errorf("session not found")
	}

	// Get user data from session
	userJSON, err := s.cacheService.HGet(ctx, sessionKey, "user")
	if err != nil {
		return nil, fmt.Errorf("failed to get user from session: %w", err)
	}

	var user models.User
	if err := json.Unmarshal([]byte(userJSON), &user); err != nil {
		return nil, fmt.Errorf("failed to unmarshal user data: %w", err)
	}

	// Update last seen
	s.cacheService.HSet(ctx, sessionKey, "last_seen", time.Now().Unix())

	return &user, nil
}

// InvalidateUserSession removes user session from Redis
func (s *UserService) InvalidateUserSession(ctx context.Context, sessionToken string) error {
	sessionKey := fmt.Sprintf(SessionKeyPattern, sessionToken)
	return s.cacheService.Delete(ctx, sessionKey)
}

// IncrementUserLoginCount increments user login counter
func (s *UserService) IncrementUserLoginCount(ctx context.Context, userID uuid.UUID) (int64, error) {
	counterKey := fmt.Sprintf("user_login_count:%s", userID.String())
	return s.cacheService.Increment(ctx, counterKey)
}

// GetActiveUserSessions gets list of active sessions for a user (for security/admin purposes)
func (s *UserService) GetActiveUserSessions(ctx context.Context, userID uuid.UUID) ([]string, error) {
	sessionsKey := fmt.Sprintf("user_sessions:%s", userID.String())
	return s.cacheService.SMembers(ctx, sessionsKey)
}

// AddUserToActiveList adds user to active users set
func (s *UserService) AddUserToActiveList(ctx context.Context, userID uuid.UUID) error {
	activeUsersKey := "active_users"
	return s.cacheService.SAdd(ctx, activeUsersKey, userID.String())
}

// CacheUserPermissions caches user permissions for quick access
func (s *UserService) CacheUserPermissions(ctx context.Context, userID uuid.UUID, permissions []string) error {
	permissionsKey := fmt.Sprintf("user_permissions:%s", userID.String())

	// Store as a set for efficient membership testing
	permissionInterfaces := make([]interface{}, len(permissions))
	for i, perm := range permissions {
		permissionInterfaces[i] = perm
	}

	if err := s.cacheService.SAdd(ctx, permissionsKey, permissionInterfaces...); err != nil {
		return fmt.Errorf("failed to cache permissions: %w", err)
	}

	// Set expiration
	s.cacheService.Set(ctx, permissionsKey+"_ttl", "1", CacheLongTerm)

	return nil
}

// Example: Rate limiting per user
func (s *UserService) CheckUserRateLimit(ctx context.Context, userID uuid.UUID, action string, limit int64) (bool, error) {
	rateLimitKey := fmt.Sprintf("rate_limit:%s:%s", userID.String(), action)

	// Increment counter
	count, err := s.cacheService.Increment(ctx, rateLimitKey)
	if err != nil {
		return false, fmt.Errorf("failed to increment rate limit: %w", err)
	}

	// Set expiration on first increment
	if count == 1 {
		s.cacheService.Set(ctx, rateLimitKey+"_ttl", "1", RateLimitWindow)
	}

	return count <= limit, nil
}
