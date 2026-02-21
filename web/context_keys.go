package web

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/gjovanovicst/auth_api/pkg/models"
	"github.com/google/uuid"
)

// Context keys used by GUI middleware and handlers.
// Defined here (in the web package) to avoid import cycles between
// the middleware and admin packages.

const (
	// AdminSessionCookie is the name of the HTTP-only cookie for admin GUI sessions.
	AdminSessionCookie = "admin_session"

	// GUIAdminIDKey is the Gin context key for the authenticated admin's ID.
	GUIAdminIDKey = "admin_id"

	// GUIAdminUsernameKey is the Gin context key for the authenticated admin's username.
	GUIAdminUsernameKey = "admin_username"

	// GUISessionIDKey is the Gin context key for the current session ID.
	GUISessionIDKey = "admin_session_id"

	// CSRFTokenKey is the Gin context key where the CSRF token is stored for templates.
	CSRFTokenKey = "csrf_token"

	// RateLimitErrorKey is the Gin context key set by the rate limiter
	// when a request is rate-limited. The value is the error message string.
	RateLimitErrorKey = "rate_limit_error"
)

// SessionValidator is the interface used by GUI middleware to validate sessions
// and manage CSRF tokens. Implemented by admin.AccountService.
type SessionValidator interface {
	// ValidateSession checks if a session ID is valid and returns the associated admin account.
	ValidateSession(sessionID string) (*models.AdminAccount, error)

	// GenerateCSRFToken creates a CSRF token bound to the session.
	GenerateCSRFToken(sessionID string) (string, error)

	// ValidateCSRFToken checks if the provided token matches the stored one for the session.
	ValidateCSRFToken(sessionID, token string) bool
}

// SetSessionCookie sets the admin session cookie with security flags.
// Uses http.SetCookie directly to set SameSite=Strict (not supported by Gin's c.SetCookie).
func SetSessionCookie(c *gin.Context, sessionID string, maxAge int) {
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     AdminSessionCookie,
		Value:    sessionID,
		Path:     "/gui",
		MaxAge:   maxAge,
		Secure:   IsSecureCookie(c),
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
	})
}

// ClearSessionCookie removes the admin session cookie.
func ClearSessionCookie(c *gin.Context) {
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     AdminSessionCookie,
		Value:    "",
		Path:     "/gui",
		MaxAge:   -1,
		Secure:   IsSecureCookie(c),
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
	})
}

// IsSecureCookie returns true if the request is over HTTPS (Secure flag for cookies).
func IsSecureCookie(c *gin.Context) bool {
	return c.Request.TLS != nil || c.GetHeader("X-Forwarded-Proto") == "https"
}

// ClearRateLimitFallback is a hook set by the rate-limit middleware package.
// It clears the in-memory fallback counters for a given prefix + identifier.
// Callers (e.g. login handlers) should invoke this alongside the Redis clear
// so that both stores are reset on success.
//
// The function is nil until the middleware package's init() registers it.
var ClearRateLimitFallback func(keyPrefix, identifier string)

// ApiKeyValidator is the interface used by admin/app API key middleware to validate keys
// against hashed keys stored in the database. Implemented by admin.Repository.
type ApiKeyValidator interface {
	// FindActiveKeyByHash looks up an active (non-revoked, non-expired) API key by its SHA-256 hash.
	// Returns nil, nil if no matching key is found.
	FindActiveKeyByHash(keyHash string) (*models.ApiKey, error)

	// UpdateApiKeyLastUsed sets the last_used_at timestamp to now (fire-and-forget).
	UpdateApiKeyLastUsed(id uuid.UUID)
}
