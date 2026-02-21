package models

import (
	"time"

	"github.com/google/uuid"
)

// Application represents a specific app belonging to a tenant
type Application struct {
	ID                   uuid.UUID             `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	TenantID             uuid.UUID             `gorm:"type:uuid;not null;index" json:"tenant_id"`
	Name                 string                `gorm:"not null" json:"name"`
	Description          string                `json:"description"`
	CreatedAt            time.Time             `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt            time.Time             `gorm:"autoUpdateTime" json:"updated_at"`
	OAuthProviderConfigs []OAuthProviderConfig `gorm:"foreignKey:AppID" json:"oauth_provider_configs"`
}
