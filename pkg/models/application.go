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
	TwoFAIssuerName      string                `gorm:"default:''" json:"two_fa_issuer_name"`                  // Custom name shown in authenticator apps (overrides app name)
	TwoFAEnabled         bool                  `gorm:"default:true" json:"two_fa_enabled"`                    // Master switch: allow 2FA for this application
	TwoFARequired        bool                  `gorm:"default:false" json:"two_fa_required"`                  // Force all users to set up 2FA
	Email2FAEnabled      bool                  `gorm:"default:false" json:"email_2fa_enabled"`                // Allow email-based 2FA for this application
	TwoFAMethods         string                `gorm:"type:varchar(50);default:'totp'" json:"two_fa_methods"` // Comma-separated available methods: "totp", "email", "totp,email"
	CreatedAt            time.Time             `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt            time.Time             `gorm:"autoUpdateTime" json:"updated_at"`
	OAuthProviderConfigs []OAuthProviderConfig `gorm:"foreignKey:AppID" json:"oauth_provider_configs"`
	EmailServerConfig    *EmailServerConfig    `gorm:"foreignKey:AppID" json:"email_server_config,omitempty"`
}
