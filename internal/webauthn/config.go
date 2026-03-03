package webauthn

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/gjovanovicst/auth_api/pkg/models"
	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/spf13/viper"
	"gorm.io/gorm"
)

// GetWebAuthn creates a configured webauthn.WebAuthn instance for the given application.
// It resolves configuration from application settings and falls back to environment variables.
func GetWebAuthn(db *gorm.DB, app *models.Application) (*webauthn.WebAuthn, error) {
	rpID := resolveRPID(app)
	rpName := resolveRPName(app)
	rpOrigins := resolveRPOrigins(app)

	if rpID == "" {
		return nil, fmt.Errorf("WebAuthn RP ID is not configured. Set WEBAUTHN_RP_ID or FRONTEND_URL")
	}

	cfg := &webauthn.Config{
		RPID:                  rpID,
		RPDisplayName:         rpName,
		RPOrigins:             rpOrigins,
		AttestationPreference: protocol.PreferNoAttestation, // Most common for passkeys
		AuthenticatorSelection: protocol.AuthenticatorSelection{
			// For passkeys (discoverable credentials / resident keys)
			ResidentKey:             protocol.ResidentKeyRequirementPreferred,
			UserVerification:        protocol.VerificationPreferred,
			AuthenticatorAttachment: "",
		},
	}

	return webauthn.New(cfg)
}

// GetWebAuthnForPasswordless creates a WebAuthn instance configured specifically
// for passwordless (discoverable credential) login, requiring user verification.
func GetWebAuthnForPasswordless(db *gorm.DB, app *models.Application) (*webauthn.WebAuthn, error) {
	rpID := resolveRPID(app)
	rpName := resolveRPName(app)
	rpOrigins := resolveRPOrigins(app)

	if rpID == "" {
		return nil, fmt.Errorf("WebAuthn RP ID is not configured. Set WEBAUTHN_RP_ID or FRONTEND_URL")
	}

	cfg := &webauthn.Config{
		RPID:                  rpID,
		RPDisplayName:         rpName,
		RPOrigins:             rpOrigins,
		AttestationPreference: protocol.PreferNoAttestation,
		AuthenticatorSelection: protocol.AuthenticatorSelection{
			ResidentKey:             protocol.ResidentKeyRequirementRequired, // Required for discoverable credentials
			UserVerification:        protocol.VerificationRequired,           // Must verify identity for passwordless
			AuthenticatorAttachment: "",
		},
	}

	return webauthn.New(cfg)
}

// resolveRPID determines the Relying Party ID for WebAuthn.
// Priority: WEBAUTHN_RP_ID env var → hostname from FRONTEND_URL.
func resolveRPID(app *models.Application) string {
	// Check environment variable first
	rpID := viper.GetString("WEBAUTHN_RP_ID")
	if rpID != "" {
		return rpID
	}

	// Fall back to hostname from FRONTEND_URL
	frontendURL := viper.GetString("FRONTEND_URL")
	if frontendURL != "" {
		if parsed, err := url.Parse(frontendURL); err == nil && parsed.Hostname() != "" {
			return parsed.Hostname()
		}
	}

	return ""
}

// resolveRPName determines the Relying Party display name.
// Priority: WEBAUTHN_RP_NAME → Application name → APP_NAME → "Auth API".
func resolveRPName(app *models.Application) string {
	rpName := viper.GetString("WEBAUTHN_RP_NAME")
	if rpName != "" {
		return rpName
	}

	if app != nil && app.Name != "" {
		return app.Name
	}

	appName := viper.GetString("APP_NAME")
	if appName != "" {
		return appName
	}

	return "Auth API"
}

// resolveRPOrigins determines the allowed origins for WebAuthn ceremonies.
// Priority: WEBAUTHN_RP_ORIGINS → FRONTEND_URL.
func resolveRPOrigins(app *models.Application) []string {
	originsStr := viper.GetString("WEBAUTHN_RP_ORIGINS")
	if originsStr != "" {
		origins := strings.Split(originsStr, ",")
		var result []string
		for _, o := range origins {
			o = strings.TrimSpace(o)
			if o != "" {
				result = append(result, o)
			}
		}
		if len(result) > 0 {
			return result
		}
	}

	frontendURL := viper.GetString("FRONTEND_URL")
	if frontendURL != "" {
		return []string{strings.TrimRight(frontendURL, "/")}
	}

	return []string{}
}

