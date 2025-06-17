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

func TestTagRepository_Create(t *testing.T) {
	db := testutil.NewTestDB(t)
	defer db.Cleanup(t)

	repo := NewTagRepository(db.DB)
	ctx := context.Background()

	tenant := db.CreateTestTenant(t)

	tag := &models.Tag{
		TenantID:      tenant.ID,
		Name:          "Test Tag",
		Color:         "#FF5733",
		IsAIGenerated: false,
		UsageCount:    0,
	}

	err := repo.Create(ctx, tag)
	require.NoError(t, err)
	assert.NotEqual(t, uuid.Nil, tag.ID)
	assert.NotZero(t, tag.CreatedAt)
}

func TestTagRepository_Create_DuplicateName(t *testing.T) {
	db := testutil.NewTestDB(t)
	defer db.Cleanup(t)

	repo := NewTagRepository(db.DB)
	ctx := context.Background()

	tenant := db.CreateTestTenant(t)

	// Create first tag
	tag1 := &models.Tag{
		TenantID:      tenant.ID,
		Name:          "Duplicate Tag",
		Color:         "#FF5733",
		IsAIGenerated: false,
		UsageCount:    0,
	}
	err := repo.Create(ctx, tag1)
	require.NoError(t, err)

	// Try to create second tag with same name in same tenant
	tag2 := &models.Tag{
		TenantID:      tenant.ID,
		Name:          "Duplicate Tag", // Same name
		Color:         "#00FF00",
		IsAIGenerated: false,
		UsageCount:    0,
	}
	err = repo.Create(ctx, tag2)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already exists")
}

func TestTagRepository_Create_SameNameDifferentTenant(t *testing.T) {
	db := testutil.NewTestDB(t)
	defer db.Cleanup(t)

	repo := NewTagRepository(db.DB)
	ctx := context.Background()

	tenant1 := db.CreateTestTenant(t)
	tenant2 := db.CreateTestTenant(t)

	// Create tag in tenant1
	tag1 := &models.Tag{
		TenantID:      tenant1.ID,
		Name:          "Same Name",
		Color:         "#FF5733",
		IsAIGenerated: false,
		UsageCount:    0,
	}
	err := repo.Create(ctx, tag1)
	require.NoError(t, err)

	// Create tag with same name in tenant2 - should succeed
	tag2 := &models.Tag{
		TenantID:      tenant2.ID,
		Name:          "Same Name", // Same name but different tenant
		Color:         "#00FF00",
		IsAIGenerated: false,
		UsageCount:    0,
	}
	err = repo.Create(ctx, tag2)
	require.NoError(t, err)
	assert.NotEqual(t, tag1.ID, tag2.ID)
}

func TestTagRepository_GetByID(t *testing.T) {
	db := testutil.NewTestDB(t)
	defer db.Cleanup(t)

	repo := NewTagRepository(db.DB)
	ctx := context.Background()

	tenant := db.CreateTestTenant(t)

	// Create tag
	original := &models.Tag{
		TenantID:      tenant.ID,
		Name:          "Test Tag",
		Color:         "#FF5733",
		IsAIGenerated: true,
		UsageCount:    5,
	}
	err := repo.Create(ctx, original)
	require.NoError(t, err)

	// Get by ID
	found, err := repo.GetByID(ctx, original.ID)
	require.NoError(t, err)
	assert.Equal(t, original.ID, found.ID)
	assert.Equal(t, original.Name, found.Name)
	assert.Equal(t, original.Color, found.Color)
	assert.Equal(t, original.IsAIGenerated, found.IsAIGenerated)
	assert.Equal(t, original.UsageCount, found.UsageCount)
	assert.Equal(t, original.TenantID, found.TenantID)
}

