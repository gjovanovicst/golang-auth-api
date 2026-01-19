package main

import (
	"fmt"
	"log"
	"os"

	"github.com/gjovanovicst/auth_api/pkg/models"
	"github.com/google/uuid"
	"github.com/joho/godotenv"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func main() {
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, relying on environment variables")
	}

	dsn := fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%s sslmode=disable TimeZone=UTC",
		os.Getenv("DB_HOST"),
		os.Getenv("DB_USER"),
		os.Getenv("DB_PASSWORD"),
		os.Getenv("DB_NAME"),
		os.Getenv("DB_PORT"),
	)

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	// Default App ID from migration 20260105_add_multi_tenancy.sql
	appIDString := "00000000-0000-0000-0000-000000000001"
	appID, err := uuid.Parse(appIDString)
	if err != nil {
		log.Fatalf("Invalid default App UUID: %v", err)
	}

	// Verify App Exists
	var app models.Application
	if err := db.First(&app, "id = ?", appID).Error; err != nil {
		log.Println("Default app not found, creating default tenant and app...")
		
		tenantID := uuid.MustParse(appIDString)
		tenant := models.Tenant{
			ID:   tenantID,
			Name: "Default Tenant",
		}
		if err := db.FirstOrCreate(&tenant, models.Tenant{ID: tenantID}).Error; err != nil {
			log.Fatalf("Failed to ensure tenant: %v", err)
		}

		app = models.Application{
			ID:          appID,
			TenantID:    tenantID,
			Name:        "Default App",
			Description: "Created by migration script",
		}
		if err := db.Create(&app).Error; err != nil {
			log.Fatalf("Failed to create default app: %v", err)
		}
	}
	log.Printf("Using App: %s (%s)", app.Name, app.ID)

	providers := []struct {
		Name        string
		EnvID       string
		EnvSecret   string
		EnvRedirect string
	}{
		{"google", "GOOGLE_CLIENT_ID", "GOOGLE_CLIENT_SECRET", "GOOGLE_REDIRECT_URL"},
		{"facebook", "FACEBOOK_CLIENT_ID", "FACEBOOK_CLIENT_SECRET", "FACEBOOK_REDIRECT_URL"},
		{"github", "GITHUB_CLIENT_ID", "GITHUB_CLIENT_SECRET", "GITHUB_REDIRECT_URL"},
	}

	for _, p := range providers {
		clientID := os.Getenv(p.EnvID)
		clientSecret := os.Getenv(p.EnvSecret)
		redirectURL := os.Getenv(p.EnvRedirect)

		if clientID == "" || clientSecret == "" {
			log.Printf("Skipping %s: missing ID or Secret in env", p.Name)
			continue
		}

		// Upsert logic
		var existing models.OAuthProviderConfig
		result := db.Where("app_id = ? AND provider = ?", appID, p.Name).First(&existing)
		
		if result.Error == nil {
			// Update
			existing.ClientID = clientID
			existing.ClientSecret = clientSecret
			existing.RedirectURL = redirectURL
			existing.IsEnabled = true
			if err := db.Save(&existing).Error; err != nil {
				log.Printf("Failed to update %s config: %v", p.Name, err)
			} else {
				log.Printf("Updated %s config", p.Name)
			}
		} else if result.Error == gorm.ErrRecordNotFound {
			// Create
			config := models.OAuthProviderConfig{
				AppID:        appID,
				Provider:     p.Name,
				ClientID:     clientID,
				ClientSecret: clientSecret,
				RedirectURL:  redirectURL,
				IsEnabled:    true,
			}
			if err := db.Create(&config).Error; err != nil {
				log.Printf("Failed to create %s config: %v", p.Name, err)
			} else {
				log.Printf("Created %s config", p.Name)
			}
		} else {
			log.Printf("Error checking %s config: %v", p.Name, result.Error)
		}
	}
	log.Println("Migration completed.")
}
