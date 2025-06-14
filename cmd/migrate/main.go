package main

import (
	"fmt"
	"log"
	"os"

	"github.com/archivus/archivus/internal/app/config"
	"github.com/archivus/archivus/internal/infrastructure/database"
	"github.com/archivus/archivus/internal/infrastructure/database/models"
	"github.com/archivus/archivus/pkg/logger"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		return
	}

	command := os.Args[1]

	// Initialize logger
	logger := logger.New()

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Connect to database
	db, err := database.New(cfg.GetDatabaseURL())
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	switch command {
	case "up":
		runMigrations(db, logger)
	case "down":
		rollbackMigrations(db, logger)
	case "reset":
		resetDatabase(db, logger)
	case "seed":
		seedDatabase(db, logger)
	case "status":
		migrationStatus(db, logger)
	default:
		logger.Error("Unknown command", "command", command)
		printUsage()
	}
}

func printUsage() {
	fmt.Println("Usage: go run cmd/migrate/main.go <command>")
	fmt.Println("")
	fmt.Println("Commands:")
	fmt.Println("  up     - Run all pending migrations")
	fmt.Println("  down   - Rollback the last migration")
	fmt.Println("  reset  - Drop all tables and recreate them")
	fmt.Println("  seed   - Seed the database with initial data")
	fmt.Println("  status - Show migration status")
}

func runMigrations(db *database.DB, logger *logger.Logger) {
	logger.Info("Running database migrations...")

	// Auto-migrate all models
	if err := db.AutoMigrate(models.GetAllModels()...); err != nil {
		logger.Error("Failed to run migrations", "error", err)
		return
	}

	// Create indexes for better performance
	if err := createIndexes(db); err != nil {
		logger.Error("Failed to create indexes", "error", err)
		return
	}

	logger.Info("Database migrations completed successfully")
}

func rollbackMigrations(db *database.DB, logger *logger.Logger) {
	logger.Info("Rolling back migrations...")
	
	// Note: GORM doesn't support rollbacks out of the box
	// This is a simplified version - in production you might want to use a migration tool like golang-migrate
	logger.Warn("Rollback not implemented - use 'reset' to recreate schema")
}

func resetDatabase(db *database.DB, logger *logger.Logger) {
	logger.Info("Resetting database...")

	// Drop all tables in reverse order to handle foreign key constraints
	tables := []interface{}{
		&models.Share{},
		&models.AuditLog{},
		&models.AIProcessingJob{},
		&models.Document{},
		&models.Tag{},
		&models.Category{},
		&models.Folder{},
		&models.User{},
		&models.Tenant{},
	}

	for _, table := range tables {
		if err := db.Migrator().DropTable(table); err != nil {
			logger.Error("Failed to drop table", "error", err)
		}
	}

	// Also drop junction tables
	junctionTables := []string{
		"document_tags",
		"document_categories",
	}

	for _, table := range junctionTables {
		if err := db.Exec(fmt.Sprintf("DROP TABLE IF EXISTS %s", table)).Error; err != nil {
			logger.Error("Failed to drop junction table", "table", table, "error", err)
		}
	}

	// Recreate all tables
	runMigrations(db, logger)
	
	logger.Info("Database reset completed")
}

