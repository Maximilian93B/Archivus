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

func TestDocumentRepository_Create(t *testing.T) {
	db := testutil.NewTestDB(t)
	defer db.Cleanup(t)

	repo := NewDocumentRepository(db.DB)
	ctx := context.Background()

	tenant := db.CreateTestTenant(t)
	user := db.CreateTestUser(t, tenant)

	document := &models.Document{
		TenantID:     tenant.ID,
		FileName:     "test-document.pdf",
		OriginalName: "Test Document.pdf",
		ContentType:  "application/pdf",
		FileSize:     1024,
		StoragePath:  "/test/documents/test-document.pdf",
		ContentHash:  "abcdef123456789",
		Title:        "Test Document",
		DocumentType: models.DocTypeGeneral,
		Status:       models.DocStatusPending,
		CreatedBy:    user.ID,
	}

	err := repo.Create(ctx, document)
	require.NoError(t, err)
	assert.NotEqual(t, uuid.Nil, document.ID)
	assert.NotZero(t, document.CreatedAt)
}

func TestDocumentRepository_Create_DuplicateContentHash(t *testing.T) {
	db := testutil.NewTestDB(t)
	defer db.Cleanup(t)

	repo := NewDocumentRepository(db.DB)
	ctx := context.Background()

	tenant := db.CreateTestTenant(t)
	user := db.CreateTestUser(t, tenant)

	// Create first document
	doc1 := &models.Document{
		TenantID:     tenant.ID,
		FileName:     "test-doc-1.pdf",
		OriginalName: "Test Document 1.pdf",
		ContentType:  "application/pdf",
		FileSize:     1024,
		StoragePath:  "/test/documents/test-doc-1.pdf",
		ContentHash:  "samehash123456789",
		Title:        "Test Document 1",
		DocumentType: models.DocTypeGeneral,
		Status:       models.DocStatusPending,
		CreatedBy:    user.ID,
	}
	err := repo.Create(ctx, doc1)
	require.NoError(t, err)

	// Try to create second document with same content hash in same tenant
	doc2 := &models.Document{
		TenantID:     tenant.ID,
		FileName:     "test-doc-2.pdf",
		OriginalName: "Test Document 2.pdf",
		ContentType:  "application/pdf",
		FileSize:     1024,
		StoragePath:  "/test/documents/test-doc-2.pdf",
		ContentHash:  "samehash123456789", // Same hash
		Title:        "Test Document 2",
		DocumentType: models.DocTypeGeneral,
		Status:       models.DocStatusPending,
		CreatedBy:    user.ID,
	}
	err = repo.Create(ctx, doc2)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "duplicate")
}

func TestDocumentRepository_GetByID(t *testing.T) {
	db := testutil.NewTestDB(t)
	defer db.Cleanup(t)

	repo := NewDocumentRepository(db.DB)
	ctx := context.Background()

	tenant := db.CreateTestTenant(t)
	user := db.CreateTestUser(t, tenant)
	original := db.CreateTestDocument(t, tenant, user)

	// Get by ID
	found, err := repo.GetByID(ctx, original.ID)
	require.NoError(t, err)
	assert.Equal(t, original.ID, found.ID)
	assert.Equal(t, original.TenantID, found.TenantID)
	assert.Equal(t, original.FileName, found.FileName)
	assert.Equal(t, original.Title, found.Title)
	assert.Equal(t, original.ContentHash, found.ContentHash)
	assert.Equal(t, original.CreatedBy, found.CreatedBy)
}

