//go:build !postgres

package database

import (
	"fmt"
	"strings"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type DB struct {
	*gorm.DB
}

// New creates a new database connection
func New(databaseURL string) (*DB, error) {
	// Configure GORM
	config := &gorm.Config{
		PrepareStmt: true,
		NowFunc: func() time.Time {
			return time.Now().UTC()
		},
		Logger: logger.Default.LogMode(logger.Info),
	}

	var db *gorm.DB
	var err error

	// Determine database type based on URL format
	if strings.HasPrefix(databaseURL, "file:") || strings.HasSuffix(databaseURL, ".db") {
		// SQLite connection
		db, err = gorm.Open(sqlite.Open(databaseURL), config)
	} else {
		// PostgreSQL connection
		db, err = gorm.Open(postgres.Open(databaseURL), config)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	// Configure connection pool (only for non-SQLite)
	if !strings.HasPrefix(databaseURL, "file:") && !strings.HasSuffix(databaseURL, ".db") {
		sqlDB, err := db.DB()
		if err != nil {
			return nil, fmt.Errorf("failed to get underlying sql.DB: %w", err)
		}

		// Set connection pool settings
		sqlDB.SetMaxIdleConns(10)
		sqlDB.SetMaxOpenConns(100)
		sqlDB.SetConnMaxLifetime(time.Hour)

		// Enable required PostgreSQL extensions
		if err := enableExtensions(db); err != nil {
			return nil, fmt.Errorf("failed to enable extensions: %w", err)
		}
	}

	return &DB{DB: db}, nil
}

// Close closes the database connection
func (db *DB) Close() error {
	sqlDB, err := db.DB.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}

// Ping checks if the database connection is alive
func (db *DB) Ping() error {
	sqlDB, err := db.DB.DB()
	if err != nil {
		return err
	}
	return sqlDB.Ping()
}

// AutoMigrate runs database migrations
func (db *DB) AutoMigrate(models ...interface{}) error {
	return db.DB.AutoMigrate(models...)
}

// enableExtensions enables required PostgreSQL extensions
func enableExtensions(db *gorm.DB) error {
	extensions := []string{
		"CREATE EXTENSION IF NOT EXISTS \"uuid-ossp\"",
		"CREATE EXTENSION IF NOT EXISTS \"vector\"",
	}

	for _, ext := range extensions {
		if err := db.Exec(ext).Error; err != nil {
			// For SQLite, these extensions don't exist, so we can ignore the error
			// In a real implementation, you might want to check the database type
			continue
		}
	}

	return nil
}
