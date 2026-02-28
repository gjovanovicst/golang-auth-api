package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// jsonBody is a helper to extract the "error" field from a JSON response.
func jsonBody(w *httptest.ResponseRecorder) string {
	var body map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		return w.Body.String()
	}
	return body["error"]
}

// setupGuardRouter creates a Gin router with AppRouteGuardMiddleware and
// a test handler on the given route pattern.
func setupGuardRouter(pattern string) *gin.Engine {
	r := gin.New()
	r.GET(pattern, AppRouteGuardMiddleware(), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
	return r
}

func TestAppRouteGuard_MatchingIDs(t *testing.T) {
	appID := uuid.New()
	r := gin.New()
	r.GET("/app/:id/test", func(c *gin.Context) {
		// Simulate AppIDMiddleware setting the context
		c.Set(AppIDKey, appID)
		AppRouteGuardMiddleware()(c)
	}, func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/app/"+appID.String()+"/test", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200 for matching IDs, got %d: %s", w.Code, w.Body.String())
	}
}

func TestAppRouteGuard_MismatchedIDs(t *testing.T) {
	contextAppID := uuid.New()
	urlAppID := uuid.New()

	r := gin.New()
	r.GET("/app/:id/test", func(c *gin.Context) {
		c.Set(AppIDKey, contextAppID)
		AppRouteGuardMiddleware()(c)
	}, func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/app/"+urlAppID.String()+"/test", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("Expected 403 for mismatched IDs, got %d: %s", w.Code, w.Body.String())
	}
	errMsg := jsonBody(w)
	if errMsg != "X-App-ID header does not match the application in the URL" {
		t.Fatalf("Expected mismatch error, got: %s", errMsg)
	}
}

func TestAppRouteGuard_MissingContextAppID(t *testing.T) {
	r := gin.New()
	r.GET("/app/:id/test", func(c *gin.Context) {
		// Don't set app_id in context — simulate missing AppIDMiddleware
		AppRouteGuardMiddleware()(c)
	}, func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/app/"+uuid.New().String()+"/test", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("Expected 400 for missing context app_id, got %d: %s", w.Code, w.Body.String())
	}
}

func TestAppRouteGuard_InvalidContextAppID(t *testing.T) {
	r := gin.New()
	r.GET("/app/:id/test", func(c *gin.Context) {
		c.Set(AppIDKey, "not-a-uuid-object") // Wrong type — string instead of uuid.UUID
		AppRouteGuardMiddleware()(c)
	}, func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/app/"+uuid.New().String()+"/test", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("Expected 400 for invalid context app_id type, got %d: %s", w.Code, w.Body.String())
	}
}

func TestAppRouteGuard_InvalidURLAppID(t *testing.T) {
	appID := uuid.New()
	r := gin.New()
	r.GET("/app/:id/test", func(c *gin.Context) {
		c.Set(AppIDKey, appID)
		AppRouteGuardMiddleware()(c)
	}, func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/app/not-a-uuid/test", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("Expected 400 for invalid URL app_id, got %d: %s", w.Code, w.Body.String())
	}
	errMsg := jsonBody(w)
	if errMsg != "Invalid App ID in URL" {
		t.Fatalf("Expected 'Invalid App ID in URL', got: %s", errMsg)
	}
}

func TestAppRouteGuard_MissingURLParam(t *testing.T) {
	appID := uuid.New()
	r := gin.New()
	// Route without :id parameter
	r.GET("/app/test", func(c *gin.Context) {
		c.Set(AppIDKey, appID)
		AppRouteGuardMiddleware()(c)
	}, func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/app/test", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("Expected 400 for missing :id param, got %d: %s", w.Code, w.Body.String())
	}
}
