package middleware

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/gjovanovicst/auth_api/internal/admin"
	"github.com/gjovanovicst/auth_api/web"
	"github.com/google/uuid"
)

const (
	// HeaderAppAPIKey is the HTTP header for per-application API keys.
	HeaderAppAPIKey = "X-App-API-Key" // #nosec G101 -- This is a header name, not a credential.
)

// AppApiKeyMiddleware validates per-application API keys.
// It requires both X-App-ID (already set by AppIDMiddleware) and X-App-API-Key headers.
// The key is looked up by SHA-256 hash and must be a non-revoked, non-expired "app" type key
// bound to the same application ID from the X-App-ID header.
//
// This middleware is OPTIONAL — it can be applied to specific route groups
// that require app-level key authentication in addition to the existing X-App-ID header.
// If keyValidator is nil, all requests are rejected.
func AppApiKeyMiddleware(keyValidator web.ApiKeyValidator) gin.HandlerFunc {
	return func(c *gin.Context) {
		if keyValidator == nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "App API key validation not configured"})
			return
		}

		// Require the API key header
		apiKey := c.GetHeader(HeaderAppAPIKey)
		if apiKey == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "X-App-API-Key header is required"})
			return
		}

		// Require X-App-ID to be present (should be set by AppIDMiddleware)
		appIDVal, exists := c.Get(AppIDKey)
		if !exists {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "X-App-ID header is required"})
			return
		}
		appID, ok := appIDVal.(uuid.UUID)
		if !ok {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "Invalid App ID"})
			return
		}

		// Hash the provided key and look it up
		h := sha256.Sum256([]byte(apiKey))
		keyHash := hex.EncodeToString(h[:])

		foundKey, err := keyValidator.FindActiveKeyByHash(keyHash)
		if err != nil || foundKey == nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Invalid App API Key"})
			return
		}

		// Must be an app-type key
		if foundKey.KeyType != admin.KeyTypeApp {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Invalid App API Key"})
			return
		}

		// Must be bound to the same application
		if foundKey.AppID == nil || *foundKey.AppID != appID {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "API key does not match the application"})
			return
		}

		// Update last_used_at asynchronously
		go keyValidator.UpdateApiKeyLastUsed(foundKey.ID)

		c.Set(web.AuthTypeKey, web.AuthTypeApp)
		c.Next()
	}
}
