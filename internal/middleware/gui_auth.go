package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/gjovanovicst/auth_api/web"
)

// GUIAuthMiddleware validates admin sessions via HTTP-only cookies.
// Unauthenticated requests are redirected to the login page.
func GUIAuthMiddleware(sessionValidator web.SessionValidator) gin.HandlerFunc {
	return func(c *gin.Context) {
		path := c.Request.URL.Path

		// Skip authentication for login page, 2FA verification, passkey login, and static assets
		if path == "/gui/login" ||
			path == "/gui/2fa-verify" ||
			path == "/gui/2fa-resend-email" ||
			path == "/gui/passkey-login/begin" ||
			path == "/gui/passkey-login/finish" ||
			strings.HasPrefix(path, "/gui/static/") {
			c.Next()
			return
		}

		// Read session cookie
		sessionID, err := c.Cookie(web.AdminSessionCookie)
		if err != nil || sessionID == "" {
			redirectToLogin(c)
			return
		}

		// Validate session
		account, err := sessionValidator.ValidateSession(sessionID)
		if err != nil {
			// Clear invalid cookie
			web.ClearSessionCookie(c)
			redirectToLogin(c)
			return
		}

		// Set admin context for downstream handlers
		c.Set(web.GUIAdminIDKey, account.ID.String())
		c.Set(web.GUIAdminUsernameKey, account.Username)
		c.Set(web.GUISessionIDKey, sessionID)

		c.Next()
	}
}

// redirectToLogin sends a 302 redirect to the login page, preserving the original URL.
// The ?error=session_revoked param causes the login page to show a neutral "session no
// longer valid" message regardless of whether the session expired naturally or was
// administratively revoked — the middleware cannot distinguish the two without an extra
// Redis lookup, so we use a single neutral message for both cases.
func redirectToLogin(c *gin.Context) {
	originalURL := c.Request.URL.Path
	if originalURL == "/gui/" || originalURL == "/gui" {
		c.Redirect(http.StatusFound, "/gui/login?error=session_revoked")
	} else {
		c.Redirect(http.StatusFound, "/gui/login?error=session_revoked&redirect="+originalURL)
	}
	c.Abort()
}
