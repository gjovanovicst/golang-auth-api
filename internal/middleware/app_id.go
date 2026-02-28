package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

const (
	AppIDKey     = "app_id"
	DefaultAppID = "00000000-0000-0000-0000-000000000001"
	HeaderAppID  = "X-App-ID"
)

func AppIDMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		path := c.Request.URL.Path

		// Skip validation for Swagger documentation, Admin API routes, and GUI routes
		if (len(path) >= 8 && path[:8] == "/swagger") ||
			(len(path) >= 6 && path[:6] == "/admin") ||
			(len(path) >= 4 && path[:4] == "/gui") {
			c.Next()
			return
		}

		appIDStr := c.GetHeader(HeaderAppID)

		// If header is missing, check query parameter
		if appIDStr == "" {
			appIDStr = c.Query("app_id")
		}

		// If still missing, check if it's a social callback
		// Social callbacks carry app_id in the 'state' parameter which is handled by the handler
		if appIDStr == "" && strings.HasPrefix(path, "/auth") && strings.Contains(path, "/callback") {
			c.Next()
			return
		}

		// If still missing, return error
		if appIDStr == "" {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "X-App-ID header is required"})
			return
		}

		// Validate UUID
		appID, err := uuid.Parse(appIDStr)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "Invalid App ID"})
			return
		}

		c.Set(AppIDKey, appID)
		c.Next()
	}
}
