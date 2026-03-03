package webauthn

import (
	"strings"

	"github.com/gjovanovicst/auth_api/pkg/models"
	"github.com/go-webauthn/webauthn/protocol"
	gowebauthn "github.com/go-webauthn/webauthn/webauthn"
)

// WebAuthnUser implements the webauthn.User interface to bridge our User model
// with the go-webauthn library.
type WebAuthnUser struct {
	User        *models.User
	Credentials []models.WebAuthnCredential
}

// WebAuthnID returns a unique identifier for the user (used as user.id in WebAuthn).
// We use the UUID bytes directly.
func (u *WebAuthnUser) WebAuthnID() []byte {
	id := u.User.ID
	return id[:]
}

// WebAuthnName returns the user's email (used as user.name in WebAuthn).
func (u *WebAuthnUser) WebAuthnName() string {
	return u.User.Email
}

// WebAuthnDisplayName returns the user's display name.
func (u *WebAuthnUser) WebAuthnDisplayName() string {
	if u.User.Name != "" {
		return u.User.Name
	}
	if u.User.FirstName != "" || u.User.LastName != "" {
		return strings.TrimSpace(u.User.FirstName + " " + u.User.LastName)
	}
	return u.User.Email
}

// WebAuthnCredentials returns the user's registered WebAuthn credentials
// in the format expected by the go-webauthn library.
func (u *WebAuthnUser) WebAuthnCredentials() []gowebauthn.Credential {
	creds := make([]gowebauthn.Credential, len(u.Credentials))
	for i, c := range u.Credentials {
		creds[i] = gowebauthn.Credential{
			ID:              c.CredentialID,
			PublicKey:       c.PublicKey,
			AttestationType: c.AttestationType,
			Authenticator: gowebauthn.Authenticator{
				AAGUID:    c.AAGUID,
				SignCount: c.SignCount,
			},
			Transport: parseTransports(c.Transports),
			Flags: gowebauthn.CredentialFlags{
				BackupEligible: c.BackupEligible,
				BackupState:    c.BackupState,
			},
		}
	}
	return creds
}

// WebAuthnIcon returns the user's profile picture URL (deprecated in WebAuthn spec but still in the interface).
func (u *WebAuthnUser) WebAuthnIcon() string {
	return u.User.ProfilePicture
}

// parseTransports converts a comma-separated transport string to AuthenticatorTransport slice.
func parseTransports(transports string) []protocol.AuthenticatorTransport {
	if transports == "" {
		return nil
	}
	parts := strings.Split(transports, ",")
	result := make([]protocol.AuthenticatorTransport, 0, len(parts))
	for _, t := range parts {
		t = strings.TrimSpace(t)
		if t != "" {
			result = append(result, protocol.AuthenticatorTransport(t))
		}
	}
	return result
}

// serializeTransports converts a slice of AuthenticatorTransport to a comma-separated string.
func serializeTransports(transports []protocol.AuthenticatorTransport) string {
	if len(transports) == 0 {
		return ""
	}
	parts := make([]string, len(transports))
	for i, t := range transports {
		parts[i] = string(t)
	}
	return strings.Join(parts, ",")
}

// ============================================================================
// Admin Account WebAuthn Adapter
// ============================================================================

// AdminWebAuthnUser implements the webauthn.User interface for admin accounts.
type AdminWebAuthnUser struct {
	Admin       *models.AdminAccount
	Credentials []models.WebAuthnCredential
}

// WebAuthnID returns a unique identifier for the admin (used as user.id in WebAuthn).
func (u *AdminWebAuthnUser) WebAuthnID() []byte {
	id := u.Admin.ID
	return id[:]
}

// WebAuthnName returns the admin's username (used as user.name in WebAuthn).
func (u *AdminWebAuthnUser) WebAuthnName() string {
	return u.Admin.Username
}

// WebAuthnDisplayName returns the admin's display name.
func (u *AdminWebAuthnUser) WebAuthnDisplayName() string {
	if u.Admin.Email != "" {
		return u.Admin.Email
	}
	return u.Admin.Username
}

// WebAuthnCredentials returns the admin's registered WebAuthn credentials.
func (u *AdminWebAuthnUser) WebAuthnCredentials() []gowebauthn.Credential {
	creds := make([]gowebauthn.Credential, len(u.Credentials))
	for i, c := range u.Credentials {
		creds[i] = gowebauthn.Credential{
			ID:              c.CredentialID,
			PublicKey:       c.PublicKey,
			AttestationType: c.AttestationType,
			Authenticator: gowebauthn.Authenticator{
				AAGUID:    c.AAGUID,
				SignCount: c.SignCount,
			},
			Transport: parseTransports(c.Transports),
			Flags: gowebauthn.CredentialFlags{
				BackupEligible: c.BackupEligible,
				BackupState:    c.BackupState,
			},
		}
	}
	return creds
}

// WebAuthnIcon returns the admin's icon URL (deprecated in spec but required by interface).
func (u *AdminWebAuthnUser) WebAuthnIcon() string {
	return ""
}
