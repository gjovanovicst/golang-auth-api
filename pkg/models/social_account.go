package models

import (
	"time"

	"github.com/google/uuid"
)

// SocialAccount stores information related to a user's social media logins
type SocialAccount struct {
	ID             uuid.UUID  `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	UserID         uuid.UUID  `gorm:"type:uuid;not null;index" json:"user_id"`
	Provider       string     `gorm:"not null;index;uniqueIndex:idx_provider_user_id" json:"provider"`
	ProviderUserID string     `gorm:"not null;uniqueIndex:idx_provider_user_id" json:"provider_user_id"` // Composite unique index with Provider
	AccessToken    string     `json:"-"` // Stored encrypted, not exposed via JSON
	RefreshToken   string     `json:"-"` // Stored encrypted, not exposed via JSON
	ExpiresAt      *time.Time `json:"expires_at"`
	CreatedAt      time.Time  `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt      time.Time  `gorm:"autoUpdateTime" json:"updated_at"`
}