// GetWebAuthnForAdmin creates a WebAuthn instance for admin GUI passkey ceremonies.
// Admin passkeys are not tied to any application — configuration comes from environment variables.
func GetWebAuthnForAdmin() (*webauthn.WebAuthn, error) {
	rpID := viper.GetString("WEBAUTHN_RP_ID")
	if rpID == "" {
		// Fall back to hostname from ADMIN_URL or FRONTEND_URL
		for _, key := range []string{"ADMIN_URL", "FRONTEND_URL"} {
			u := viper.GetString(key)
			if u != "" {
				if parsed, err := url.Parse(u); err == nil && parsed.Hostname() != "" {
					rpID = parsed.Hostname()
					break
				}
			}
		}
	}
	if rpID == "" {
		return nil, fmt.Errorf("WebAuthn RP ID is not configured. Set WEBAUTHN_RP_ID, ADMIN_URL, or FRONTEND_URL")
	}

	rpName := viper.GetString("WEBAUTHN_RP_NAME")
	if rpName == "" {
		rpName = viper.GetString("APP_NAME")
	}
	if rpName == "" {
		rpName = "Auth API Admin"
	}

	// Origins for admin GUI
	var rpOrigins []string
	originsStr := viper.GetString("WEBAUTHN_RP_ORIGINS")
	if originsStr != "" {
		for _, o := range strings.Split(originsStr, ",") {
			o = strings.TrimSpace(o)
			if o != "" {
				rpOrigins = append(rpOrigins, o)
			}
		}
	}
	if len(rpOrigins) == 0 {
		for _, key := range []string{"ADMIN_URL", "FRONTEND_URL"} {
			u := viper.GetString(key)
			if u != "" {
				rpOrigins = append(rpOrigins, strings.TrimRight(u, "/"))
				break
			}
		}
	}

	cfg := &webauthn.Config{
		RPID:                  rpID,
		RPDisplayName:         rpName,
		RPOrigins:             rpOrigins,
		AttestationPreference: protocol.PreferNoAttestation,
		AuthenticatorSelection: protocol.AuthenticatorSelection{
			ResidentKey:             protocol.ResidentKeyRequirementRequired, // Discoverable credentials for passkey login
			UserVerification:        protocol.VerificationPreferred,
			AuthenticatorAttachment: "", // Use platform authenticators only (Windows Hello, Touch ID) to avoid cloud passkey provider issues
		},
	}

	return webauthn.New(cfg)
}

// GetWebAuthnForAdminLogin creates a WebAuthn instance for admin GUI passkey login ceremonies.
// Uses stricter settings: discoverable credentials required, user verification required.
func GetWebAuthnForAdminLogin() (*webauthn.WebAuthn, error) {
	rpID := viper.GetString("WEBAUTHN_RP_ID")
	if rpID == "" {
		for _, key := range []string{"ADMIN_URL", "FRONTEND_URL"} {
			u := viper.GetString(key)
			if u != "" {
				if parsed, err := url.Parse(u); err == nil && parsed.Hostname() != "" {
					rpID = parsed.Hostname()
					break
				}
			}
		}
	}
	if rpID == "" {
		return nil, fmt.Errorf("WebAuthn RP ID is not configured. Set WEBAUTHN_RP_ID, ADMIN_URL, or FRONTEND_URL")
	}

	rpName := viper.GetString("WEBAUTHN_RP_NAME")
	if rpName == "" {
		rpName = viper.GetString("APP_NAME")
	}
	if rpName == "" {
		rpName = "Auth API Admin"
	}

	var rpOrigins []string
	originsStr := viper.GetString("WEBAUTHN_RP_ORIGINS")
	if originsStr != "" {
		for _, o := range strings.Split(originsStr, ",") {
			o = strings.TrimSpace(o)
			if o != "" {
				rpOrigins = append(rpOrigins, o)
			}
		}
	}
	if len(rpOrigins) == 0 {
		for _, key := range []string{"ADMIN_URL", "FRONTEND_URL"} {
			u := viper.GetString(key)
			if u != "" {
				rpOrigins = append(rpOrigins, strings.TrimRight(u, "/"))
				break
			}
		}
	}

	cfg := &webauthn.Config{
		RPID:                  rpID,
		RPDisplayName:         rpName,
		RPOrigins:             rpOrigins,
		AttestationPreference: protocol.PreferNoAttestation,
		AuthenticatorSelection: protocol.AuthenticatorSelection{
			ResidentKey:             protocol.ResidentKeyRequirementRequired, // Discoverable credentials required for login
			UserVerification:        protocol.VerificationRequired,           // Must verify identity for passwordless login
			AuthenticatorAttachment: "",                                      // Use platform authenticators only, consistent with admin registration
		},
	}

	return webauthn.New(cfg)
}
