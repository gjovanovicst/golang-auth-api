package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

// ---------------------------------------------------------------------------
// Helper
// ---------------------------------------------------------------------------

// doSecurityRequest sends a GET request to the given path through a router
// with SecurityHeadersMiddleware installed and returns the response recorder.
func doSecurityRequest(path string) *httptest.ResponseRecorder {
	r := gin.New()
	r.Use(SecurityHeadersMiddleware())
	r.GET("/*any", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	r.ServeHTTP(w, req)
	return w
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

func TestSecurityHeadersCommon(t *testing.T) {
	w := doSecurityRequest("/api/test")

	// These headers should be present on every response.
	checks := map[string]string{
		"X-Frame-Options":        "DENY",
		"X-Content-Type-Options": "nosniff",
		"Referrer-Policy":        "strict-origin-when-cross-origin",
		"X-XSS-Protection":       "0",
	}

	for header, expected := range checks {
		got := w.Header().Get(header)
		if got != expected {
			t.Errorf("%s = %q, want %q", header, got, expected)
		}
	}
}

func TestSecurityHeadersPermissionsPolicy(t *testing.T) {
	w := doSecurityRequest("/api/test")

	pp := w.Header().Get("Permissions-Policy")
	for _, feature := range []string{"camera=()", "microphone=()", "geolocation=()", "payment=()"} {
		if !strings.Contains(pp, feature) {
			t.Errorf("Permissions-Policy missing %q, got %q", feature, pp)
		}
	}
}

func TestSecurityHeadersCSPAPI(t *testing.T) {
	w := doSecurityRequest("/api/v1/test")

	csp := w.Header().Get("Content-Security-Policy")
	if !strings.Contains(csp, "default-src 'none'") {
		t.Errorf("API CSP should contain default-src 'none', got %q", csp)
	}
	if strings.Contains(csp, "'unsafe-inline'") {
		t.Errorf("API CSP should NOT contain 'unsafe-inline', got %q", csp)
	}
	if !strings.Contains(csp, "frame-ancestors 'none'") {
		t.Errorf("API CSP should contain frame-ancestors 'none', got %q", csp)
	}
}

func TestSecurityHeadersCSPGUI(t *testing.T) {
	w := doSecurityRequest("/gui/dashboard")

	csp := w.Header().Get("Content-Security-Policy")
	// GUI should allow self and unsafe-inline for scripts and styles.
	if !strings.Contains(csp, "default-src 'self'") {
		t.Errorf("GUI CSP should contain default-src 'self', got %q", csp)
	}
	if !strings.Contains(csp, "script-src 'self' 'unsafe-inline'") {
		t.Errorf("GUI CSP should contain script-src 'self' 'unsafe-inline', got %q", csp)
	}
	if !strings.Contains(csp, "style-src 'self' 'unsafe-inline'") {
		t.Errorf("GUI CSP should contain style-src 'self' 'unsafe-inline', got %q", csp)
	}
	if !strings.Contains(csp, "font-src 'self'") {
		t.Errorf("GUI CSP should contain font-src 'self', got %q", csp)
	}
}

func TestSecurityHeadersHSTSWithTLS(t *testing.T) {
	r := gin.New()
	r.Use(SecurityHeadersMiddleware())
	r.GET("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	// Simulate TLS via X-Forwarded-Proto header (reverse proxy).
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-Forwarded-Proto", "https")
	r.ServeHTTP(w, req)

	hsts := w.Header().Get("Strict-Transport-Security")
	if hsts == "" {
		t.Error("expected HSTS header when X-Forwarded-Proto=https")
	}
	if !strings.Contains(hsts, "max-age=31536000") {
		t.Errorf("HSTS should have max-age=31536000, got %q", hsts)
	}
	if !strings.Contains(hsts, "includeSubDomains") {
		t.Errorf("HSTS should include includeSubDomains, got %q", hsts)
	}
}

func TestSecurityHeadersNoHSTSWithoutTLS(t *testing.T) {
	w := doSecurityRequest("/test")

	hsts := w.Header().Get("Strict-Transport-Security")
	if hsts != "" {
		t.Errorf("expected no HSTS header without TLS, got %q", hsts)
	}
}