func TestTagRepository_GetByID_NotFound(t *testing.T) {
	db := testutil.NewTestDB(t)
	defer db.Cleanup(t)

	repo := NewTagRepository(db.DB)
	ctx := context.Background()

	// Try to get non-existent tag
	_, err := repo.GetByID(ctx, uuid.New())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestTagRepository_GetByName(t *testing.T) {
	db := testutil.NewTestDB(t)
	defer db.Cleanup(t)

	repo := NewTagRepository(db.DB)
	ctx := context.Background()

	tenant := db.CreateTestTenant(t)

	// Create tag
	tag := &models.Tag{
		TenantID:      tenant.ID,
		Name:          "Searchable Tag",
		Color:         "#FF5733",
		IsAIGenerated: false,
		UsageCount:    0,
	}
	err := repo.Create(ctx, tag)
	require.NoError(t, err)

	// Get by name
	found, err := repo.GetByName(ctx, tenant.ID, "Searchable Tag")
	require.NoError(t, err)
	assert.Equal(t, tag.ID, found.ID)
	assert.Equal(t, tag.Name, found.Name)
}

func TestTagRepository_GetByName_DifferentTenant(t *testing.T) {
	db := testutil.NewTestDB(t)
	defer db.Cleanup(t)

	repo := NewTagRepository(db.DB)
	ctx := context.Background()

	tenant1 := db.CreateTestTenant(t)
	tenant2 := db.CreateTestTenant(t)

	// Create tag in tenant1
	tag := &models.Tag{
		TenantID:      tenant1.ID,
		Name:          "Cross Tenant Tag",
		Color:         "#FF5733",
		IsAIGenerated: false,
		UsageCount:    0,
	}
	err := repo.Create(ctx, tag)
	require.NoError(t, err)

	// Try to get tag from tenant2
	_, err = repo.GetByName(ctx, tenant2.ID, "Cross Tenant Tag")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestTagRepository_Update(t *testing.T) {
	db := testutil.NewTestDB(t)
	defer db.Cleanup(t)

	repo := NewTagRepository(db.DB)
	ctx := context.Background()

	tenant := db.CreateTestTenant(t)

	// Create tag
	tag := &models.Tag{
		TenantID:      tenant.ID,
		Name:          "Original Name",
		Color:         "#FF5733",
		IsAIGenerated: false,
		UsageCount:    0,
	}
	err := repo.Create(ctx, tag)
	require.NoError(t, err)

	// Update tag
	tag.Name = "Updated Name"
	tag.Color = "#00FF00"
	tag.IsAIGenerated = true
	tag.UsageCount = 10

	err = repo.Update(ctx, tag)
	require.NoError(t, err)

	// Verify update
	found, err := repo.GetByID(ctx, tag.ID)
	require.NoError(t, err)
	assert.Equal(t, "Updated Name", found.Name)
	assert.Equal(t, "#00FF00", found.Color)
	assert.Equal(t, true, found.IsAIGenerated)
	assert.Equal(t, 10, found.UsageCount)
}

func TestTagRepository_Update_DuplicateName(t *testing.T) {
	db := testutil.NewTestDB(t)
	defer db.Cleanup(t)

	repo := NewTagRepository(db.DB)
	ctx := context.Background()

	tenant := db.CreateTestTenant(t)

	// Create two tags
	tag1 := &models.Tag{
		TenantID:      tenant.ID,
		Name:          "Tag 1",
		Color:         "#FF5733",
		IsAIGenerated: false,
		UsageCount:    0,
	}
	err := repo.Create(ctx, tag1)
	require.NoError(t, err)

	tag2 := &models.Tag{
		TenantID:      tenant.ID,
		Name:          "Tag 2",
		Color:         "#00FF00",
		IsAIGenerated: false,
		UsageCount:    0,
	}
	err = repo.Create(ctx, tag2)
	require.NoError(t, err)

	// Try to update tag2 to have same name as tag1
	tag2.Name = "Tag 1"
	err = repo.Update(ctx, tag2)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already exists")
}

func TestTagRepository_ListByTenant(t *testing.T) {
	db := testutil.NewTestDB(t)
	defer db.Cleanup(t)

	repo := NewTagRepository(db.DB)
	ctx := context.Background()

	tenant1 := db.CreateTestTenant(t)
	tenant2 := db.CreateTestTenant(t)

	// Create tags in different tenants with different usage counts
	tag1 := &models.Tag{
		TenantID:      tenant1.ID,
		Name:          "High Usage Tag",
		Color:         "#FF5733",
		IsAIGenerated: false,
		UsageCount:    10,
	}
	err := repo.Create(ctx, tag1)
	require.NoError(t, err)

	tag2 := &models.Tag{
		TenantID:      tenant1.ID,
		Name:          "Low Usage Tag",
		Color:         "#00FF00",
		IsAIGenerated: false,
		UsageCount:    2,
	}
	err = repo.Create(ctx, tag2)
	require.NoError(t, err)

	tag3 := &models.Tag{
		TenantID:      tenant2.ID,
		Name:          "Other Tenant Tag",
		Color:         "#0000FF",
		IsAIGenerated: false,
		UsageCount:    5,
	}
	err = repo.Create(ctx, tag3)
	require.NoError(t, err)

	// List tags for tenant1
	tags1, err := repo.ListByTenant(ctx, tenant1.ID)
	require.NoError(t, err)
	assert.Len(t, tags1, 2)

	// Verify tags are sorted by usage_count DESC, name ASC
	assert.Equal(t, tag1.ID, tags1[0].ID) // Higher usage count first
	assert.Equal(t, tag2.ID, tags1[1].ID)

	// Verify all tags belong to tenant1
	for _, tag := range tags1 {
		assert.Equal(t, tenant1.ID, tag.TenantID)
	}

	// List tags for tenant2
	tags2, err := repo.ListByTenant(ctx, tenant2.ID)
	require.NoError(t, err)
	assert.Len(t, tags2, 1)
	assert.Equal(t, tag3.ID, tags2[0].ID)
}

func TestTagRepository_GetPopular(t *testing.T) {
	db := testutil.NewTestDB(t)
	defer db.Cleanup(t)

	repo := NewTagRepository(db.DB)
	ctx := context.Background()

	tenant := db.CreateTestTenant(t)

	// Create tags with different usage counts
	tags := []*models.Tag{
		{
			TenantID:      tenant.ID,
			Name:          "Most Popular",
			Color:         "#FF5733",
			IsAIGenerated: false,
			UsageCount:    100,
		},
		{
			TenantID:      tenant.ID,
			Name:          "Second Popular",
			Color:         "#00FF00",
			IsAIGenerated: false,
			UsageCount:    50,
		},
		{
			TenantID:      tenant.ID,
			Name:          "Third Popular",
			Color:         "#0000FF",
			IsAIGenerated: false,
			UsageCount:    25,
		},
		{
			TenantID:      tenant.ID,
			Name:          "Least Popular",
			Color:         "#FFFF00",
			IsAIGenerated: false,
			UsageCount:    1,
		},
	}

	for _, tag := range tags {
		err := repo.Create(ctx, tag)
		require.NoError(t, err)
	}

	// Get top 3 popular tags
	popular, err := repo.GetPopular(ctx, tenant.ID, 3)
	require.NoError(t, err)
	assert.Len(t, popular, 3)

	// Verify correct order (highest usage first)
	assert.Equal(t, "Most Popular", popular[0].Name)
	assert.Equal(t, 100, popular[0].UsageCount)
	assert.Equal(t, "Second Popular", popular[1].Name)
	assert.Equal(t, 50, popular[1].UsageCount)
	assert.Equal(t, "Third Popular", popular[2].Name)
	assert.Equal(t, 25, popular[2].UsageCount)
}

func TestTagRepository_IncrementUsage(t *testing.T) {
	db := testutil.NewTestDB(t)
	defer db.Cleanup(t)

	repo := NewTagRepository(db.DB)
	ctx := context.Background()

	tenant := db.CreateTestTenant(t)

	// Create tag
	tag := &models.Tag{
		TenantID:      tenant.ID,
		Name:          "Usage Test Tag",
		Color:         "#FF5733",
		IsAIGenerated: false,
		UsageCount:    5,
	}
	err := repo.Create(ctx, tag)
	require.NoError(t, err)

	// Increment usage
	err = repo.IncrementUsage(ctx, tag.ID)
	require.NoError(t, err)

	// Verify increment
	found, err := repo.GetByID(ctx, tag.ID)
	require.NoError(t, err)
	assert.Equal(t, 6, found.UsageCount)

	// Increment again
	err = repo.IncrementUsage(ctx, tag.ID)
	require.NoError(t, err)

	// Verify second increment
	found, err = repo.GetByID(ctx, tag.ID)
	require.NoError(t, err)
	assert.Equal(t, 7, found.UsageCount)
}

func TestTagRepository_IncrementUsage_NotFound(t *testing.T) {
	db := testutil.NewTestDB(t)
	defer db.Cleanup(t)

	repo := NewTagRepository(db.DB)
	ctx := context.Background()

	// Try to increment usage of non-existent tag
	err := repo.IncrementUsage(ctx, uuid.New())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestTagRepository_BulkCreate(t *testing.T) {
	db := testutil.NewTestDB(t)
	defer db.Cleanup(t)

	repo := NewTagRepository(db.DB)
	ctx := context.Background()

	tenant := db.CreateTestTenant(t)

	// Create tags for bulk insert
	tags := []models.Tag{
		{
			TenantID:      tenant.ID,
			Name:          "Bulk Tag 1",
			Color:         "#FF5733",
			IsAIGenerated: false,
			UsageCount:    0,
		},
		{
			TenantID:      tenant.ID,
			Name:          "Bulk Tag 2",
			Color:         "#00FF00",
			IsAIGenerated: true,
			UsageCount:    0,
		},
		{
			TenantID:      tenant.ID,
			Name:          "Bulk Tag 3",
			Color:         "#0000FF",
			IsAIGenerated: false,
			UsageCount:    0,
		},
	}

	// Bulk create
	err := repo.BulkCreate(ctx, tags)
	require.NoError(t, err)

	// Verify all tags were created
	allTags, err := repo.ListByTenant(ctx, tenant.ID)
	require.NoError(t, err)
	assert.Len(t, allTags, 3)

	// Verify each tag was created with correct data
	tagNames := make(map[string]bool)
	for _, tag := range allTags {
		tagNames[tag.Name] = true
		assert.NotEqual(t, uuid.Nil, tag.ID)
		assert.Equal(t, tenant.ID, tag.TenantID)
	}

	assert.True(t, tagNames["Bulk Tag 1"])
	assert.True(t, tagNames["Bulk Tag 2"])
	assert.True(t, tagNames["Bulk Tag 3"])
}

func TestTagRepository_BulkCreate_Empty(t *testing.T) {
	db := testutil.NewTestDB(t)
	defer db.Cleanup(t)

	repo := NewTagRepository(db.DB)
	ctx := context.Background()

	// Bulk create empty slice
	err := repo.BulkCreate(ctx, []models.Tag{})
	require.NoError(t, err) // Should not error
}

func TestTagRepository_Delete(t *testing.T) {
	db := testutil.NewTestDB(t)
	defer db.Cleanup(t)

	repo := NewTagRepository(db.DB)
	ctx := context.Background()

	tenant := db.CreateTestTenant(t)
	user := db.CreateTestUser(t, tenant)

	// Create tag
	tag := &models.Tag{
		TenantID:      tenant.ID,
		Name:          "Delete Test Tag",
		Color:         "#FF5733",
		IsAIGenerated: false,
		UsageCount:    0,
	}
	err := repo.Create(ctx, tag)
	require.NoError(t, err)

	// Create document and associate with tag
	document := db.CreateTestDocument(t, tenant, user)

	// Create association (simulate many-to-many relationship)
	err = db.DB.Exec("INSERT INTO document_tags (document_id, tag_id) VALUES (?, ?)",
		document.ID, tag.ID).Error
	require.NoError(t, err)

	// Delete tag
	err = repo.Delete(ctx, tag.ID)
	require.NoError(t, err)

	// Verify tag is deleted
	_, err = repo.GetByID(ctx, tag.ID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")

	// Verify associations are cleaned up
	var count int64
	err = db.DB.Table("document_tags").Where("tag_id = ?", tag.ID).Count(&count).Error
	require.NoError(t, err)
	assert.Equal(t, int64(0), count)
}

func TestTagRepository_Delete_NotFound(t *testing.T) {
	db := testutil.NewTestDB(t)
	defer db.Cleanup(t)

	repo := NewTagRepository(db.DB)
	ctx := context.Background()

	// Try to delete non-existent tag
	err := repo.Delete(ctx, uuid.New())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestTagRepository_MultiTenant_Isolation(t *testing.T) {
	db := testutil.NewTestDB(t)
	defer db.Cleanup(t)

	repo := NewTagRepository(db.DB)
	ctx := context.Background()

	// Create two tenants with tags
	tenant1 := db.CreateTestTenant(t)
	tenant2 := db.CreateTestTenant(t)

	tag1 := &models.Tag{
		TenantID:      tenant1.ID,
		Name:          "Tenant 1 Tag",
		Color:         "#FF5733",
		IsAIGenerated: false,
		UsageCount:    10,
	}
	err := repo.Create(ctx, tag1)
	require.NoError(t, err)

	tag2 := &models.Tag{
		TenantID:      tenant2.ID,
		Name:          "Tenant 2 Tag",
		Color:         "#00FF00",
		IsAIGenerated: false,
		UsageCount:    20,
	}
	err = repo.Create(ctx, tag2)
	require.NoError(t, err)

	// List tags for tenant1 - should only see tag1
	tags1, err := repo.ListByTenant(ctx, tenant1.ID)
	require.NoError(t, err)
	assert.Len(t, tags1, 1)
	assert.Equal(t, tag1.ID, tags1[0].ID)

	// List tags for tenant2 - should only see tag2
	tags2, err := repo.ListByTenant(ctx, tenant2.ID)
	require.NoError(t, err)
	assert.Len(t, tags2, 1)
	assert.Equal(t, tag2.ID, tags2[0].ID)

	// Get popular tags for tenant1
	popular1, err := repo.GetPopular(ctx, tenant1.ID, 10)
	require.NoError(t, err)
	assert.Len(t, popular1, 1)
	assert.Equal(t, tag1.ID, popular1[0].ID)

	// Get popular tags for tenant2
	popular2, err := repo.GetPopular(ctx, tenant2.ID, 10)
	require.NoError(t, err)
	assert.Len(t, popular2, 1)
	assert.Equal(t, tag2.ID, popular2[0].ID)

	// Try to access tag1 from tenant2 context
	_, err = repo.GetByName(ctx, tenant2.ID, "Tenant 1 Tag")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}
