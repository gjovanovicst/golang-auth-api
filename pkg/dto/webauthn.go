package dto

import "encoding/json"

// ============================================================================
// Passkey Registration DTOs
// ============================================================================

// PasskeyRegisterBeginResponse contains the PublicKeyCredentialCreationOptions
// to be passed to navigator.credentials.create() on the client.
type PasskeyRegisterBeginResponse struct {
	Options json.RawMessage `json:"options" swaggertype:"object"`
}

// PasskeyRegisterFinishRequest contains the client's attestation response
// and a user-friendly name for the new passkey.
type PasskeyRegisterFinishRequest struct {
	Name       string          `json:"name" validate:"required,min=1,max=100"`
	Credential json.RawMessage `json:"credential" validate:"required" swaggertype:"object"`
}

// ============================================================================
// Passkey 2FA (Assertion) DTOs
// ============================================================================

// Passkey2FABeginRequest initiates a passkey-based 2FA verification during login.
type Passkey2FABeginRequest struct {
	TempToken string `json:"temp_token" validate:"required"`
}

// Passkey2FABeginResponse contains the PublicKeyCredentialRequestOptions
// to be passed to navigator.credentials.get() on the client.
type Passkey2FABeginResponse struct {
	Options json.RawMessage `json:"options" swaggertype:"object"`
}

// Passkey2FAFinishRequest contains the client's assertion response for 2FA verification.
type Passkey2FAFinishRequest struct {
	TempToken  string          `json:"temp_token" validate:"required"`
	Credential json.RawMessage `json:"credential" validate:"required" swaggertype:"object"`
}

// ============================================================================
// Passwordless Login DTOs
// ============================================================================

// PasskeyLoginBeginResponse contains the PublicKeyCredentialRequestOptions
// for passwordless (discoverable credential) login.
type PasskeyLoginBeginResponse struct {
	Options   json.RawMessage `json:"options" swaggertype:"object"`
	SessionID string          `json:"session_id"`
}

// PasskeyLoginFinishRequest contains the client's assertion response for passwordless login.
type PasskeyLoginFinishRequest struct {
	SessionID  string          `json:"session_id" validate:"required"`
	Credential json.RawMessage `json:"credential" validate:"required" swaggertype:"object"`
}

// ============================================================================
// Passkey Management DTOs
// ============================================================================

// PasskeyResponse represents a single passkey in API responses.
type PasskeyResponse struct {
	ID             string   `json:"id"`
	Name           string   `json:"name"`
	CreatedAt      string   `json:"created_at"`
	LastUsedAt     *string  `json:"last_used_at,omitempty"`
	BackupEligible bool     `json:"backup_eligible"`
	BackupState    bool     `json:"backup_state"`
	Transports     []string `json:"transports"`
}

// PasskeyListResponse contains a list of passkeys for a user.
type PasskeyListResponse struct {
	Passkeys []PasskeyResponse `json:"passkeys"`
}

// PasskeyRenameRequest represents a request to rename a passkey.
type PasskeyRenameRequest struct {
	Name string `json:"name" validate:"required,min=1,max=100"`
}
