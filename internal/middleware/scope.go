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
// If no scopes are set on the context (e.g. the request was authenticated via
// the static ADMIN_API_KEY env var), HasScope returns true so legacy keys
// remain fully permissive.
func HasScope(c *gin.Context, required string) bool {
	val, exists := c.Get(web.ApiKeyScopesKey)
	if !exists {
		// No scopes set — authenticated via static env key (fully permissive)
		return true
	}

	granted, ok := val.([]string)
	if !ok || len(granted) == 0 {
		// Empty scopes slice means the key was issued with no restrictions yet;
		// treat as fully permissive for backward compatibility.
		return true
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
