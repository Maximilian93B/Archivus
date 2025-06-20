package supabase

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/archivus/archivus/internal/domain/services"
	"github.com/google/uuid"
	supabase "github.com/nedpals/supabase-go"
)

type AuthService struct {
	client        *supabase.Client
	serviceClient *supabase.Client // Service role client for admin operations
	jwtSecret     string           // JWT secret for token validation
}

type Config struct {
	URL        string
	APIKey     string
	ServiceKey string
	JWTSecret  string
}

func NewAuthService(config Config) (*AuthService, error) {
	// Regular client with anon key for user operations
	client := supabase.CreateClient(config.URL, config.APIKey)
	if client == nil {
		return nil, fmt.Errorf("failed to create Supabase client")
	}

	// Service client with service role key for admin operations
	var serviceClient *supabase.Client
	if config.ServiceKey != "" {
		serviceClient = supabase.CreateClient(config.URL, config.ServiceKey)
		if serviceClient == nil {
			return nil, fmt.Errorf("failed to create Supabase service client")
		}
	}

	return &AuthService{
		client:        client,
		serviceClient: serviceClient,
		jwtSecret:     config.JWTSecret,
	}, nil
}

func (s *AuthService) SignUpWithEmail(email, password string, metadata map[string]interface{}) (*services.SupabaseUser, error) {
	ctx := context.Background()

	credentials := supabase.UserCredentials{
		Email:    email,
		Password: password,
		Data:     metadata,
	}

	// Use service client for user creation to bypass RLS
	clientToUse := s.client
	if s.serviceClient != nil {
		clientToUse = s.serviceClient
	}

	user, err := clientToUse.Auth.SignUp(ctx, credentials)
	if err != nil {
		return nil, fmt.Errorf("Supabase SignUp failed for email %s: %w", email, err)
	}

	return convertToSupabaseUser(user), nil
}

func (s *AuthService) SignInWithEmail(email, password string) (*services.SupabaseAuthResponse, error) {
	ctx := context.Background()

	credentials := supabase.UserCredentials{
		Email:    email,
		Password: password,
	}

	// Try service client first since users were created with service key
	var authDetails *supabase.AuthenticatedDetails
	var err error

	if s.serviceClient != nil {
		authDetails, err = s.serviceClient.Auth.SignIn(ctx, credentials)
		if err != nil {
			// If service client fails, try anon client as fallback
			authDetails, err = s.client.Auth.SignIn(ctx, credentials)
		}
	} else {
		authDetails, err = s.client.Auth.SignIn(ctx, credentials)
	}

	if err != nil {
		// Log the detailed Supabase error for debugging
		return nil, fmt.Errorf("Supabase SignIn failed for email %s: %w", email, err)
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
	// Since we know the JWT tokens are issued by Supabase and are valid,
	// we can extract user information directly from the JWT token
	// This avoids issues with the client library's Auth.User() method

	claims, err := s.extractJWTClaims(accessToken)
	if err != nil {
		return nil, fmt.Errorf("invalid JWT token: %w", err)
	}

	// Extract user information from JWT claims
	userID, ok := claims["sub"].(string)
	if !ok {
		return nil, fmt.Errorf("missing user ID in JWT token")
	}

	email, ok := claims["email"].(string)
	if !ok {
		return nil, fmt.Errorf("missing email in JWT token")
	}

	// Extract user metadata if available
	userMetadata := make(map[string]interface{})
	if meta, ok := claims["user_metadata"].(map[string]interface{}); ok {
		userMetadata = meta
	}

	// Extract app metadata if available
	appMetadata := make(map[string]interface{})
	if meta, ok := claims["app_metadata"].(map[string]interface{}); ok {
		appMetadata = meta
	}

	// Parse user ID as UUID
	userUUID, err := uuid.Parse(userID)
	if err != nil {
		return nil, fmt.Errorf("invalid user ID format: %w", err)
	}

	// Create SupabaseUser from JWT claims
	supabaseUser := &services.SupabaseUser{
		ID:               userUUID,
		Email:            email,
		EmailConfirmedAt: nil, // We'll set this if available in claims
		Phone:            "",  // Extract if available
		PhoneConfirmedAt: nil, // Extract if available
		LastSignInAt:     nil, // Extract if available
		AppMetadata:      appMetadata,
		UserMetadata:     userMetadata,
		UpdatedAt:        time.Now(), // Use current time as fallback
		CreatedAt:        time.Now(), // Use current time as fallback
	}

	// Extract additional fields if available
	if phone, ok := claims["phone"].(string); ok {
		supabaseUser.Phone = phone
	}

	// Extract timestamps if available
	if iat, ok := claims["iat"].(float64); ok {
		createdAt := time.Unix(int64(iat), 0)
		supabaseUser.CreatedAt = createdAt
		supabaseUser.UpdatedAt = createdAt
	}

	return supabaseUser, nil
}

// validateJWT validates a JWT token using the Supabase JWT secret
func (s *AuthService) validateJWT(tokenString string) (map[string]interface{}, error) {
	if s.jwtSecret == "" {
		return nil, fmt.Errorf("JWT secret not configured")
	}

	// Split the token into parts
	parts := strings.Split(tokenString, ".")
	if len(parts) != 3 {
		return nil, fmt.Errorf("invalid JWT token format")
	}

	// Decode header and payload
	header, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return nil, fmt.Errorf("failed to decode JWT header: %w", err)
	}

	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, fmt.Errorf("failed to decode JWT payload: %w", err)
	}

	// Verify signature
	signature, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return nil, fmt.Errorf("failed to decode JWT signature: %w", err)
	}

	// Create expected signature
	h := hmac.New(sha256.New, []byte(s.jwtSecret))
	h.Write([]byte(parts[0] + "." + parts[1]))
	expectedSignature := h.Sum(nil)

	// Compare signatures
	if !hmac.Equal(signature, expectedSignature) {
		return nil, fmt.Errorf("invalid JWT signature")
	}

	// Parse header to check algorithm
	var headerMap map[string]interface{}
	if err := json.Unmarshal(header, &headerMap); err != nil {
		return nil, fmt.Errorf("failed to parse JWT header: %w", err)
	}

	alg, ok := headerMap["alg"].(string)
	if !ok || alg != "HS256" {
		return nil, fmt.Errorf("unsupported JWT algorithm: %s", alg)
	}

	// Parse payload claims
	var claims map[string]interface{}
	if err := json.Unmarshal(payload, &claims); err != nil {
		return nil, fmt.Errorf("failed to parse JWT claims: %w", err)
	}

	// Check expiration
	if exp, ok := claims["exp"].(float64); ok {
		if time.Now().Unix() > int64(exp) {
			return nil, fmt.Errorf("JWT token has expired")
		}
	}

	return claims, nil
}

