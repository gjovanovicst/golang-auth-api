package middleware

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/gjovanovicst/auth_api/internal/admin"
	"github.com/gjovanovicst/auth_api/web"
	"github.com/spf13/viper"
)

// AdminAuthMiddleware validates the Admin API Key header.
// It checks the static ADMIN_API_KEY env var first (fast path, backward compatible),
// then falls back to looking up hashed admin-type keys in the database.
// If keyValidator is nil, only the static env var is checked.
func AdminAuthMiddleware(keyValidator web.ApiKeyValidator) gin.HandlerFunc {
	return func(c *gin.Context) {
		apiKey := c.GetHeader("X-Admin-API-Key")

		if apiKey == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "X-Admin-API-Key header is required"})
			return
		}

		// Fast path: check static env var with timing-safe comparison
		requiredKey := viper.GetString("ADMIN_API_KEY")
		if requiredKey != "" {
			if subtle.ConstantTimeCompare([]byte(apiKey), []byte(requiredKey)) == 1 {
				c.Next()
				return
			}
		}

		// Fallback: check DB-backed admin keys by SHA-256 hash
		if keyValidator != nil {
			h := sha256.Sum256([]byte(apiKey))
			keyHash := hex.EncodeToString(h[:])

			foundKey, err := keyValidator.FindActiveKeyByHash(keyHash)
			if err == nil && foundKey != nil && foundKey.KeyType == admin.KeyTypeAdmin {
				// Update last_used_at asynchronously
				go keyValidator.UpdateApiKeyLastUsed(foundKey.ID)
				c.Next()
				return
			}
		}

		// If static key is not configured and no DB key matched, give a useful error
		if requiredKey == "" && keyValidator == nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "Admin API access not configured"})
			return
		}

		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Invalid Admin API Key"})
	}
}
