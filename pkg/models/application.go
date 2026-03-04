package models

import (
	"time"

	"github.com/google/uuid"
)

// Application represents a specific app belonging to a tenant
type Application struct {
	ID                        uuid.UUID             `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	TenantID                  uuid.UUID             `gorm:"type:uuid;not null;index" json:"tenant_id"`
	Name                      string                `gorm:"not null" json:"name"`
	Description               string                `json:"description"`
	TwoFAIssuerName           string                `gorm:"default:''" json:"two_fa_issuer_name"`                   // Custom name shown in authenticator apps (overrides app name)
	TwoFAEnabled              bool                  `gorm:"default:true" json:"two_fa_enabled"`                     // Master switch: allow 2FA for this application
	TwoFARequired             bool                  `gorm:"default:false" json:"two_fa_required"`                   // Force all users to set up 2FA
	Email2FAEnabled           bool                  `gorm:"default:false" json:"email_2fa_enabled"`                 // Allow email-based 2FA for this application
	Passkey2FAEnabled         bool                  `gorm:"default:false" json:"passkey_2fa_enabled"`               // Allow passkey as a 2FA method
	PasskeyLoginEnabled       bool                  `gorm:"default:false" json:"passkey_login_enabled"`             // Allow fully passwordless login via passkey
	MagicLinkEnabled          bool                  `gorm:"default:false" json:"magic_link_enabled"`                // Allow passwordless login via email magic link
	TwoFAMethods              string                `gorm:"type:varchar(100);default:'totp'" json:"two_fa_methods"` // Comma-separated available methods: "totp", "email", "passkey", or combinations
	LoginNotificationsEnabled bool                  `gorm:"default:false" json:"login_notifications_enabled"`       // Send email notifications on new device/location logins
	SuspiciousActivityAlerts  bool                  `gorm:"default:false" json:"suspicious_activity_alerts"`        // Send email alerts for suspicious activity (brute force, etc.)
	CreatedAt                 time.Time             `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt                 time.Time             `gorm:"autoUpdateTime" json:"updated_at"`
	OAuthProviderConfigs      []OAuthProviderConfig `gorm:"foreignKey:AppID" json:"oauth_provider_configs"`
	EmailServerConfig         *EmailServerConfig    `gorm:"foreignKey:AppID" json:"email_server_config,omitempty"`
}
