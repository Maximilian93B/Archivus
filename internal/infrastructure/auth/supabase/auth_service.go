package supabase

import (
	"context"
	"fmt"
	"time"

	"github.com/archivus/archivus/internal/domain/services"
	"github.com/google/uuid"
	supabase "github.com/nedpals/supabase-go"
)

type AuthService struct {
	client *supabase.Client
}

type Config struct {
	URL    string
	APIKey string
}

func NewAuthService(config Config) (*AuthService, error) {
	client := supabase.CreateClient(config.URL, config.APIKey)
	if client == nil {
		return nil, fmt.Errorf("failed to create Supabase client")
	}

	return &AuthService{
		client: client,
	}, nil
}

func (s *AuthService) SignUpWithEmail(email, password string, metadata map[string]interface{}) (*services.SupabaseUser, error) {
	ctx := context.Background()

	credentials := supabase.UserCredentials{
		Email:    email,
		Password: password,
		Data:     metadata,
	}

	user, err := s.client.Auth.SignUp(ctx, credentials)
	if err != nil {
		return nil, fmt.Errorf("failed to sign up user: %w", err)
	}

	return convertToSupabaseUser(user), nil
}

func (s *AuthService) SignInWithEmail(email, password string) (*services.SupabaseAuthResponse, error) {
	ctx := context.Background()

	credentials := supabase.UserCredentials{
		Email:    email,
		Password: password,
	}

	authDetails, err := s.client.Auth.SignIn(ctx, credentials)
	if err != nil {
		return nil, fmt.Errorf("failed to sign in user: %w", err)
	}

	return &services.SupabaseAuthResponse{
		User: convertToSupabaseUser(&authDetails.User),
		Session: &services.Session{
			AccessToken:  authDetails.AccessToken,
			RefreshToken: authDetails.RefreshToken,
			ExpiresAt:    time.Now().Add(time.Duration(authDetails.ExpiresIn) * time.Second),
			TokenType:    authDetails.TokenType,
			User:         convertToSupabaseUser(&authDetails.User),
		},
		AccessToken:  authDetails.AccessToken,
		RefreshToken: authDetails.RefreshToken,
		ExpiresAt:    time.Now().Add(time.Duration(authDetails.ExpiresIn) * time.Second),
	}, nil
}

func (s *AuthService) SignOut(accessToken string) error {
	ctx := context.Background()

	err := s.client.Auth.SignOut(ctx, accessToken)
	if err != nil {
		return fmt.Errorf("failed to sign out user: %w", err)
	}
	return nil
}

func (s *AuthService) ValidateToken(accessToken string) (*services.SupabaseUser, error) {
	ctx := context.Background()

	user, err := s.client.Auth.User(ctx, accessToken)
	if err != nil {
		return nil, fmt.Errorf("failed to validate token: %w", err)
	}

	return convertToSupabaseUser(user), nil
}

func (s *AuthService) RefreshSession(refreshToken string) (*services.SupabaseAuthResponse, error) {
	ctx := context.Background()

	// Note: nedpals client expects both user token and refresh token
	// For refresh, we can pass empty string for userToken
	authDetails, err := s.client.Auth.RefreshUser(ctx, "", refreshToken)
	if err != nil {
		return nil, fmt.Errorf("failed to refresh token: %w", err)
	}

	return &services.SupabaseAuthResponse{
		User: convertToSupabaseUser(&authDetails.User),
		Session: &services.Session{
			AccessToken:  authDetails.AccessToken,
			RefreshToken: authDetails.RefreshToken,
			ExpiresAt:    time.Now().Add(time.Duration(authDetails.ExpiresIn) * time.Second),
			TokenType:    authDetails.TokenType,
			User:         convertToSupabaseUser(&authDetails.User),
		},
		AccessToken:  authDetails.AccessToken,
		RefreshToken: authDetails.RefreshToken,
		ExpiresAt:    time.Now().Add(time.Duration(authDetails.ExpiresIn) * time.Second),
	}, nil
}

func (s *AuthService) ResetPasswordForEmail(email string) error {
	ctx := context.Background()

	err := s.client.Auth.ResetPasswordForEmail(ctx, email, "")
	if err != nil {
		return fmt.Errorf("failed to send reset password email: %w", err)
	}
	return nil
}

func (s *AuthService) UpdatePassword(accessToken, newPassword string) error {
	ctx := context.Background()

	updates := map[string]interface{}{
		"password": newPassword,
	}

	_, err := s.client.Auth.UpdateUser(ctx, accessToken, updates)
	if err != nil {
		return fmt.Errorf("failed to update password: %w", err)
	}
	return nil
}

