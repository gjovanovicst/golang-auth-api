package models

import (
	"time"

	"github.com/google/uuid"
)

// Application represents a specific app belonging to a tenant
type Application struct {
	ID                        uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	TenantID                  uuid.UUID `gorm:"type:uuid;not null;index" json:"tenant_id"`
	Name                      string    `gorm:"not null" json:"name"`
	Description               string    `json:"description"`
	TwoFAIssuerName           string    `gorm:"default:''" json:"two_fa_issuer_name"`                   // Custom name shown in authenticator apps (overrides app name)
	TwoFAEnabled              bool      `gorm:"default:true" json:"two_fa_enabled"`                     // Master switch: allow 2FA for this application
	TwoFARequired             bool      `gorm:"default:false" json:"two_fa_required"`                   // Force all users to set up 2FA
	Email2FAEnabled           bool      `gorm:"default:false" json:"email_2fa_enabled"`                 // Allow email-based 2FA for this application
	Passkey2FAEnabled         bool      `gorm:"default:false" json:"passkey_2fa_enabled"`               // Allow passkey as a 2FA method
	PasskeyLoginEnabled       bool      `gorm:"default:false" json:"passkey_login_enabled"`             // Allow fully passwordless login via passkey
	MagicLinkEnabled          bool      `gorm:"default:false" json:"magic_link_enabled"`                // Allow passwordless login via email magic link
	TwoFAMethods              string    `gorm:"type:varchar(100);default:'totp'" json:"two_fa_methods"` // Comma-separated available methods: "totp", "email", "passkey", or combinations
	LoginNotificationsEnabled bool      `gorm:"default:false" json:"login_notifications_enabled"`       // Send email notifications on new device/location logins
	SuspiciousActivityAlerts  bool      `gorm:"default:false" json:"suspicious_activity_alerts"`        // Send email alerts for suspicious activity (brute force, etc.)
	// SMS-based recovery — allows users to register a phone number for SMS 2FA / recovery codes
	SMS2FAEnabled bool `gorm:"default:false" json:"sms_2fa_enabled"` // Allow SMS-based recovery codes for this application
	// Trusted device management — allows users to skip 2FA for a configurable number of days
	TrustedDeviceEnabled bool `gorm:"default:false" json:"trusted_device_enabled"` // Allow users to mark devices as trusted (skips 2FA)
	TrustedDeviceMaxDays int  `gorm:"default:30" json:"trusted_device_max_days"`   // How many days a device is trusted (default 30)

	// Brute-Force Protection — per-app overrides (NULL = use global default from .env)
	BfLockoutEnabled   *bool   `gorm:"default:null" json:"bf_lockout_enabled,omitempty"`                     // Override account lockout master switch
	BfLockoutThreshold *int    `gorm:"default:null" json:"bf_lockout_threshold,omitempty"`                   // Override failed attempts before lockout
	BfLockoutDurations *string `gorm:"type:varchar(255);default:null" json:"bf_lockout_durations,omitempty"` // Override escalating durations (comma-separated, e.g. "15m,30m,1h")
	BfLockoutWindow    *string `gorm:"type:varchar(50);default:null" json:"bf_lockout_window,omitempty"`     // Override sliding window for counting failures (e.g. "15m")
	BfLockoutTierTTL   *string `gorm:"type:varchar(50);default:null" json:"bf_lockout_tier_ttl,omitempty"`   // Override tier escalation persistence (e.g. "24h")
	BfDelayEnabled     *bool   `gorm:"default:null" json:"bf_delay_enabled,omitempty"`                       // Override progressive delay master switch
	BfDelayStartAfter  *int    `gorm:"default:null" json:"bf_delay_start_after,omitempty"`                   // Override failures before delays begin
	BfDelayMaxSeconds  *int    `gorm:"default:null" json:"bf_delay_max_seconds,omitempty"`                   // Override maximum delay cap
	BfDelayTierTTL     *string `gorm:"type:varchar(50);default:null" json:"bf_delay_tier_ttl,omitempty"`     // Override delay tier persistence (e.g. "30m")
	BfCaptchaEnabled   *bool   `gorm:"default:null" json:"bf_captcha_enabled,omitempty"`                     // Override CAPTCHA master switch
	BfCaptchaSiteKey   *string `gorm:"type:varchar(500);default:null" json:"bf_captcha_site_key,omitempty"`  // Override reCAPTCHA site key
	BfCaptchaSecretKey *string `gorm:"type:varchar(500);default:null" json:"-"`                              // Override reCAPTCHA secret key (hidden from API responses)
	BfCaptchaThreshold *int    `gorm:"default:null" json:"bf_captcha_threshold,omitempty"`                   // Override failures before CAPTCHA required

	// Frontend URL — per-app override for the frontend URL used in emails and WebAuthn origins.
	// Falls back to the FRONTEND_URL environment variable when empty.
	FrontendURL string `gorm:"type:varchar(500);default:''" json:"frontend_url"`

	// OIDC Provider settings — allows this application to act as an OIDC issuer
	OIDCEnabled       bool   `gorm:"column:oidc_enabled;default:false" json:"oidc_enabled"`                      // Master switch: expose OIDC endpoints for this app
	OIDCRSAPrivateKey string `gorm:"column:oidc_rsa_private_key;type:text;default:''" json:"-"`                  // PEM-encoded RSA private key (generated on first use, never exposed)
	OIDCIDTokenTTL    int    `gorm:"column:oidc_id_token_ttl;default:3600" json:"oidc_id_token_ttl"`             // ID token lifetime in seconds (default 1h)
	OIDCIssuerURL     string `gorm:"column:oidc_issuer_url;type:varchar(500);default:''" json:"oidc_issuer_url"` // Optional custom issuer URL override (empty = auto-generated)

	CreatedAt            time.Time             `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt            time.Time             `gorm:"autoUpdateTime" json:"updated_at"`
	OAuthProviderConfigs []OAuthProviderConfig `gorm:"foreignKey:AppID" json:"oauth_provider_configs"`
	EmailServerConfig    *EmailServerConfig    `gorm:"foreignKey:AppID" json:"email_server_config,omitempty"`
	OIDCClients          []OIDCClient          `gorm:"foreignKey:AppID" json:"oidc_clients,omitempty"`
}
