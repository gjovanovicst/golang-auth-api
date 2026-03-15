package models

import (
	"time"

	"github.com/google/uuid"
)

// OIDCAuthCode represents a single-use authorization code issued during the
// OAuth2 Authorization Code flow. The code is exchanged at the token endpoint
// for an access token + ID token within a short expiry window.
type OIDCAuthCode struct {
	ID    uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	AppID uuid.UUID `gorm:"type:uuid;not null;index" json:"app_id"`

	// ClientID of the OIDC client that initiated the authorization request
	ClientID string `gorm:"not null;index" json:"client_id"`

	// UserID of the end user who approved the authorization request
	UserID uuid.UUID `gorm:"type:uuid;not null" json:"user_id"`

	// Code is the random single-use authorization code sent to the redirect_uri
	Code string `gorm:"uniqueIndex;not null" json:"code"` // #nosec G101 -- random code, not a secret credential

	// RedirectURI must exactly match the redirect_uri used in the token request
	RedirectURI string `gorm:"not null" json:"redirect_uri"`

	// Scopes granted (space-separated, e.g. "openid profile email")
	Scopes string `gorm:"not null" json:"scopes"`

	// Nonce from the original authorization request — echoed into the ID token
	Nonce string `gorm:"default:''" json:"nonce"`

	// PKCE fields
	CodeChallenge       string `gorm:"default:''" json:"code_challenge"`
	CodeChallengeMethod string `gorm:"default:''" json:"code_challenge_method"` // "S256"

	// ExpiresAt: codes expire after a short window (default 10 minutes)
	ExpiresAt time.Time `gorm:"not null;index" json:"expires_at"`

	// Used: true after the code has been exchanged (prevents replay attacks)
	Used bool `gorm:"default:false" json:"used"`

	CreatedAt time.Time `gorm:"autoCreateTime" json:"created_at"`
}
