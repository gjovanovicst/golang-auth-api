package models

import (
	"time"

	"github.com/google/uuid"
)

// OIDCClient represents a registered OIDC/OAuth2 relying party (client application)
// that delegates authentication to this OIDC provider.
// Each client is scoped to an Application (AppID).
type OIDCClient struct {
	ID    uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	AppID uuid.UUID `gorm:"type:uuid;not null;index" json:"app_id"`

	// Human-readable name shown on the consent screen
	Name        string `gorm:"not null" json:"name"`
	Description string `gorm:"default:''" json:"description"`

	// OIDC client credentials
	// ClientID is a public identifier — safe to expose to end users
	ClientID string `gorm:"uniqueIndex;not null" json:"client_id"`
	// ClientSecretHash is the bcrypt hash of the client secret — never exposed via JSON
	ClientSecretHash string `gorm:"not null" json:"-"`

	// JSON array of allowed redirect URIs
	// Example: '["https://app.example.com/callback"]'
	RedirectURIs string `gorm:"type:text;not null;default:'[]'" json:"redirect_uris"`

	// Comma-separated list of allowed grant types
	// Supported: "authorization_code", "client_credentials", "refresh_token"
	AllowedGrantTypes string `gorm:"type:varchar(200);default:'authorization_code,refresh_token'" json:"allowed_grant_types"`

	// Comma-separated list of allowed OIDC scopes
	// Supported: "openid", "profile", "email", "roles"
	AllowedScopes string `gorm:"type:varchar(200);default:'openid profile email'" json:"allowed_scopes"`

	// RequireConsent: if true, shows consent screen; if false, auto-approves all scopes
	RequireConsent bool `gorm:"default:true" json:"require_consent"`

	// IsConfidential: true = confidential client (has secret), false = public client (PKCE only)
	IsConfidential bool `gorm:"default:true" json:"is_confidential"`

	// PKCERequired: if true, PKCE code_challenge is mandatory (even for confidential clients)
	PKCERequired bool `gorm:"default:false" json:"pkce_required"`

	// LogoURL: optional URL to client logo shown on consent screen
	LogoURL string `gorm:"default:''" json:"logo_url"`

	// IsActive: soft-disable a client without deleting it
	IsActive bool `gorm:"default:true" json:"is_active"`

	CreatedAt time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}
