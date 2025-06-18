package postgresql

import (
	"context"
	"testing"

	"github.com/archivus/archivus/internal/domain/repositories"
	"github.com/archivus/archivus/internal/infrastructure/database/models"
	"github.com/archivus/archivus/internal/infrastructure/repositories/postgresql/testutil"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUserRepository_Create(t *testing.T) {
	db := testutil.NewTestDB(t)
	defer db.Cleanup(t)

	repo := NewUserRepository(db.DB)
	ctx := context.Background()

	tenant := db.CreateTestTenant(t)

	user := &models.User{
		TenantID:     tenant.ID,
		Email:        "test@example.com",
		PasswordHash: "hashedpassword",
		FirstName:    "John",
		LastName:     "Doe",
		Role:         models.UserRoleUser,
		IsActive:     true,
	}

	err := repo.Create(ctx, user)
	require.NoError(t, err)
	assert.NotEqual(t, uuid.Nil, user.ID)
	assert.NotZero(t, user.CreatedAt)
}

func TestUserRepository_Create_DuplicateEmail(t *testing.T) {
	db := testutil.NewTestDB(t)
	defer db.Cleanup(t)

	repo := NewUserRepository(db.DB)
	ctx := context.Background()

	tenant := db.CreateTestTenant(t)

	// Create first user
	user1 := &models.User{
		TenantID:     tenant.ID,
		Email:        "duplicate@example.com",
		PasswordHash: "hashedpassword",
		FirstName:    "John",
		LastName:     "Doe",
		Role:         models.UserRoleUser,
		IsActive:     true,
	}
	err := repo.Create(ctx, user1)
	require.NoError(t, err)

	// Try to create second user with same email in same tenant
	user2 := &models.User{
		TenantID:     tenant.ID,
		Email:        "duplicate@example.com",
		PasswordHash: "hashedpassword",
		FirstName:    "Jane",
		LastName:     "Doe",
		Role:         models.UserRoleUser,
		IsActive:     true,
	}
	err = repo.Create(ctx, user2)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already exists")
}

func TestUserRepository_GetByID(t *testing.T) {
	db := testutil.NewTestDB(t)
	defer db.Cleanup(t)

	repo := NewUserRepository(db.DB)
	ctx := context.Background()

	// Create user
	original := db.CreateTestUser(t, db.CreateTestTenant(t))

	// Get by ID
	found, err := repo.GetByID(ctx, original.ID)
	require.NoError(t, err)
	assert.Equal(t, original.ID, found.ID)
	assert.Equal(t, original.TenantID, found.TenantID)
	assert.Equal(t, original.Email, found.Email)
	assert.Equal(t, original.FirstName, found.FirstName)
	assert.Equal(t, original.LastName, found.LastName)
	assert.Equal(t, original.Role, found.Role)
	assert.Equal(t, original.IsActive, found.IsActive)
}

func TestUserRepository_GetByEmail(t *testing.T) {
	db := testutil.NewTestDB(t)
	defer db.Cleanup(t)

	repo := NewUserRepository(db.DB)
	ctx := context.Background()

	tenant := db.CreateTestTenant(t)
	user := db.CreateTestUser(t, tenant)

	// Get by email
	found, err := repo.GetByEmail(ctx, tenant.ID, user.Email)
	require.NoError(t, err)
	assert.Equal(t, user.ID, found.ID)
	assert.Equal(t, user.Email, found.Email)
	assert.Equal(t, user.TenantID, found.TenantID)
}

func TestUserRepository_GetByEmail_DifferentTenant(t *testing.T) {
	db := testutil.NewTestDB(t)
	defer db.Cleanup(t)

	repo := NewUserRepository(db.DB)
	ctx := context.Background()

	tenant1 := db.CreateTestTenant(t)
	tenant2 := db.CreateTestTenant(t)
	user := db.CreateTestUser(t, tenant1)

	// Try to get user from different tenant
	_, err := repo.GetByEmail(ctx, tenant2.ID, user.Email)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestUserRepository_Update(t *testing.T) {
	db := testutil.NewTestDB(t)
	defer db.Cleanup(t)

	repo := NewUserRepository(db.DB)
	ctx := context.Background()

	// Create user
	original := db.CreateTestUser(t, db.CreateTestTenant(t))

	// Update user
	updated := &models.User{
		ID:           original.ID,
		TenantID:     original.TenantID,
		Email:        "updated@example.com",
		PasswordHash: "updatedhashedpassword",
		FirstName:    "Updated",
		LastName:     "User",
		Role:         models.UserRoleAdmin,
		IsActive:     false,
	}
	err := repo.Update(ctx, updated)
	require.NoError(t, err)

	// Verify update
	found, err := repo.GetByID(ctx, original.ID)
	require.NoError(t, err)
	assert.Equal(t, updated.ID, found.ID)
	assert.Equal(t, updated.TenantID, found.TenantID)
	assert.Equal(t, updated.Email, found.Email)
	assert.Equal(t, updated.PasswordHash, found.PasswordHash)
	assert.Equal(t, updated.FirstName, found.FirstName)
	assert.Equal(t, updated.LastName, found.LastName)
	assert.Equal(t, updated.Role, found.Role)
	assert.Equal(t, updated.IsActive, found.IsActive)
}

func TestUserRepository_UpdateLastLogin(t *testing.T) {
	db := testutil.NewTestDB(t)
	defer db.Cleanup(t)

	repo := NewUserRepository(db.DB)
	ctx := context.Background()

	tenant := db.CreateTestTenant(t)
	user := db.CreateTestUser(t, tenant)

	// Update last login
	err := repo.UpdateLastLogin(ctx, user.ID)
	require.NoError(t, err)

	// Verify update
	updated, err := repo.GetByID(ctx, user.ID)
	require.NoError(t, err)
	assert.NotNil(t, updated.LastLoginAt)
}

func TestUserRepository_ListByTenant(t *testing.T) {
	db := testutil.NewTestDB(t)
	defer db.Cleanup(t)

	repo := NewUserRepository(db.DB)
	ctx := context.Background()

	tenant := db.CreateTestTenant(t)

	// Create multiple users
	_ = db.CreateTestUser(t, tenant)
	_ = db.CreateTestUser(t, tenant)

	params := repositories.ListParams{
		Page:     1,
		PageSize: 10,
	}

	users, total, err := repo.ListByTenant(ctx, tenant.ID, params)
	require.NoError(t, err)
	assert.Equal(t, int64(2), total)
	assert.Len(t, users, 2)

	// Verify users are from correct tenant
	for _, user := range users {
		assert.Equal(t, tenant.ID, user.TenantID)
	}
}

func TestUserRepository_SetMFA(t *testing.T) {
	db := testutil.NewTestDB(t)
	defer db.Cleanup(t)

	repo := NewUserRepository(db.DB)
	ctx := context.Background()

	tenant := db.CreateTestTenant(t)
	user := db.CreateTestUser(t, tenant)

	// Enable MFA
	secret := "MFASECRET123"
	err := repo.SetMFA(ctx, user.ID, true, secret)
	require.NoError(t, err)

	// Verify MFA is enabled
	updated, err := repo.GetByID(ctx, user.ID)
	require.NoError(t, err)
	assert.True(t, updated.MFAEnabled) // Fixed: was MFADisabled
	assert.Equal(t, secret, updated.MFASecret)
}

func TestUserRepository_Delete(t *testing.T) {
	db := testutil.NewTestDB(t)
	defer db.Cleanup(t)

	repo := NewUserRepository(db.DB)
	ctx := context.Background()

	// Create user
	original := db.CreateTestUser(t, db.CreateTestTenant(t))

	// Delete user
	err := repo.Delete(ctx, original.ID)
	require.NoError(t, err)

	// Verify deletion
	_, err = repo.GetByID(ctx, original.ID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}
