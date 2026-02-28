package middleware

import (
	"strings"

	"github.com/gin-gonic/gin"
)

// SecurityHeadersMiddleware adds security-related HTTP headers to every response.
//
// Headers set:
//   - X-Frame-Options: DENY                           — prevents clickjacking
//   - X-Content-Type-Options: nosniff                 — prevents MIME-type sniffing
//   - Referrer-Policy: strict-origin-when-cross-origin — limits referrer leakage
//   - X-XSS-Protection: 0                             — disable legacy XSS filter (CSP is the modern alternative)
//   - Permissions-Policy                               — restricts browser features
//   - Content-Security-Policy                          — restrictive CSP with exceptions for GUI assets
//   - Strict-Transport-Security                        — HSTS when served over TLS
func SecurityHeadersMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		h := c.Writer.Header()

		// Clickjacking protection
		h.Set("X-Frame-Options", "DENY")

		// MIME-type sniffing protection
		h.Set("X-Content-Type-Options", "nosniff")

		// Referrer policy — send origin on cross-origin, full URL on same-origin
		h.Set("Referrer-Policy", "strict-origin-when-cross-origin")

		// Disable legacy XSS auditor (modern browsers use CSP instead)
		h.Set("X-XSS-Protection", "0")

		// Permissions policy — disable unused browser features
		h.Set("Permissions-Policy", "camera=(), microphone=(), geolocation=(), payment=()")

		// Content-Security-Policy
		// The GUI uses embedded Bootstrap CSS/JS, HTMX, and Bootstrap Icons,
		// all served from /gui/static/*. We need 'unsafe-inline' for styles
		// because Bootstrap and HTMX use inline styles, and for scripts
		// because HTMX uses inline event handlers.
		path := c.Request.URL.Path
		if strings.HasPrefix(path, "/gui") {
			// GUI routes — allow self-hosted assets + inline styles/scripts for HTMX/Bootstrap
			h.Set("Content-Security-Policy", strings.Join([]string{
				"default-src 'self'",
				"script-src 'self' 'unsafe-inline'",
				"style-src 'self' 'unsafe-inline'",
				"font-src 'self'",
				"img-src 'self' data:",
				"connect-src 'self'",
				"frame-ancestors 'none'",
				"form-action 'self'",
				"base-uri 'self'",
			}, "; "))
		} else if strings.HasPrefix(path, "/swagger") {
			// Swagger UI — needs inline scripts/styles and to fetch its own JSON spec
			h.Set("Content-Security-Policy", strings.Join([]string{
				"default-src 'self'",
				"script-src 'self' 'unsafe-inline'",
				"style-src 'self' 'unsafe-inline'",
				"img-src 'self' data:",
				"font-src 'self' data:",
				"connect-src 'self'",
				"frame-ancestors 'none'",
				"base-uri 'self'",
			}, "; "))
		} else {
			// API routes — strict CSP (no inline, no styles needed)
			h.Set("Content-Security-Policy", strings.Join([]string{
				"default-src 'none'",
				"frame-ancestors 'none'",
				"base-uri 'self'",
			}, "; "))
		}

		// HSTS — only when serving over TLS (or behind a TLS-terminating proxy)
		if c.Request.TLS != nil || c.GetHeader("X-Forwarded-Proto") == "https" {
			// max-age=31536000 = 1 year; includeSubDomains for complete coverage
			h.Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		}

		c.Next()
	}
}
