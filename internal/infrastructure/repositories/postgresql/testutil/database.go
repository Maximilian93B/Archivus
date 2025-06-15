package testutil

import (
	"fmt"
	"os"
	"testing"

	"github.com/archivus/archivus/internal/infrastructure/database"
	"github.com/archivus/archivus/internal/infrastructure/database/models"
	"github.com/google/uuid"
)

// TestDB wraps the database for testing
type TestDB struct {
	*database.DB
}

// NewTestDB creates a new test database connection
func NewTestDB(t *testing.T) *TestDB {
	t.Helper()

	// Use DATABASE_URL_TEST if available (for Docker), otherwise SQLite
	databaseURL := os.Getenv("DATABASE_URL_TEST")
	if databaseURL == "" {
		// Use SQLite in-memory for testing
		databaseURL = "file::memory:?cache=shared"
		t.Logf("Using SQLite in-memory database for testing")
	} else {
		t.Logf("Using PostgreSQL database for testing: %s", databaseURL)
	}

	db, err := database.New(databaseURL)
	if err != nil {
		t.Fatalf("Failed to connect to test database: %v", err)
	}

	// Auto-migrate all models
	if err := db.AutoMigrate(models.GetAllModels()...); err != nil {
		t.Fatalf("Failed to migrate test database: %v", err)
	}

	return &TestDB{DB: db}
}

// Cleanup closes the test database
func (db *TestDB) Cleanup(t *testing.T) {
	t.Helper()
	if err := db.Close(); err != nil {
		t.Errorf("Failed to close test database: %v", err)
	}
}

// CreateTestTenant creates a test tenant
func (db *TestDB) CreateTestTenant(t *testing.T) *models.Tenant {
	t.Helper()

	tenant := &models.Tenant{
		ID:        uuid.New(),
		Name:      fmt.Sprintf("Test Tenant %s", uuid.New().String()[:8]),
		Subdomain: fmt.Sprintf("test-%s", uuid.New().String()[:8]),
		IsActive:  true,
	}

	if err := db.Create(tenant).Error; err != nil {
		t.Fatalf("Failed to create test tenant: %v", err)
	}

	return tenant
}

// CreateTestUser creates a test user
func (db *TestDB) CreateTestUser(t *testing.T, tenant *models.Tenant) *models.User {
	t.Helper()

	user := &models.User{
		ID:           uuid.New(),
		TenantID:     tenant.ID,
		Email:        fmt.Sprintf("test-%s@example.com", uuid.New().String()[:8]),
		PasswordHash: "hashedpassword",
		FirstName:    "Test",
		LastName:     "User",
		Role:         models.UserRoleUser,
		IsActive:     true,
	}

	if err := db.Create(user).Error; err != nil {
		t.Fatalf("Failed to create test user: %v", err)
	}

	return user
}

// CreateTestDocument creates a test document
func (db *TestDB) CreateTestDocument(t *testing.T, tenant *models.Tenant, user *models.User) *models.Document {
	t.Helper()

	document := &models.Document{
		ID:           uuid.New(),
		TenantID:     tenant.ID,
		FileName:     fmt.Sprintf("test-doc-%s.pdf", uuid.New().String()[:8]),
		OriginalName: "test-document.pdf",
		ContentType:  "application/pdf",
		FileSize:     1024,
		StoragePath:  "/test/path/document.pdf",
		ContentHash:  fmt.Sprintf("hash-%s", uuid.New().String()[:16]),
		Title:        "Test Document",
		DocumentType: models.DocTypeGeneral,
		Status:       models.DocStatusPending,
		CreatedBy:    user.ID,
	}

	if err := db.Create(document).Error; err != nil {
		t.Fatalf("Failed to create test document: %v", err)
	}

	return document
}
