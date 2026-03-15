package dto

import (
	"time"

	"github.com/google/uuid"
)

// CreateTenantRequest represents the payload for creating a new tenant
type CreateTenantRequest struct {
	Name string `json:"name" binding:"required"`
}

// TenantResponse represents the tenant data returned to clients
type TenantResponse struct {
	ID        uuid.UUID `json:"id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// CreateAppRequest represents the payload for creating a new application
type CreateAppRequest struct {
	TenantID         string `json:"tenant_id" binding:"required"`
	Name             string `json:"name" binding:"required"`
	Description      string `json:"description"`
	FrontendURL      string `json:"frontend_url"`
	MagicLinkEnabled bool   `json:"magic_link_enabled"`
	// Email Action Link Paths (optional; empty = use system defaults)
	ResetPasswordPath string `json:"reset_password_path"`
	MagicLinkPath     string `json:"magic_link_path"`
	VerifyEmailPath   string `json:"verify_email_path"`
}

// AppResponse represents the application data returned to clients
type AppResponse struct {
	ID          uuid.UUID `json:"id"`
	TenantID    uuid.UUID `json:"tenant_id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	FrontendURL string    `json:"frontend_url"`
	// Email Action Link Paths (empty = system defaults apply)
	ResetPasswordPath string    `json:"reset_password_path"`
	MagicLinkPath     string    `json:"magic_link_path"`
	VerifyEmailPath   string    `json:"verify_email_path"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

// UpsertOAuthConfigRequest represents the payload for setting OAuth credentials
type UpsertOAuthConfigRequest struct {
	Provider     string `json:"provider" binding:"required"` // e.g., "google", "github"
	ClientID     string `json:"client_id" binding:"required"`
	ClientSecret string `json:"client_secret" binding:"required"` // #nosec G101,G117 -- This is a DTO field, not a hardcoded credential
	RedirectURL  string `json:"redirect_url" binding:"required"`
}

// OAuthConfigResponse represents the OAuth config data returned (excluding secret)
type OAuthConfigResponse struct {
	ID          uuid.UUID `json:"id"`
	AppID       uuid.UUID `json:"app_id"`
	Provider    string    `json:"provider"`
	ClientID    string    `json:"client_id"`
	RedirectURL string    `json:"redirect_url"`
	IsEnabled   bool      `json:"is_enabled"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// AppLoginConfigResponse is the public response for GET /app-config/:app_id.
// It exposes only the information the login/register UI needs — no secrets.
type AppLoginConfigResponse struct {
	AppID                  string   `json:"app_id"`
	EnabledSocialProviders []string `json:"enabled_social_providers"` // e.g. ["google","github"]
	OIDCEnabled            bool     `json:"oidc_enabled"`
	HasOIDCClients         bool     `json:"has_oidc_clients"`
	MagicLinkEnabled       bool     `json:"magic_link_enabled"`
	PasskeyLoginEnabled    bool     `json:"passkey_login_enabled"`
	TwoFAEnabled           bool     `json:"two_fa_enabled"`         // whether 2FA is allowed for this app
	TwoFARequired          bool     `json:"two_fa_required"`        // whether every user must set up 2FA before accessing the app
	SMS2FAEnabled          bool     `json:"sms_2fa_enabled"`        // whether SMS is available as a 2FA method
	TrustedDeviceEnabled   bool     `json:"trusted_device_enabled"` // whether "remember this device" is available
	// Login Page Branding
	LoginLogoURL        string `json:"login_logo_url,omitempty"`        // URL to the app logo shown on login pages
	LoginPrimaryColor   string `json:"login_primary_color,omitempty"`   // Primary brand color (e.g. "#4f46e5")
	LoginSecondaryColor string `json:"login_secondary_color,omitempty"` // Secondary brand color
	LoginDisplayName    string `json:"login_display_name,omitempty"`    // Display name shown on login page
	// OIDCClientLoginTheme is the login_theme of the first active OIDC client for this app.
	// "app" means the client follows the app's own theme (frontend should send ?ui_theme=).
	// Empty string means no active OIDC client exists.
	OIDCClientLoginTheme string `json:"oidc_client_login_theme,omitempty"`
	// Password Policy — exposed so the frontend can show real-time requirements before submission
	PwMinLength     int  `json:"pw_min_length"`     // Minimum password length (default 8)
	PwMaxLength     int  `json:"pw_max_length"`     // Maximum password length (default 128)
	PwRequireUpper  bool `json:"pw_require_upper"`  // Require at least one uppercase letter
	PwRequireLower  bool `json:"pw_require_lower"`  // Require at least one lowercase letter
	PwRequireDigit  bool `json:"pw_require_digit"`  // Require at least one digit
	PwRequireSymbol bool `json:"pw_require_symbol"` // Require at least one special character
}