func seedDatabase(db *database.DB, logger *logger.Logger) {
	logger.Info("Seeding database with initial data...")

	// Create default tenant
	defaultTenant := &models.Tenant{
		Name:      "Default Tenant",
		Subdomain: "default",
		IsActive:  true,
	}

	if err := db.FirstOrCreate(defaultTenant, models.Tenant{Subdomain: "default"}).Error; err != nil {
		logger.Error("Failed to create default tenant", "error", err)
		return
	}

	// Create default categories
	defaultCategories := []models.Category{
		{
			TenantID:    defaultTenant.ID,
			Name:        "Documents",
			Description: "General documents",
			Color:       "#3B82F6",
			Icon:        "document",
			IsSystem:    true,
		},
		{
			TenantID:    defaultTenant.ID,
			Name:        "Images",
			Description: "Image files",
			Color:       "#10B981",
			Icon:        "photo",
			IsSystem:    true,
		},
		{
			TenantID:    defaultTenant.ID,
			Name:        "Contracts",
			Description: "Legal contracts and agreements",
			Color:       "#F59E0B",
			Icon:        "document-text",
			IsSystem:    true,
		},
		{
			TenantID:    defaultTenant.ID,
			Name:        "Reports",
			Description: "Business reports and analytics",
			Color:       "#8B5CF6",
			Icon:        "chart-bar",
			IsSystem:    true,
		},
	}

	for _, category := range defaultCategories {
		if err := db.FirstOrCreate(&category, models.Category{
			TenantID: defaultTenant.ID,
			Name:     category.Name,
		}).Error; err != nil {
			logger.Error("Failed to create category", "name", category.Name, "error", err)
		}
	}

	// Create default tags
	defaultTags := []models.Tag{
		{
			TenantID: defaultTenant.ID,
			Name:     "important",
			Color:    "#EF4444",
		},
		{
			TenantID: defaultTenant.ID,
			Name:     "urgent",
			Color:    "#F97316",
		},
		{
			TenantID: defaultTenant.ID,
			Name:     "draft",
			Color:    "#6B7280",
		},
		{
			TenantID: defaultTenant.ID,
			Name:     "reviewed",
			Color:    "#10B981",
		},
		{
			TenantID: defaultTenant.ID,
			Name:     "archived",
			Color:    "#6B7280",
		},
	}

	for _, tag := range defaultTags {
		if err := db.FirstOrCreate(&tag, models.Tag{
			TenantID: defaultTenant.ID,
			Name:     tag.Name,
		}).Error; err != nil {
			logger.Error("Failed to create tag", "name", tag.Name, "error", err)
		}
	}

	logger.Info("Database seeding completed successfully")
}

func migrationStatus(db *database.DB, logger *logger.Logger) {
	logger.Info("Checking migration status...")

	// Check if tables exist
	tables := map[string]interface{}{
		"tenants":            &models.Tenant{},
		"users":              &models.User{},
		"folders":            &models.Folder{},
		"categories":         &models.Category{},
		"tags":               &models.Tag{},
		"documents":          &models.Document{},
		"ai_processing_jobs": &models.AIProcessingJob{},
		"audit_logs":         &models.AuditLog{},
		"shares":             &models.Share{},
	}

	for tableName, model := range tables {
		exists := db.Migrator().HasTable(model)
		status := "✓ exists"
		if !exists {
			status = "✗ missing"
		}
		logger.Info("Table status", "table", tableName, "status", status)
	}

	// Check junction tables
	junctionTables := []string{"document_tags", "document_categories"}
	for _, tableName := range junctionTables {
		var count int64
		err := db.Raw(fmt.Sprintf("SELECT COUNT(*) FROM information_schema.tables WHERE table_name = '%s'", tableName)).Scan(&count).Error
		status := "✓ exists"
		if err != nil || count == 0 {
			status = "✗ missing"
		}
		logger.Info("Junction table status", "table", tableName, "status", status)
	}
}

func createIndexes(db *database.DB) error {
	// Create additional indexes for better performance
	indexes := []string{
		"CREATE INDEX IF NOT EXISTS idx_documents_tenant_status ON documents(tenant_id, status)",
		"CREATE INDEX IF NOT EXISTS idx_documents_created_at ON documents(created_at)",
		"CREATE INDEX IF NOT EXISTS idx_documents_file_size ON documents(file_size)",
		"CREATE INDEX IF NOT EXISTS idx_ai_jobs_status_priority ON ai_processing_jobs(status, priority)",
		"CREATE INDEX IF NOT EXISTS idx_audit_logs_created_at ON audit_logs(created_at)",
		"CREATE INDEX IF NOT EXISTS idx_shares_expires_at ON shares(expires_at)",
		"CREATE INDEX IF NOT EXISTS idx_folders_path_gin ON folders USING gin(to_tsvector('english', path))",
		"CREATE INDEX IF NOT EXISTS idx_documents_text_gin ON documents USING gin(to_tsvector('english', coalesce(extracted_text, '') || ' ' || coalesce(ocr_text, '')))",
	}

	for _, indexSQL := range indexes {
		if err := db.Exec(indexSQL).Error; err != nil {
			return fmt.Errorf("failed to create index: %w", err)
		}
	}

	return nil
} 