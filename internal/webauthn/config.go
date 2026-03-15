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
// Priority: WEBAUTHN_RP_ID env var → hostname from app FrontendURL → hostname from FRONTEND_URL.
func resolveRPID(app *models.Application) string {
	// Check environment variable first
	rpID := viper.GetString("WEBAUTHN_RP_ID")
	if rpID != "" {
		return rpID
	}

	// Try per-app FrontendURL
	if app != nil && app.FrontendURL != "" {
		if parsed, err := url.Parse(app.FrontendURL); err == nil && parsed.Hostname() != "" {
			return parsed.Hostname()
		}
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
// It starts with WEBAUTHN_RP_ORIGINS, then always appends the per-app FrontendURL
// (if set and not already present) so that apps running on different origins than
// the global default are accepted. Falls back to FRONTEND_URL if the list is empty.
func resolveRPOrigins(app *models.Application) []string {
	var result []string

	originsStr := viper.GetString("WEBAUTHN_RP_ORIGINS")
	if originsStr != "" {
		for _, o := range strings.Split(originsStr, ",") {
			o = strings.TrimSpace(o)
			if o != "" {
				result = append(result, o)
			}
		}
	}

	// Always append the per-app FrontendURL when it differs from the global list.
	// This ensures apps hosted on a different origin than WEBAUTHN_RP_ORIGINS
	// (e.g. a second tenant app on a different port) can still complete passkey ceremonies.
	if app != nil && app.FrontendURL != "" {
		appOrigin := strings.TrimRight(app.FrontendURL, "/")
		found := false
		for _, o := range result {
			if o == appOrigin {
				found = true
				break
			}
		}
		if !found {
			result = append(result, appOrigin)
		}
	}

	if len(result) == 0 {
		if frontendURL := viper.GetString("FRONTEND_URL"); frontendURL != "" {
			result = append(result, strings.TrimRight(frontendURL, "/"))
		}
	}

	return result
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

	// Origins for admin GUI: start with WEBAUTHN_RP_ORIGINS, then always append
	// ADMIN_URL so admin passkey ceremonies (registered/verified at the admin GUI
	// origin) are accepted even when WEBAUTHN_RP_ORIGINS is set for tenant apps.
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
	if adminURL := strings.TrimRight(viper.GetString("ADMIN_URL"), "/"); adminURL != "" {
		found := false
		for _, o := range rpOrigins {
			if o == adminURL {
				found = true
				break
			}
		}
		if !found {
			rpOrigins = append(rpOrigins, adminURL)
		}
	}
	if len(rpOrigins) == 0 {
		if u := viper.GetString("FRONTEND_URL"); u != "" {
			rpOrigins = append(rpOrigins, strings.TrimRight(u, "/"))
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

	// Origins for admin GUI: start with WEBAUTHN_RP_ORIGINS, then always append
	// ADMIN_URL so admin passkey ceremonies (registered/verified at the admin GUI
	// origin) are accepted even when WEBAUTHN_RP_ORIGINS is set for tenant apps.
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
	if adminURL := strings.TrimRight(viper.GetString("ADMIN_URL"), "/"); adminURL != "" {
		found := false
		for _, o := range rpOrigins {
			if o == adminURL {
				found = true
				break
			}
		}
		if !found {
			rpOrigins = append(rpOrigins, adminURL)
		}
	}
	if len(rpOrigins) == 0 {
		if u := viper.GetString("FRONTEND_URL"); u != "" {
			rpOrigins = append(rpOrigins, strings.TrimRight(u, "/"))
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
