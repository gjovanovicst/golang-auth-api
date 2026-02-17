package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
)

// AdminAuthMiddleware validates the Admin API Key header
func AdminAuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		apiKey := c.GetHeader("X-Admin-API-Key")
		requiredKey := viper.GetString("ADMIN_API_KEY")

		if requiredKey == "" {
			// If not configured, block all admin access for security
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "Admin API access not configured"})
			return
		}

		if apiKey == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "X-Admin-API-Key header is required"})
			return
		}

		if apiKey != requiredKey {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Invalid Admin API Key"})
			return
		}

		c.Next()
	}
}
