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

func TestCategoryRepository_Create(t *testing.T) {
	db := testutil.NewTestDB(t)
	defer db.Cleanup(t)

	repo := NewCategoryRepository(db.DB)
	ctx := context.Background()

	tenant := db.CreateTestTenant(t)

	category := &models.Category{
		TenantID:    tenant.ID,
		Name:        "Test Category",
		Description: "A test category",
		Color:       "#FF5733",
		Icon:        "folder",
		IsSystem:    false,
		SortOrder:   10,
	}

	err := repo.Create(ctx, category)
	require.NoError(t, err)
	assert.NotEqual(t, uuid.Nil, category.ID)
	assert.NotZero(t, category.CreatedAt)
}

func TestCategoryRepository_Create_DuplicateName(t *testing.T) {
	db := testutil.NewTestDB(t)
	defer db.Cleanup(t)

	repo := NewCategoryRepository(db.DB)
	ctx := context.Background()

	tenant := db.CreateTestTenant(t)

	// Create first category
	category1 := &models.Category{
		TenantID:  tenant.ID,
		Name:      "Duplicate Category",
		Color:     "#FF5733",
		IsSystem:  false,
		SortOrder: 10,
	}
	err := repo.Create(ctx, category1)
	require.NoError(t, err)

	// Try to create second category with same name in same tenant
	category2 := &models.Category{
		TenantID:  tenant.ID,
		Name:      "Duplicate Category", // Same name
		Color:     "#00FF00",
		IsSystem:  false,
		SortOrder: 20,
	}
	err = repo.Create(ctx, category2)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already exists")
}

func TestCategoryRepository_Create_SameNameDifferentTenant(t *testing.T) {
	db := testutil.NewTestDB(t)
	defer db.Cleanup(t)

	repo := NewCategoryRepository(db.DB)
	ctx := context.Background()

	tenant1 := db.CreateTestTenant(t)
	tenant2 := db.CreateTestTenant(t)

	// Create category in tenant1
	category1 := &models.Category{
		TenantID:  tenant1.ID,
		Name:      "Same Name",
		Color:     "#FF5733",
		IsSystem:  false,
		SortOrder: 10,
	}
	err := repo.Create(ctx, category1)
	require.NoError(t, err)

	// Create category with same name in tenant2 - should succeed
	category2 := &models.Category{
		TenantID:  tenant2.ID,
		Name:      "Same Name", // Same name but different tenant
		Color:     "#00FF00",
		IsSystem:  false,
		SortOrder: 10,
	}
	err = repo.Create(ctx, category2)
	require.NoError(t, err)
	assert.NotEqual(t, category1.ID, category2.ID)
}

func TestCategoryRepository_GetByID(t *testing.T) {
	db := testutil.NewTestDB(t)
	defer db.Cleanup(t)

	repo := NewCategoryRepository(db.DB)
	ctx := context.Background()

	tenant := db.CreateTestTenant(t)

	// Create category
	original := &models.Category{
		TenantID:    tenant.ID,
		Name:        "Test Category",
		Description: "A test category",
		Color:       "#FF5733",
		Icon:        "folder",
		IsSystem:    true,
		SortOrder:   5,
	}
	err := repo.Create(ctx, original)
	require.NoError(t, err)

	// Get by ID
	found, err := repo.GetByID(ctx, original.ID)
	require.NoError(t, err)
	assert.Equal(t, original.ID, found.ID)
	assert.Equal(t, original.Name, found.Name)
	assert.Equal(t, original.Description, found.Description)
	assert.Equal(t, original.Color, found.Color)
	assert.Equal(t, original.Icon, found.Icon)
	assert.Equal(t, original.IsSystem, found.IsSystem)
	assert.Equal(t, original.SortOrder, found.SortOrder)
	assert.Equal(t, original.TenantID, found.TenantID)
}

