package util

import (
	"strings"

	"github.com/spf13/viper"
)

// ResolveFrontendURL returns the effective base frontend URL for an application.
// Resolution priority: per-app FrontendURL → FRONTEND_URL env/config → http://localhost:8080
// The returned value always has any trailing slash stripped.
func ResolveFrontendURL(appFrontendURL string) string {
	if u := strings.TrimRight(appFrontendURL, "/"); u != "" {
		return u
	}
	if u := strings.TrimRight(viper.GetString("FRONTEND_URL"), "/"); u != "" {
		return u
	}
	return "http://localhost:8080"
}

// ResolveLinkPath returns the effective path suffix for an email action link.
// If appPath is non-empty it is used as-is; otherwise defaultPath is returned.
// The returned value always has a leading slash and no trailing slash.
func ResolveLinkPath(appPath, defaultPath string) string {
	p := strings.TrimRight(appPath, "/")
	if p == "" {
		p = strings.TrimRight(defaultPath, "/")
	}
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	return p
}

// ResolveAppName returns the effective application name for use in emails and UI.
// Resolution priority: APP_NAME env/config → "Auth API" default.
func ResolveAppName() string {
	if name := viper.GetString("APP_NAME"); name != "" {
		return name
	}
	return "Auth API"
}

// Default path constants used when per-app overrides are not configured.
const (
	DefaultResetPasswordPath = "/reset-password"
	DefaultMagicLinkPath     = "/magic-link"
	DefaultVerifyEmailPath   = "/verify-email"
)
