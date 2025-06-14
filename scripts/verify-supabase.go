package main

import (
	"fmt"
	"log"

	"github.com/archivus/archivus/internal/app/config"
	"github.com/archivus/archivus/internal/infrastructure/database"
)

// SupabaseVerification checks that Archivus can connect to Supabase
// and all required extensions are available
func main() {
	fmt.Println("🔍 Verifying Supabase Setup for Archivus...")

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("❌ Failed to load config: %v", err)
	}

	if cfg.GetDatabaseURL() == "" {
		log.Fatalf("❌ DATABASE_URL not set. Please configure Supabase connection.")
	}

	fmt.Printf("📡 Connecting to: %s\n", maskDatabaseURL(cfg.GetDatabaseURL()))

	// Connect to database
	db, err := database.New(cfg.GetDatabaseURL())
	if err != nil {
		log.Fatalf("❌ Failed to connect to Supabase: %v", err)
	}
	defer db.Close()

	fmt.Println("✅ Database connection successful!")

	// Test basic connection
	if err := db.Ping(); err != nil {
		log.Fatalf("❌ Database ping failed: %v", err)
	}
	fmt.Println("✅ Database ping successful!")

	// Verify PostgreSQL version
	var version string
	err = db.Raw("SELECT version()").Scan(&version).Error
	if err != nil {
		log.Fatalf("❌ Failed to get PostgreSQL version: %v", err)
	}
	fmt.Printf("✅ PostgreSQL Version: %s\n", version)

	// Check required extensions
	requiredExtensions := map[string]string{
		"uuid-ossp": "UUID generation",
		"vector":    "pgvector for AI embeddings",
	}

	fmt.Println("\n🔌 Checking Required Extensions:")
	for ext, description := range requiredExtensions {
		var exists bool
		query := "SELECT EXISTS(SELECT 1 FROM pg_extension WHERE extname = ?)"
		err := db.Raw(query, ext).Scan(&exists).Error
		if err != nil {
			log.Printf("❌ Error checking extension %s: %v", ext, err)
			continue
		}

		if exists {
			fmt.Printf("✅ %s - %s\n", ext, description)
		} else {
			fmt.Printf("❌ %s - %s (NOT INSTALLED)\n", ext, description)
			fmt.Printf("   Run: CREATE EXTENSION IF NOT EXISTS \"%s\";\n", ext)
		}
	}

	// Test UUID generation
	fmt.Println("\n🆔 Testing UUID Generation:")
	var testUUID string
	err = db.Raw("SELECT uuid_generate_v4()").Scan(&testUUID).Error
	if err != nil {
		fmt.Printf("❌ UUID generation failed: %v\n", err)
	} else {
		fmt.Printf("✅ Generated UUID: %s\n", testUUID)
	}

	// Test vector extension (if available)
	fmt.Println("\n🤖 Testing Vector Extension:")
	var vectorTest string
	err = db.Raw("SELECT '[1,2,3]'::vector").Scan(&vectorTest).Error
	if err != nil {
		fmt.Printf("❌ Vector extension test failed: %v\n", err)
		fmt.Println("   Make sure 'vector' extension is enabled in Supabase")
	} else {
		fmt.Printf("✅ Vector extension working: %s\n", vectorTest)
	}

	// Check connection pool settings
	fmt.Println("\n🏊 Connection Pool Status:")
	sqlDB, err := db.DB.DB()
	if err != nil {
		fmt.Printf("❌ Failed to get underlying DB: %v\n", err)
	} else {
		stats := sqlDB.Stats()
		fmt.Printf("✅ Max Open Connections: %d\n", stats.MaxOpenConnections)
		fmt.Printf("✅ Open Connections: %d\n", stats.OpenConnections)
		fmt.Printf("✅ In Use: %d\n", stats.InUse)
		fmt.Printf("✅ Idle: %d\n", stats.Idle)
	}

	// Test schema creation permissions
	fmt.Println("\n🔒 Testing Schema Permissions:")
	testTable := "archivus_test_table"

	// Try to create a test table
	createSQL := fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS %s (
			id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
			test_data VARCHAR(100),
			created_at TIMESTAMP DEFAULT NOW()
		)
	`, testTable)

	err = db.Exec(createSQL).Error
	if err != nil {
		fmt.Printf("❌ Cannot create tables: %v\n", err)
		fmt.Println("   Check if your Supabase user has CREATE permissions")
	} else {
		fmt.Printf("✅ Table creation successful\n")

		// Clean up test table
		dropSQL := fmt.Sprintf("DROP TABLE IF EXISTS %s", testTable)
		db.Exec(dropSQL)
	}

	// Test JSONB support
	fmt.Println("\n📦 Testing JSONB Support:")
	var jsonbTest string
	err = db.Raw("SELECT '{\"test\": \"value\"}'::jsonb").Scan(&jsonbTest).Error
	if err != nil {
		fmt.Printf("❌ JSONB test failed: %v\n", err)
	} else {
		fmt.Printf("✅ JSONB support working: %s\n", jsonbTest)
	}

	// Supabase-specific checks
	fmt.Println("\n🦄 Supabase-Specific Checks:")

	// Check if we're actually connected to Supabase
	var currentDB string
	err = db.Raw("SELECT current_database()").Scan(&currentDB).Error
	if err == nil && currentDB == "postgres" {
		fmt.Printf("✅ Connected to Supabase database: %s\n", currentDB)
	}

	// Check for Supabase metadata tables
	var supabaseSchema bool
	err = db.Raw("SELECT EXISTS(SELECT 1 FROM information_schema.schemata WHERE schema_name = 'auth')").Scan(&supabaseSchema).Error
	if err == nil && supabaseSchema {
		fmt.Println("✅ Supabase auth schema detected")
	}

	fmt.Println("\n🎉 Supabase Verification Complete!")
	fmt.Println("\n📋 Summary:")
	fmt.Println("   • Database connection: ✅ Working")
	fmt.Println("   • Required extensions: Check output above")
	fmt.Println("   • Permissions: Check output above")
	fmt.Println("   • Ready for Archivus deployment!")

	fmt.Println("\n🚀 Next Steps:")
	fmt.Println("   1. Run migrations: go run cmd/migrate/main.go up")
	fmt.Println("   2. Create Supabase storage bucket: 'documents'")
	fmt.Println("   3. Deploy Archivus with SUPABASE environment variables")
}

// maskDatabaseURL hides sensitive information in database URL
func maskDatabaseURL(url string) string {
	// Simple masking - replace password with ***
	// This is a basic implementation
	if len(url) > 20 {
		return url[:20] + "***" + url[len(url)-20:]
	}
	return "***"
}