func TestDocumentRepository_GetByID_NotFound(t *testing.T) {
	db := testutil.NewTestDB(t)
	defer db.Cleanup(t)

	repo := NewDocumentRepository(db.DB)
	ctx := context.Background()

	// Try to get non-existent document
	_, err := repo.GetByID(ctx, uuid.New())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestDocumentRepository_Update(t *testing.T) {
	db := testutil.NewTestDB(t)
	defer db.Cleanup(t)

	repo := NewDocumentRepository(db.DB)
	ctx := context.Background()

	tenant := db.CreateTestTenant(t)
	user := db.CreateTestUser(t, tenant)
	original := db.CreateTestDocument(t, tenant, user)

	// Update document
	original.Title = "Updated Title"
	original.Status = models.DocStatusCompleted
	original.DocumentType = models.DocTypeInvoice
	original.UpdatedBy = &user.ID

	err := repo.Update(ctx, original)
	require.NoError(t, err)

	// Verify update
	found, err := repo.GetByID(ctx, original.ID)
	require.NoError(t, err)
	assert.Equal(t, "Updated Title", found.Title)
	assert.Equal(t, models.DocStatusCompleted, found.Status)
	assert.Equal(t, models.DocTypeInvoice, found.DocumentType)
	assert.Equal(t, user.ID, *found.UpdatedBy)
}

func TestDocumentRepository_ListByTenant(t *testing.T) {
	db := testutil.NewTestDB(t)
	defer db.Cleanup(t)

	repo := NewDocumentRepository(db.DB)
	ctx := context.Background()

	tenant1 := db.CreateTestTenant(t)
	tenant2 := db.CreateTestTenant(t)
	user1 := db.CreateTestUser(t, tenant1)
	user2 := db.CreateTestUser(t, tenant2)

	// Create documents in different tenants
	_ = db.CreateTestDocument(t, tenant1, user1)
	_ = db.CreateTestDocument(t, tenant1, user1)
	_ = db.CreateTestDocument(t, tenant2, user2)

	filters := repositories.DocumentFilters{
		ListParams: repositories.ListParams{
			Page:     1,
			PageSize: 10,
		},
	}

	// List documents for tenant1 (using List method instead of ListByTenant)
	docs1, total1, err := repo.List(ctx, tenant1.ID, filters)
	require.NoError(t, err)
	assert.Equal(t, int64(2), total1)
	assert.Len(t, docs1, 2)

	// Verify all documents belong to tenant1
	for _, doc := range docs1 {
		assert.Equal(t, tenant1.ID, doc.TenantID)
	}

	// List documents for tenant2
	docs2, total2, err := repo.List(ctx, tenant2.ID, filters)
	require.NoError(t, err)
	assert.Equal(t, int64(1), total2)
	assert.Len(t, docs2, 1)
	assert.Equal(t, tenant2.ID, docs2[0].TenantID)
}

func TestDocumentRepository_SearchDocuments(t *testing.T) {
	db := testutil.NewTestDB(t)
	defer db.Cleanup(t)

	repo := NewDocumentRepository(db.DB)
	ctx := context.Background()

	tenant := db.CreateTestTenant(t)
	user := db.CreateTestUser(t, tenant)

	// Create documents with different titles and content
	doc1 := &models.Document{
		TenantID:      tenant.ID,
		FileName:      "invoice-2024.pdf",
		OriginalName:  "Invoice 2024.pdf",
		ContentType:   "application/pdf",
		FileSize:      1024,
		StoragePath:   "/test/invoices/invoice-2024.pdf",
		ContentHash:   "hash1",
		Title:         "Annual Invoice Report",
		ExtractedText: "This document contains financial data and invoice details for 2024",
		DocumentType:  models.DocTypeInvoice,
		Status:        models.DocStatusCompleted,
		CreatedBy:     user.ID,
	}
	err := repo.Create(ctx, doc1)
	require.NoError(t, err)

	doc2 := &models.Document{
		TenantID:      tenant.ID,
		FileName:      "contract-2024.pdf",
		OriginalName:  "Contract 2024.pdf",
		ContentType:   "application/pdf",
		FileSize:      2048,
		StoragePath:   "/test/contracts/contract-2024.pdf",
		ContentHash:   "hash2",
		Title:         "Service Contract Agreement",
		ExtractedText: "This is a legal contract for services provided in 2024",
		DocumentType:  models.DocTypeContract,
		Status:        models.DocStatusCompleted,
		CreatedBy:     user.ID,
	}
	err = repo.Create(ctx, doc2)
	require.NoError(t, err)

	query := repositories.SearchQuery{
		Query: "invoice",
		Limit: 10,
	}

	// Search for "invoice" (using Search method instead of SearchDocuments)
	docs, err := repo.Search(ctx, tenant.ID, query)
	require.NoError(t, err)
	assert.Len(t, docs, 1)
	assert.Equal(t, doc1.ID, docs[0].ID)
	assert.Contains(t, docs[0].Title, "Invoice")

	// Search for "2024" (should return both)
	query.Query = "2024"
	docs, err = repo.Search(ctx, tenant.ID, query)
	require.NoError(t, err)
	assert.Len(t, docs, 2)
}

func TestDocumentRepository_GetByContentHash(t *testing.T) {
	db := testutil.NewTestDB(t)
	defer db.Cleanup(t)

	repo := NewDocumentRepository(db.DB)
	ctx := context.Background()

	tenant := db.CreateTestTenant(t)
	user := db.CreateTestUser(t, tenant)
	original := db.CreateTestDocument(t, tenant, user)

	// Get by content hash
	found, err := repo.GetByContentHash(ctx, tenant.ID, original.ContentHash)
	require.NoError(t, err)
	assert.Equal(t, original.ID, found.ID)
	assert.Equal(t, original.ContentHash, found.ContentHash)
}

func TestDocumentRepository_GetByContentHash_DifferentTenant(t *testing.T) {
	db := testutil.NewTestDB(t)
	defer db.Cleanup(t)

	repo := NewDocumentRepository(db.DB)
	ctx := context.Background()

	tenant1 := db.CreateTestTenant(t)
	tenant2 := db.CreateTestTenant(t)
	user1 := db.CreateTestUser(t, tenant1)
	document := db.CreateTestDocument(t, tenant1, user1)

	// Try to get document from different tenant
	_, err := repo.GetByContentHash(ctx, tenant2.ID, document.ContentHash)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestDocumentRepository_UpdateStatus(t *testing.T) {
	db := testutil.NewTestDB(t)
	defer db.Cleanup(t)

	repo := NewDocumentRepository(db.DB)
	ctx := context.Background()

	tenant := db.CreateTestTenant(t)
	user := db.CreateTestUser(t, tenant)
	document := db.CreateTestDocument(t, tenant, user)

	// Update status
	newStatus := models.DocStatusCompleted
	err := repo.UpdateStatus(ctx, document.ID, newStatus)
	require.NoError(t, err)

	// Verify status update
	found, err := repo.GetByID(ctx, document.ID)
	require.NoError(t, err)
	assert.Equal(t, newStatus, found.Status)
}

func TestDocumentRepository_Delete(t *testing.T) {
	db := testutil.NewTestDB(t)
	defer db.Cleanup(t)

	repo := NewDocumentRepository(db.DB)
	ctx := context.Background()

	tenant := db.CreateTestTenant(t)
	user := db.CreateTestUser(t, tenant)
	document := db.CreateTestDocument(t, tenant, user)

	// Delete document
	err := repo.Delete(ctx, document.ID)
	require.NoError(t, err)

	// Verify deletion
	_, err = repo.GetByID(ctx, document.ID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestDocumentRepository_ListByFolder(t *testing.T) {
	db := testutil.NewTestDB(t)
	defer db.Cleanup(t)

	repo := NewDocumentRepository(db.DB)
	ctx := context.Background()

	tenant := db.CreateTestTenant(t)
	user := db.CreateTestUser(t, tenant)

	// Create folder
	folder := &models.Folder{
		ID:        uuid.New(),
		TenantID:  tenant.ID,
		Name:      "Test Folder",
		Path:      "/test-folder",
		Level:     1,
		CreatedBy: user.ID,
	}
	err := db.DB.Create(folder).Error
	require.NoError(t, err)

	// Create documents in folder and outside folder
	docInFolder := &models.Document{
		TenantID:     tenant.ID,
		FolderID:     &folder.ID,
		FileName:     "doc-in-folder.pdf",
		OriginalName: "Document in Folder.pdf",
		ContentType:  "application/pdf",
		FileSize:     1024,
		StoragePath:  "/test/folder/doc-in-folder.pdf",
		ContentHash:  "hash1",
		Title:        "Document in Folder",
		DocumentType: models.DocTypeGeneral,
		Status:       models.DocStatusCompleted,
		CreatedBy:    user.ID,
	}
	err = repo.Create(ctx, docInFolder)
	require.NoError(t, err)

	docOutsideFolder := db.CreateTestDocument(t, tenant, user)

	params := repositories.ListParams{
		Page:     1,
		PageSize: 10,
	}

	// List documents in folder (using GetByFolder method)
	docs, total, err := repo.GetByFolder(ctx, folder.ID, params)
	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
	assert.Len(t, docs, 1)
	assert.Equal(t, docInFolder.ID, docs[0].ID)
	assert.Equal(t, &folder.ID, docs[0].FolderID)

	// Verify the document outside folder is not included
	assert.NotEqual(t, docOutsideFolder.ID, docs[0].ID)
}

func TestDocumentRepository_CountByStatus(t *testing.T) {
	// Skip this test as CountByStatus method doesn't exist in the interface
	t.Skip("CountByStatus method not implemented in repository interface")
}

func TestDocumentRepository_GetTotalSize(t *testing.T) {
	// Skip this test as GetTotalSize method doesn't exist in the interface
	t.Skip("GetTotalSize method not implemented in repository interface")
}

func TestDocumentRepository_MultiTenant_Isolation(t *testing.T) {
	db := testutil.NewTestDB(t)
	defer db.Cleanup(t)

	repo := NewDocumentRepository(db.DB)
	ctx := context.Background()

	// Create two tenants with documents
	tenant1 := db.CreateTestTenant(t)
	tenant2 := db.CreateTestTenant(t)
	user1 := db.CreateTestUser(t, tenant1)
	user2 := db.CreateTestUser(t, tenant2)

	doc1 := db.CreateTestDocument(t, tenant1, user1)
	doc2 := db.CreateTestDocument(t, tenant2, user2)

	filters := repositories.DocumentFilters{
		ListParams: repositories.ListParams{Page: 1, PageSize: 10},
	}

	// List documents for tenant1 - should only see doc1 (using List method)
	docs1, total1, err := repo.List(ctx, tenant1.ID, filters)
	require.NoError(t, err)
	assert.Equal(t, int64(1), total1)
	assert.Len(t, docs1, 1)
	assert.Equal(t, doc1.ID, docs1[0].ID)

	// List documents for tenant2 - should only see doc2
	docs2, total2, err := repo.List(ctx, tenant2.ID, filters)
	require.NoError(t, err)
	assert.Equal(t, int64(1), total2)
	assert.Len(t, docs2, 1)
	assert.Equal(t, doc2.ID, docs2[0].ID)

	// Verify complete isolation
	assert.NotEqual(t, docs1[0].ID, docs2[0].ID)
	assert.Equal(t, tenant1.ID, docs1[0].TenantID)
	assert.Equal(t, tenant2.ID, docs2[0].TenantID)
}
