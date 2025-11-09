package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
)

// SocialAccount stores information related to a user's social media logins
type SocialAccount struct {
	ID             uuid.UUID      `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	UserID         uuid.UUID      `gorm:"type:uuid;not null;index" json:"user_id"`
	Provider       string         `gorm:"not null;index;uniqueIndex:idx_provider_user_id" json:"provider"`
	ProviderUserID string         `gorm:"not null;uniqueIndex:idx_provider_user_id" json:"provider_user_id"` // Composite unique index with Provider
	Email          string         `gorm:"" json:"email"`                                                     // Email from social provider
	Name           string         `gorm:"" json:"name"`                                                      // Name from social provider
	FirstName      string         `gorm:"" json:"first_name"`                                                // First name from social provider
	LastName       string         `gorm:"" json:"last_name"`                                                 // Last name from social provider
	ProfilePicture string         `gorm:"" json:"profile_picture"`                                           // Profile picture URL from social provider
	Username       string         `gorm:"" json:"username"`                                                  // Username/login from social provider (e.g., GitHub login)
	Locale         string         `gorm:"" json:"locale"`                                                    // Locale from social provider
	RawData        datatypes.JSON `gorm:"type:jsonb" json:"raw_data"`                                        // Complete raw JSON data from provider
	AccessToken    string         `json:"-"`                                                                 // Stored encrypted, not exposed via JSON
	RefreshToken   string         `json:"-"`                                                                 // Stored encrypted, not exposed via JSON
	ExpiresAt      *time.Time     `json:"expires_at"`
	CreatedAt      time.Time      `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt      time.Time      `gorm:"autoUpdateTime" json:"updated_at"`
}
