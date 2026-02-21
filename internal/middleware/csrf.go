package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/gjovanovicst/auth_api/web"
)

// CSRFMiddleware provides CSRF protection for the Admin GUI.
//
// On safe methods (GET, HEAD, OPTIONS): generates a CSRF token and makes it
// available in the Gin context for template rendering.
//
// On state-changing methods (POST, PUT, DELETE): validates the CSRF token from
// the X-CSRF-Token header (HTMX) or the _csrf form field.
func CSRFMiddleware(sessionValidator web.SessionValidator) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get session ID from context (set by GUIAuthMiddleware)
		sessionID, exists := c.Get(web.GUISessionIDKey)
		if !exists {
			// No session — skip CSRF (GUIAuthMiddleware will handle redirect)
			c.Next()
			return
		}

		sessionIDStr, ok := sessionID.(string)
		if !ok || sessionIDStr == "" {
			c.Next()
			return
		}

		method := c.Request.Method

		if method == http.MethodGet || method == http.MethodHead || method == http.MethodOptions {
			// Generate and inject CSRF token for templates
			token, err := sessionValidator.GenerateCSRFToken(sessionIDStr)
			if err != nil {
				// Non-fatal: log and continue without CSRF token
				c.Next()
				return
			}
			c.Set(web.CSRFTokenKey, token)
			c.Next()
			return
		}

		// State-changing request: validate CSRF token
		token := c.GetHeader("X-CSRF-Token")
		if token == "" {
			token = c.PostForm("_csrf")
		}

		if token == "" {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "CSRF token missing"})
			return
		}

		if !sessionValidator.ValidateCSRFToken(sessionIDStr, token) {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "CSRF token invalid"})
			return
		}

		// Token is valid — proceed
		c.Set(web.CSRFTokenKey, token)
		c.Next()
	}
}
