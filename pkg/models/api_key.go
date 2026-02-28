package models

import (
	"time"

	"github.com/google/uuid"
)

// ApiKey represents an API key for admin or per-application authentication.
// Admin keys authenticate to /admin/* JSON API routes.
// App keys authenticate to per-app routes alongside X-App-ID.
type ApiKey struct {
	ID          uuid.UUID    `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	KeyType     string       `gorm:"not null;index" json:"key_type"`                                            // "admin" or "app"
	Name        string       `gorm:"not null" json:"name"`                                                      // Human-readable label
	Description string       `json:"description"`                                                               // Optional purpose description
	KeyHash     string       `gorm:"not null;uniqueIndex" json:"-"`                                             // SHA-256 hash of the raw key
	KeyPrefix   string       `gorm:"not null" json:"key_prefix"`                                                // First 8 chars for display (e.g., "ak_a1b2c")
	KeySuffix   string       `gorm:"not null" json:"key_suffix"`                                                // Last 4 chars for identification
	AppID       *uuid.UUID   `gorm:"type:uuid;index" json:"app_id"`                                             // Required when key_type = "app"
	ExpiresAt   *time.Time   `gorm:"index" json:"expires_at"`                                                   // Optional expiration
	LastUsedAt  *time.Time   `json:"last_used_at"`                                                              // Updated on each use
	IsRevoked   bool         `gorm:"default:false;index" json:"is_revoked"`                                     // Revocation flag
	CreatedAt   time.Time    `json:"created_at"`                                                                // Auto-managed by GORM
	UpdatedAt   time.Time    `json:"updated_at"`                                                                // Auto-managed by GORM
	Application *Application `gorm:"foreignKey:AppID;constraint:OnDelete:CASCADE" json:"application,omitempty"` // Optional relation
}

// TableName specifies the table name for ApiKey.
func (ApiKey) TableName() string {
	return "api_keys"
}
