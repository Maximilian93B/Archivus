package postgresql

import (
	"context"
	"testing"

	"github.com/archivus/archivus/internal/infrastructure/database/models"
	"github.com/archivus/archivus/internal/infrastructure/repositories/postgresql/testutil"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTenantRepository_Create(t *testing.T) {
	db := testutil.NewTestDB(t)
	defer db.Cleanup(t)

	repo := NewTenantRepository(db.DB)
	ctx := context.Background()

	tenant := &models.Tenant{
		Name:      "Test Tenant",
		Subdomain: "test-tenant",
		IsActive:  true,
	}

	err := repo.Create(ctx, tenant)
	require.NoError(t, err)
	assert.NotEqual(t, uuid.Nil, tenant.ID)
	assert.NotZero(t, tenant.CreatedAt)
}

func TestTenantRepository_Create_DuplicateSubdomain(t *testing.T) {
	db := testutil.NewTestDB(t)
	defer db.Cleanup(t)

	repo := NewTenantRepository(db.DB)
	ctx := context.Background()

	// Create first tenant
	tenant1 := &models.Tenant{
		Name:      "Test Tenant 1",
		Subdomain: "duplicate",
		IsActive:  true,
	}
	err := repo.Create(ctx, tenant1)
	require.NoError(t, err)

	// Try to create second tenant with same subdomain
	tenant2 := &models.Tenant{
		Name:      "Test Tenant 2",
		Subdomain: "duplicate",
		IsActive:  true,
	}
	err = repo.Create(ctx, tenant2)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already exists")
}

func TestTenantRepository_GetByID(t *testing.T) {
	db := testutil.NewTestDB(t)
	defer db.Cleanup(t)

	repo := NewTenantRepository(db.DB)
	ctx := context.Background()

	// Create tenant
	original := db.CreateTestTenant(t)

	// Get by ID
	found, err := repo.GetByID(ctx, original.ID)
	require.NoError(t, err)
	assert.Equal(t, original.ID, found.ID)
	assert.Equal(t, original.Name, found.Name)
	assert.Equal(t, original.Subdomain, found.Subdomain)
}

func TestTenantRepository_GetByID_NotFound(t *testing.T) {
	db := testutil.NewTestDB(t)
	defer db.Cleanup(t)

	repo := NewTenantRepository(db.DB)
	ctx := context.Background()

	_, err := repo.GetByID(ctx, uuid.New())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestTenantRepository_GetBySubdomain(t *testing.T) {
	db := testutil.NewTestDB(t)
	defer db.Cleanup(t)

	repo := NewTenantRepository(db.DB)
	ctx := context.Background()

	// Create tenant
	original := db.CreateTestTenant(t)

	// Get by subdomain
	found, err := repo.GetBySubdomain(ctx, original.Subdomain)
	require.NoError(t, err)
	assert.Equal(t, original.ID, found.ID)
	assert.Equal(t, original.Subdomain, found.Subdomain)
}

func TestTenantRepository_UpdateUsage(t *testing.T) {
	db := testutil.NewTestDB(t)
	defer db.Cleanup(t)

	repo := NewTenantRepository(db.DB)
	ctx := context.Background()

	// Create tenant
	tenant := db.CreateTestTenant(t)

	// Update usage
	err := repo.UpdateUsage(ctx, tenant.ID, 1000, 5)
	require.NoError(t, err)

	// Verify update
	updated, err := repo.GetByID(ctx, tenant.ID)
	require.NoError(t, err)
	assert.Equal(t, int64(1000), updated.StorageUsed)
	assert.Equal(t, 5, updated.APIUsed)
}

func TestTenantRepository_CheckQuotaLimits(t *testing.T) {
	db := testutil.NewTestDB(t)
	defer db.Cleanup(t)

	repo := NewTenantRepository(db.DB)
	ctx := context.Background()

	// Create tenant with specific quotas
	tenant := &models.Tenant{
		Name:         "Test Tenant",
		Subdomain:    "quota-test",
		StorageQuota: 10000,
		StorageUsed:  5000,
		APIQuota:     100,
		APIUsed:      50,
		IsActive:     true,
	}
	err := repo.Create(ctx, tenant)
	require.NoError(t, err)

	// Check quota
	quota, err := repo.CheckQuotaLimits(ctx, tenant.ID)
	require.NoError(t, err)
	assert.Equal(t, int64(5000), quota.StorageUsed)
	assert.Equal(t, int64(10000), quota.StorageQuota)
	assert.Equal(t, 50.0, quota.StoragePercent)
	assert.Equal(t, 50, quota.APIUsed)
	assert.Equal(t, 100, quota.APIQuota)
	assert.Equal(t, 50.0, quota.APIPercent)
	assert.True(t, quota.CanUpload)
	assert.True(t, quota.CanProcessAI)
}
