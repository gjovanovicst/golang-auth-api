package middleware

import (
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/gjovanovicst/auth_api/web"
)

// parseScopes splits a comma-separated scopes string into a trimmed []string.
// An empty or blank input returns an empty slice.
func parseScopes(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return []string{}
	}
	parts := strings.Split(raw, ",")
	scopes := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			scopes = append(scopes, p)
		}
	}
	return scopes
}

// HasScope checks whether the validated API key (stored in the Gin context) has
// the given required scope. Wildcard support: a granted scope of "resource:*"
// covers any "resource:<action>". A granted scope of "*" covers everything.
//
// Permissive cases (returns true without checking scopes):
//   - The ApiKeyScopesKey context key is absent entirely — this means the
//     request was authenticated via the static ADMIN_API_KEY env var, which
//     is always fully permissive.
//
// Deny cases (returns false immediately):
//   - The context value is present but the type assertion to []string fails
//     (defensive; should not happen in normal operation).
//   - The granted scopes slice is empty — a key issued with no scopes has no
//     permissions. Use scope "*" to grant unrestricted access to a DB-backed key.
func HasScope(c *gin.Context, required string) bool {
	val, exists := c.Get(web.ApiKeyScopesKey)
	if !exists {
		// No scopes key in context — authenticated via static env key (fully permissive).
		return true
	}

	granted, ok := val.([]string)
	if !ok {
		// Type assertion failed — deny as a safe default.
		return false
	}

	if len(granted) == 0 {
		// Key was issued with no scopes — deny by default (least privilege).
		// To grant unrestricted access, create the key with scope "*".
		return false
	}

	requiredParts := strings.SplitN(required, ":", 2)

	for _, g := range granted {
		if g == "*" {
			return true
		}
		if g == required {
			return true
		}
		// Wildcard: "resource:*" covers "resource:<anything>"
		if strings.HasSuffix(g, ":*") && len(requiredParts) == 2 {
			gResource := strings.TrimSuffix(g, ":*")
			if gResource == requiredParts[0] {
				return true
			}
		}
	}
	return false
}