func (s *AuthService) GetUser(accessToken string) (*services.SupabaseUser, error) {
	ctx := context.Background()

	user, err := s.client.Auth.User(ctx, accessToken)
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	return convertToSupabaseUser(user), nil
}

func (s *AuthService) UpdateUser(accessToken string, updates map[string]interface{}) (*services.SupabaseUser, error) {
	ctx := context.Background()

	user, err := s.client.Auth.UpdateUser(ctx, accessToken, updates)
	if err != nil {
		return nil, fmt.Errorf("failed to update user: %w", err)
	}

	return convertToSupabaseUser(user), nil
}

func (s *AuthService) AdminGetUser(userID string) (*services.SupabaseUser, error) {
	ctx := context.Background()

	adminUser, err := s.client.Admin.GetUser(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user as admin: %w", err)
	}

	return convertAdminUserToSupabaseUser(adminUser), nil
}

func (s *AuthService) AdminUpdateUser(userID string, updates map[string]interface{}) (*services.SupabaseUser, error) {
	ctx := context.Background()

	// Convert generic updates to AdminUserParams
	params := supabase.AdminUserParams{
		UserMetadata: make(supabase.JSONMap),
		AppMetadata:  make(supabase.JSONMap),
	}

	if email, ok := updates["email"].(string); ok {
		params.Email = email
	}
	if phone, ok := updates["phone"].(string); ok {
		params.Phone = phone
	}
	if role, ok := updates["role"].(string); ok {
		params.Role = role
	}
	if userMeta, ok := updates["user_metadata"].(map[string]interface{}); ok {
		params.UserMetadata = userMeta
	}
	if appMeta, ok := updates["app_metadata"].(map[string]interface{}); ok {
		params.AppMetadata = appMeta
	}

	adminUser, err := s.client.Admin.UpdateUser(ctx, userID, params)
	if err != nil {
		return nil, fmt.Errorf("failed to update user as admin: %w", err)
	}

	return convertAdminUserToSupabaseUser(adminUser), nil
}

func (s *AuthService) AdminDeleteUser(userID string) error {
	// Note: nedpals client doesn't seem to have AdminDeleteUser method
	// This would need to be implemented via direct HTTP call or a different approach
	return fmt.Errorf("admin delete user not implemented in nedpals client")
}

// Helper function to convert nedpals User to our domain model
func convertToSupabaseUser(user *supabase.User) *services.SupabaseUser {
	if user == nil {
		return nil
	}

	// Parse the user ID from string to UUID
	userID, err := uuid.Parse(user.ID)
	if err != nil {
		return nil
	}

	return &services.SupabaseUser{
		ID:               userID,
		Email:            user.Email,
		EmailConfirmedAt: timePointerFromTime(user.ConfirmedAt),
		Phone:            "", // nedpals User doesn't have Phone field
		PhoneConfirmedAt: nil,
		UserMetadata:     user.UserMetadata,
		AppMetadata:      convertAppMetadata(user.AppMetadata),
		CreatedAt:        user.CreatedAt,
		UpdatedAt:        user.UpdatedAt,
		LastSignInAt:     nil, // nedpals User doesn't have LastSignInAt
	}
}

// Helper function to convert nedpals AdminUser to our domain model
func convertAdminUserToSupabaseUser(adminUser *supabase.AdminUser) *services.SupabaseUser {
	if adminUser == nil {
		return nil
	}

	// Parse the user ID from string to UUID
	userID, err := uuid.Parse(adminUser.ID)
	if err != nil {
		return nil
	}

	return &services.SupabaseUser{
		ID:               userID,
		Email:            adminUser.Email,
		EmailConfirmedAt: adminUser.EmailConfirmedAt,
		Phone:            adminUser.Phone,
		PhoneConfirmedAt: adminUser.PhoneConfirmedAt,
		UserMetadata:     map[string]interface{}(adminUser.UserMetaData),
		AppMetadata:      map[string]interface{}(adminUser.AppMetaData),
		CreatedAt:        adminUser.CreatedAt,
		UpdatedAt:        adminUser.UpdatedAt,
		LastSignInAt:     adminUser.LastSignInAt,
	}
}

// Helper functions
func timePointerFromTime(t time.Time) *time.Time {
	if t.IsZero() {
		return nil
	}
	return &t
}

func convertAppMetadata(appMeta interface{}) map[string]interface{} {
	if appMeta == nil {
		return make(map[string]interface{})
	}
	if meta, ok := appMeta.(map[string]interface{}); ok {
		return meta
	}
	return make(map[string]interface{})
}
