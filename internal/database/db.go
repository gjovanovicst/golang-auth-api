package database

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/gjovanovicst/auth_api/pkg/models"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var DB *gorm.DB

// ConnectDatabase establishes connection to PostgreSQL database
func ConnectDatabase() {
	dsn := fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%s sslmode=disable TimeZone=UTC",
		os.Getenv("DB_HOST"),
		os.Getenv("DB_USER"),
		os.Getenv("DB_PASSWORD"),
		os.Getenv("DB_NAME"),
		os.Getenv("DB_PORT"),
	)

	newLogger := logger.New(
		log.New(os.Stdout, "\r\n", log.LstdFlags), // io writer
		logger.Config{
			SlowThreshold:             time.Second, // Slow SQL threshold
			LogLevel:                  logger.Info, // Log level
			IgnoreRecordNotFoundError: true,        // Ignore ErrRecordNotFound error for logger
			Colorful:                  true,        // Enable color
		},
	)

	var err error
	DB, err = gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: newLogger,
	})

	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	log.Println("Database connected successfully!")
}

// MigrateDatabase runs GORM auto-migration for all models
func MigrateDatabase() {
	// AutoMigrate will create tables, missing columns, and missing indexes
	// It will NOT change existing column types or delete unused columns
	// NOTE: For critical migrations (like adding NOT NULL columns to existing tables),
	// use manually applied SQL migrations via scripts/migrate.sh BEFORE running the app.
	err := DB.AutoMigrate(
		&models.User{},
		&models.SocialAccount{},
		&models.ActivityLog{},
		&models.SchemaMigration{},   // Migration tracking table
		&models.AdminAccount{},      // Admin GUI accounts
		&models.ApiKey{},            // API keys (admin + per-app)
		&models.SystemSetting{},     // System settings (DB-backed config)
		&models.EmailServerConfig{}, // Per-app SMTP configuration
		&models.EmailType{},         // Email type registry
		&models.EmailTemplate{},     // Email templates (per-app and global)
	)

	if err != nil {
		log.Printf("GORM AutoMigrate Warning: %v. This might be expected if manual SQL migration is pending.", err)
		// We don't Fatalf here because sometimes GORM conflicts with complex manual migrations
		// log.Fatalf("Failed to migrate database: %v", err)
	}

	log.Println("Database migration check completed!")
}