func TestCategoryRepository_GetByID_NotFound(t *testing.T) {
	db := testutil.NewTestDB(t)
	defer db.Cleanup(t)

	repo := NewCategoryRepository(db.DB)
	ctx := context.Background()

	// Try to get non-existent category
	_, err := repo.GetByID(ctx, uuid.New())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestCategoryRepository_GetByName(t *testing.T) {
	db := testutil.NewTestDB(t)
	defer db.Cleanup(t)

	repo := NewCategoryRepository(db.DB)
	ctx := context.Background()

	tenant := db.CreateTestTenant(t)

	// Create category
	category := &models.Category{
		TenantID:  tenant.ID,
		Name:      "Searchable Category",
		Color:     "#FF5733",
		IsSystem:  false,
		SortOrder: 10,
	}
	err := repo.Create(ctx, category)
	require.NoError(t, err)

	// Get by name
	found, err := repo.GetByName(ctx, tenant.ID, "Searchable Category")
	require.NoError(t, err)
	assert.Equal(t, category.ID, found.ID)
	assert.Equal(t, category.Name, found.Name)
}

func TestCategoryRepository_GetByName_DifferentTenant(t *testing.T) {
	db := testutil.NewTestDB(t)
	defer db.Cleanup(t)

	repo := NewCategoryRepository(db.DB)
	ctx := context.Background()

	tenant1 := db.CreateTestTenant(t)
	tenant2 := db.CreateTestTenant(t)

	// Create category in tenant1
	category := &models.Category{
		TenantID:  tenant1.ID,
		Name:      "Cross Tenant Category",
		Color:     "#FF5733",
		IsSystem:  false,
		SortOrder: 10,
	}
	err := repo.Create(ctx, category)
	require.NoError(t, err)

	// Try to get category from tenant2
	_, err = repo.GetByName(ctx, tenant2.ID, "Cross Tenant Category")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestCategoryRepository_Update(t *testing.T) {
	db := testutil.NewTestDB(t)
	defer db.Cleanup(t)

	repo := NewCategoryRepository(db.DB)
	ctx := context.Background()

	tenant := db.CreateTestTenant(t)

	// Create category
	category := &models.Category{
		TenantID:    tenant.ID,
		Name:        "Original Name",
		Description: "Original description",
		Color:       "#FF5733",
		Icon:        "folder",
		IsSystem:    false,
		SortOrder:   10,
	}
	err := repo.Create(ctx, category)
	require.NoError(t, err)

	// Update category
	category.Name = "Updated Name"
	category.Description = "Updated description"
	category.Color = "#00FF00"
	category.Icon = "updated-icon"
	category.SortOrder = 20

	err = repo.Update(ctx, category)
	require.NoError(t, err)

	// Verify update
	found, err := repo.GetByID(ctx, category.ID)
	require.NoError(t, err)
	assert.Equal(t, "Updated Name", found.Name)
	assert.Equal(t, "Updated description", found.Description)
	assert.Equal(t, "#00FF00", found.Color)
	assert.Equal(t, "updated-icon", found.Icon)
	assert.Equal(t, 20, found.SortOrder)
}

func TestCategoryRepository_Update_DuplicateName(t *testing.T) {
	db := testutil.NewTestDB(t)
	defer db.Cleanup(t)

	repo := NewCategoryRepository(db.DB)
	ctx := context.Background()

	tenant := db.CreateTestTenant(t)

	// Create two categories
	category1 := &models.Category{
		TenantID:  tenant.ID,
		Name:      "Category 1",
		Color:     "#FF5733",
		IsSystem:  false,
		SortOrder: 10,
	}
	err := repo.Create(ctx, category1)
	require.NoError(t, err)

	category2 := &models.Category{
		TenantID:  tenant.ID,
		Name:      "Category 2",
		Color:     "#00FF00",
		IsSystem:  false,
		SortOrder: 20,
	}
	err = repo.Create(ctx, category2)
	require.NoError(t, err)

	// Try to update category2 to have same name as category1
	category2.Name = "Category 1"
	err = repo.Update(ctx, category2)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already exists")
}

func TestCategoryRepository_ListByTenant(t *testing.T) {
	db := testutil.NewTestDB(t)
	defer db.Cleanup(t)

	repo := NewCategoryRepository(db.DB)
	ctx := context.Background()

	tenant1 := db.CreateTestTenant(t)
	tenant2 := db.CreateTestTenant(t)

	// Create categories in different tenants with different sort orders
	category1 := &models.Category{
		TenantID:  tenant1.ID,
		Name:      "Z Category", // Alphabetically last
		Color:     "#FF5733",
		IsSystem:  false,
		SortOrder: 1, // But first in sort order
	}
	err := repo.Create(ctx, category1)
	require.NoError(t, err)

	category2 := &models.Category{
		TenantID:  tenant1.ID,
		Name:      "A Category", // Alphabetically first
		Color:     "#00FF00",
		IsSystem:  false,
		SortOrder: 2, // But second in sort order
	}
	err = repo.Create(ctx, category2)
	require.NoError(t, err)

	category3 := &models.Category{
		TenantID:  tenant2.ID,
		Name:      "Other Tenant Category",
		Color:     "#0000FF",
		IsSystem:  false,
		SortOrder: 1,
	}
	err = repo.Create(ctx, category3)
	require.NoError(t, err)

	// List categories for tenant1
	categories1, err := repo.ListByTenant(ctx, tenant1.ID)
	require.NoError(t, err)
	assert.Len(t, categories1, 2)

	// Verify categories are sorted by sort_order ASC, name ASC
	assert.Equal(t, category1.ID, categories1[0].ID) // Lower sort order first
	assert.Equal(t, category2.ID, categories1[1].ID)

	// Verify all categories belong to tenant1
	for _, category := range categories1 {
		assert.Equal(t, tenant1.ID, category.TenantID)
	}

	// List categories for tenant2
	categories2, err := repo.ListByTenant(ctx, tenant2.ID)
	require.NoError(t, err)
	assert.Len(t, categories2, 1)
	assert.Equal(t, category3.ID, categories2[0].ID)
}

func TestCategoryRepository_GetDocumentCount(t *testing.T) {
	db := testutil.NewTestDB(t)
	defer db.Cleanup(t)

	repo := NewCategoryRepository(db.DB)
	ctx := context.Background()

	tenant := db.CreateTestTenant(t)
	user := db.CreateTestUser(t, tenant)

	// Create category
	category := &models.Category{
		TenantID:  tenant.ID,
		Name:      "Test Category",
		Color:     "#FF5733",
		IsSystem:  false,
		SortOrder: 10,
	}
	err := repo.Create(ctx, category)
	require.NoError(t, err)

	// Create documents and associate with category
	doc1 := &models.Document{
		TenantID:     tenant.ID,
		FileName:     "doc1.pdf",
		OriginalName: "Document 1.pdf",
		ContentType:  "application/pdf",
		FileSize:     1024,
		StoragePath:  "/test/doc1.pdf",
		ContentHash:  "hash1",
		Title:        "Document 1",
		DocumentType: models.DocTypeGeneral,
		Status:       models.DocStatusCompleted,
		CreatedBy:    user.ID,
	}
	err = db.DB.Create(doc1).Error
	require.NoError(t, err)

	doc2 := &models.Document{
		TenantID:     tenant.ID,
		FileName:     "doc2.pdf",
		OriginalName: "Document 2.pdf",
		ContentType:  "application/pdf",
		FileSize:     2048,
		StoragePath:  "/test/doc2.pdf",
		ContentHash:  "hash2",
		Title:        "Document 2",
		DocumentType: models.DocTypeGeneral,
		Status:       models.DocStatusCompleted,
		CreatedBy:    user.ID,
	}
	err = db.DB.Create(doc2).Error
	require.NoError(t, err)

	// Associate documents with category
	err = db.DB.Exec("INSERT INTO document_categories (document_id, category_id) VALUES (?, ?)",
		doc1.ID, category.ID).Error
	require.NoError(t, err)

	err = db.DB.Exec("INSERT INTO document_categories (document_id, category_id) VALUES (?, ?)",
		doc2.ID, category.ID).Error
	require.NoError(t, err)

	// Get document count
	count, err := repo.GetDocumentCount(ctx, category.ID)
	require.NoError(t, err)
	assert.Equal(t, int64(2), count)
}

func TestCategoryRepository_GetDocumentCount_NoDocuments(t *testing.T) {
	db := testutil.NewTestDB(t)
	defer db.Cleanup(t)

	repo := NewCategoryRepository(db.DB)
	ctx := context.Background()

	tenant := db.CreateTestTenant(t)

	// Create category without documents
	category := &models.Category{
		TenantID:  tenant.ID,
		Name:      "Empty Category",
		Color:     "#FF5733",
		IsSystem:  false,
		SortOrder: 10,
	}
	err := repo.Create(ctx, category)
	require.NoError(t, err)

	// Get document count
	count, err := repo.GetDocumentCount(ctx, category.ID)
	require.NoError(t, err)
	assert.Equal(t, int64(0), count)
}

func TestCategoryRepository_Delete(t *testing.T) {
	db := testutil.NewTestDB(t)
	defer db.Cleanup(t)

	repo := NewCategoryRepository(db.DB)
	ctx := context.Background()

	tenant := db.CreateTestTenant(t)
	user := db.CreateTestUser(t, tenant)

	// Create category
	category := &models.Category{
		TenantID:  tenant.ID,
		Name:      "Delete Test Category",
		Color:     "#FF5733",
		IsSystem:  false,
		SortOrder: 10,
	}
	err := repo.Create(ctx, category)
	require.NoError(t, err)

	// Create document and associate with category
	document := db.CreateTestDocument(t, tenant, user)

	// Create association (simulate many-to-many relationship)
	err = db.DB.Exec("INSERT INTO document_categories (document_id, category_id) VALUES (?, ?)",
		document.ID, category.ID).Error
	require.NoError(t, err)

	// Delete category
	err = repo.Delete(ctx, category.ID)
	require.NoError(t, err)

	// Verify category is deleted
	_, err = repo.GetByID(ctx, category.ID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")

	// Verify associations are cleaned up
	var count int64
	err = db.DB.Table("document_categories").Where("category_id = ?", category.ID).Count(&count).Error
	require.NoError(t, err)
	assert.Equal(t, int64(0), count)
}

func TestCategoryRepository_Delete_SystemCategory(t *testing.T) {
	db := testutil.NewTestDB(t)
	defer db.Cleanup(t)

	repo := NewCategoryRepository(db.DB)
	ctx := context.Background()

	tenant := db.CreateTestTenant(t)

	// Create system category
	systemCategory := &models.Category{
		TenantID:  tenant.ID,
		Name:      "System Category",
		Color:     "#FF5733",
		IsSystem:  true, // System category
		SortOrder: 10,
	}
	err := repo.Create(ctx, systemCategory)
	require.NoError(t, err)

	// Try to delete system category
	err = repo.Delete(ctx, systemCategory.ID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot delete system category")

	// Verify system category still exists
	found, err := repo.GetByID(ctx, systemCategory.ID)
	require.NoError(t, err)
	assert.Equal(t, systemCategory.ID, found.ID)
}

func TestCategoryRepository_Delete_NotFound(t *testing.T) {
	db := testutil.NewTestDB(t)
	defer db.Cleanup(t)

	repo := NewCategoryRepository(db.DB)
	ctx := context.Background()

	// Try to delete non-existent category
	err := repo.Delete(ctx, uuid.New())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestCategoryRepository_SortOrder_Handling(t *testing.T) {
	db := testutil.NewTestDB(t)
	defer db.Cleanup(t)

	repo := NewCategoryRepository(db.DB)
	ctx := context.Background()

	tenant := db.CreateTestTenant(t)

	// Create categories with same sort order but different names
	category1 := &models.Category{
		TenantID:  tenant.ID,
		Name:      "Z Category",
		Color:     "#FF5733",
		IsSystem:  false,
		SortOrder: 10, // Same sort order
	}
	err := repo.Create(ctx, category1)
	require.NoError(t, err)

	category2 := &models.Category{
		TenantID:  tenant.ID,
		Name:      "A Category",
		Color:     "#00FF00",
		IsSystem:  false,
		SortOrder: 10, // Same sort order
	}
	err = repo.Create(ctx, category2)
	require.NoError(t, err)

	// List categories
	categories, err := repo.ListByTenant(ctx, tenant.ID)
	require.NoError(t, err)
	assert.Len(t, categories, 2)

	// When sort order is same, should be sorted by name ASC
	assert.Equal(t, "A Category", categories[0].Name)
	assert.Equal(t, "Z Category", categories[1].Name)
}

func TestCategoryRepository_MultiTenant_Isolation(t *testing.T) {
	db := testutil.NewTestDB(t)
	defer db.Cleanup(t)

	repo := NewCategoryRepository(db.DB)
	ctx := context.Background()

	// Create two tenants with categories
	tenant1 := db.CreateTestTenant(t)
	tenant2 := db.CreateTestTenant(t)

	category1 := &models.Category{
		TenantID:  tenant1.ID,
		Name:      "Tenant 1 Category",
		Color:     "#FF5733",
		IsSystem:  false,
		SortOrder: 10,
	}
	err := repo.Create(ctx, category1)
	require.NoError(t, err)

	category2 := &models.Category{
		TenantID:  tenant2.ID,
		Name:      "Tenant 2 Category",
		Color:     "#00FF00",
		IsSystem:  false,
		SortOrder: 10,
	}
	err = repo.Create(ctx, category2)
	require.NoError(t, err)

	// List categories for tenant1 - should only see category1
	categories1, err := repo.ListByTenant(ctx, tenant1.ID)
	require.NoError(t, err)
	assert.Len(t, categories1, 1)
	assert.Equal(t, category1.ID, categories1[0].ID)

	// List categories for tenant2 - should only see category2
	categories2, err := repo.ListByTenant(ctx, tenant2.ID)
	require.NoError(t, err)
	assert.Len(t, categories2, 1)
	assert.Equal(t, category2.ID, categories2[0].ID)

	// Try to access category1 from tenant2 context
	_, err = repo.GetByName(ctx, tenant2.ID, "Tenant 1 Category")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")

	// Verify document count isolation
	count1, err := repo.GetDocumentCount(ctx, category1.ID)
	require.NoError(t, err)
	assert.Equal(t, int64(0), count1)

	count2, err := repo.GetDocumentCount(ctx, category2.ID)
	require.NoError(t, err)
	assert.Equal(t, int64(0), count2)
}

func TestCategoryRepository_SystemCategories_Special_Handling(t *testing.T) {
	db := testutil.NewTestDB(t)
	defer db.Cleanup(t)

	repo := NewCategoryRepository(db.DB)
	ctx := context.Background()

	tenant := db.CreateTestTenant(t)

	// Create mix of system and user categories
	systemCategory := &models.Category{
		TenantID:  tenant.ID,
		Name:      "System Category",
		Color:     "#FF5733",
		IsSystem:  true,
		SortOrder: 1,
	}
	err := repo.Create(ctx, systemCategory)
	require.NoError(t, err)

	userCategory := &models.Category{
		TenantID:  tenant.ID,
		Name:      "User Category",
		Color:     "#00FF00",
		IsSystem:  false,
		SortOrder: 2,
	}
	err = repo.Create(ctx, userCategory)
	require.NoError(t, err)

	// List all categories
	categories, err := repo.ListByTenant(ctx, tenant.ID)
	require.NoError(t, err)
	assert.Len(t, categories, 2)

	// Verify both types are returned in correct order
	assert.Equal(t, systemCategory.ID, categories[0].ID)
	assert.True(t, categories[0].IsSystem)
	assert.Equal(t, userCategory.ID, categories[1].ID)
	assert.False(t, categories[1].IsSystem)

	// Verify system category protection
	err = repo.Delete(ctx, systemCategory.ID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot delete system category")

	// Verify user category can be deleted
	err = repo.Delete(ctx, userCategory.ID)
	require.NoError(t, err)
}
