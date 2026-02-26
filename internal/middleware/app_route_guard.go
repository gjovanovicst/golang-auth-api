package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// AppRouteGuardMiddleware ensures that the X-App-ID header (stored in context by
// AppIDMiddleware) matches the :id URL path parameter. This prevents an authenticated
// app API key holder from accessing resources belonging to a different application
// by manipulating the URL while keeping their own X-App-ID header.
//
// Middleware chain order: AppIDMiddleware -> AppApiKeyMiddleware -> AppRouteGuardMiddleware
func AppRouteGuardMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Read app_id from context (set by AppIDMiddleware from X-App-ID header)
		appIDVal, exists := c.Get(AppIDKey)
		if !exists {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "X-App-ID header is required"})
			return
		}
		contextAppID, ok := appIDVal.(uuid.UUID)
		if !ok {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "Invalid App ID in context"})
			return
		}

		// Read :id from URL path parameter (used by admin handlers)
		urlIDStr := c.Param("id")
		if urlIDStr == "" {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "App ID is required in URL"})
			return
		}
		urlAppID, err := uuid.Parse(urlIDStr)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "Invalid App ID in URL"})
			return
		}

		// Ensure they match — prevents cross-app access
		if contextAppID != urlAppID {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "X-App-ID header does not match the application in the URL"})
			return
		}

		c.Next()
	}
}