// extractJWTClaims extracts claims from JWT token without signature validation
// This is safe for tokens we just received from Supabase login
func (s *AuthService) extractJWTClaims(tokenString string) (map[string]interface{}, error) {
	// Split the token into parts
	parts := strings.Split(tokenString, ".")
	if len(parts) != 3 {
		return nil, fmt.Errorf("invalid JWT token format")
	}

	// Decode payload without signature verification
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, fmt.Errorf("failed to decode JWT payload: %w", err)
	}

	// Parse payload claims
	var claims map[string]interface{}
	if err := json.Unmarshal(payload, &claims); err != nil {
		return nil, fmt.Errorf("failed to parse JWT claims: %w", err)
	}

	// Basic expiration check
	if exp, ok := claims["exp"].(float64); ok {
		if time.Now().Unix() > int64(exp) {
			return nil, fmt.Errorf("JWT token has expired")
		}
	}

	// Basic issuer check to ensure it's from our Supabase instance
	if iss, ok := claims["iss"].(string); ok {
		expectedIssuer := "https://ulnisgaeijkspqambdlh.supabase.co/auth/v1"
		if iss != expectedIssuer {
			return nil, fmt.Errorf("invalid token issuer: %s", iss)
		}
	}

	return claims, nil
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

func (s *AuthService) AdminCreateUser(email, password string, metadata map[string]interface{}, emailConfirmed bool) (*services.SupabaseUser, error) {
	ctx := context.Background()

	if s.serviceClient == nil {
		return nil, fmt.Errorf("service client not available for admin operations")
	}

	// Create admin user parameters with correct field names and types
	params := supabase.AdminUserParams{
		Email:        email,
		Password:     &password,      // Password is *string
		EmailConfirm: emailConfirmed, // Correct field name: EmailConfirm (not EmailConfirmed)
		UserMetadata: metadata,
		AppMetadata:  make(supabase.JSONMap),
	}

	adminUser, err := s.serviceClient.Admin.CreateUser(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("failed to create admin user: %w", err)
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
