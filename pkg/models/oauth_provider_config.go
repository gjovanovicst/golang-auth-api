package models

import (
	"time"

	"github.com/google/uuid"
)

// OAuthProviderConfig stores OAuth credentials for a specific application and provider
type OAuthProviderConfig struct {
	ID           uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	AppID        uuid.UUID `gorm:"type:uuid;not null;index;uniqueIndex:idx_app_provider" json:"app_id"`
	Provider     string    `gorm:"not null;uniqueIndex:idx_app_provider" json:"provider"` // google, facebook, github
	ClientID     string    `gorm:"not null" json:"client_id"`
	ClientSecret string    `gorm:"not null" json:"-"` // Stored encrypted, not exposed via JSON
	RedirectURL  string    `gorm:"not null" json:"redirect_url"`
	IsEnabled    bool      `gorm:"default:true" json:"is_enabled"`
	CreatedAt    time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt    time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

// TableName overrides the default table name
func (OAuthProviderConfig) TableName() string {
	return "oauth_provider_configs"
}
