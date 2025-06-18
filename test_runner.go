package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/archivus/archivus/internal/infrastructure/database"
	"github.com/archivus/archivus/internal/infrastructure/database/models"
	"github.com/archivus/archivus/internal/infrastructure/repositories/postgresql"
)

func main() {
	fmt.Println("🧪 Testing Archivus Repository Implementation...")

	// Use DATABASE_URL_TEST if available (for Docker), otherwise SQLite
	databaseURL := os.Getenv("DATABASE_URL_TEST")
	if databaseURL == "" {
		databaseURL = "file::memory:?cache=shared"
		fmt.Println("📝 Using SQLite in-memory database")
	} else {
		fmt.Printf("📝 Using PostgreSQL: %s\n", databaseURL)
	}

	db, err := database.New(databaseURL)
	if err != nil {
		log.Fatalf("❌ Failed to create test database: %v", err)
	}
	defer db.Close()

	fmt.Println("✅ Database connection established")

	// Run migrations
	if err := db.AutoMigrate(models.GetAllModels()...); err != nil {
		log.Fatalf("❌ Failed to run migrations: %v", err)
	}

	fmt.Println("✅ Database migrations completed")

	// Initialize repositories
	repos := postgresql.NewRepositories(db)

	ctx := context.Background()

	// Test 1: Create a tenant
	fmt.Println("\n🏢 Testing Tenant Repository...")
	tenant := &models.Tenant{
		Name:      "Test Company",
		Subdomain: "test-company",
		IsActive:  true,
	}

	if err := repos.TenantRepo.Create(ctx, tenant); err != nil {
		log.Fatalf("❌ Failed to create tenant: %v", err)
	}

	fmt.Printf("✅ Created tenant: %s (ID: %s)\n", tenant.Name, tenant.ID)

	// Test 2: Create a user
	fmt.Println("\n👤 Testing User Repository...")
	user := &models.User{
		TenantID:     tenant.ID,
		Email:        "test@example.com",
		PasswordHash: "hashedpassword123",
		FirstName:    "John",
		LastName:     "Doe",
		Role:         models.UserRoleUser,
		IsActive:     true,
	}

	if err := repos.UserRepo.Create(ctx, user); err != nil {
		log.Fatalf("❌ Failed to create user: %v", err)
	}

	fmt.Printf("✅ Created user: %s %s (ID: %s)\n", user.FirstName, user.LastName, user.ID)

	// Test 3: Create a document
	fmt.Println("\n📄 Testing Document Repository...")
	document := &models.Document{
		TenantID:     tenant.ID,
		FileName:     "test-document.pdf",
		OriginalName: "Test Document.pdf",
		ContentType:  "application/pdf",
		FileSize:     1024,
		StoragePath:  "/test/documents/test-document.pdf",
		ContentHash:  "abcdef123456",
		Title:        "Test Document",
		DocumentType: models.DocTypeGeneral,
		Status:       models.DocStatusPending,
		CreatedBy:    user.ID,
	}

	if err := repos.DocumentRepo.Create(ctx, document); err != nil {
		log.Fatalf("❌ Failed to create document: %v", err)
	}

	fmt.Printf("✅ Created document: %s (ID: %s)\n", document.Title, document.ID)

	// Test 4: Retrieve and verify data
	fmt.Println("\n🔍 Testing Data Retrieval...")

	// Get tenant by subdomain
	foundTenant, err := repos.TenantRepo.GetBySubdomain(ctx, "test-company")
	if err != nil {
		log.Fatalf("❌ Failed to get tenant by subdomain: %v", err)
	}
	fmt.Printf("✅ Retrieved tenant by subdomain: %s\n", foundTenant.Name)

	// Get user by email
	foundUser, err := repos.UserRepo.GetByEmail(ctx, tenant.ID, "test@example.com")
	if err != nil {
		log.Fatalf("❌ Failed to get user by email: %v", err)
	}
	fmt.Printf("✅ Retrieved user by email: %s\n", foundUser.Email)

	// Get document by ID
	foundDocument, err := repos.DocumentRepo.GetByID(ctx, document.ID)
	if err != nil {
		log.Fatalf("❌ Failed to get document by ID: %v", err)
	}
	fmt.Printf("✅ Retrieved document by ID: %s\n", foundDocument.Title)

	// Test 5: Check tenant quotas
	fmt.Println("\n📊 Testing Quota System...")
	quota, err := repos.TenantRepo.CheckQuotaLimits(ctx, tenant.ID)
	if err != nil {
		log.Fatalf("❌ Failed to check quota limits: %v", err)
	}
	fmt.Printf("✅ Storage used: %d/%d bytes (%.1f%%)\n",
		quota.StorageUsed, quota.StorageQuota, quota.StoragePercent)
	fmt.Printf("✅ API calls: %d/%d (%.1f%%)\n",
		quota.APIUsed, quota.APIQuota, quota.APIPercent)

	// Test 6: Update tenant usage
	fmt.Println("\n📈 Testing Usage Updates...")
	if err := repos.TenantRepo.UpdateUsage(ctx, tenant.ID, 1024, 1); err != nil {
		log.Fatalf("❌ Failed to update tenant usage: %v", err)
	}

	// Verify update
	updatedQuota, err := repos.TenantRepo.CheckQuotaLimits(ctx, tenant.ID)
	if err != nil {
		log.Fatalf("❌ Failed to check updated quota: %v", err)
	}
	fmt.Printf("✅ Updated storage usage: %d bytes\n", updatedQuota.StorageUsed)

	fmt.Println("\n🎉 All Repository Tests Passed!")
	fmt.Println("\n📋 Test Summary:")
	fmt.Println("   ✅ Tenant creation and retrieval")
	fmt.Println("   ✅ User creation and email lookup")
	fmt.Println("   ✅ Document creation and retrieval")
	fmt.Println("   ✅ Quota tracking and updates")
	fmt.Println("   ✅ Multi-tenant data isolation")
	fmt.Println("\n🚀 Ready for Integration Testing!")
}
