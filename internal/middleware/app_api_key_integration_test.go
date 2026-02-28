//go:build integration

package middleware

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gjovanovicst/auth_api/internal/admin"
	"github.com/gjovanovicst/auth_api/pkg/models"
	"github.com/gjovanovicst/auth_api/web"
	"github.com/google/uuid"
)

// =============================================================================
// App API Key Integration Tests
// =============================================================================
//
// These tests exercise the full middleware chain for app API key authentication:
//   AppIDMiddleware -> AppApiKeyMiddleware -> AppRouteGuardMiddleware -> Handler
//
// They use a mock ApiKeyValidator (no database required).
//
// Run with:
//   go test -v -tags=integration ./internal/middleware/...
//
// =============================================================================

// --- Mock ApiKeyValidator ---------------------------------------------------

// mockKeyStore is a simple in-memory implementation of web.ApiKeyValidator
// for testing purposes.
type mockKeyStore struct {
	keys map[string]*models.ApiKey // keyHash -> ApiKey
}

func newMockKeyStore() *mockKeyStore {
	return &mockKeyStore{keys: make(map[string]*models.ApiKey)}
}

func (m *mockKeyStore) FindActiveKeyByHash(keyHash string) (*models.ApiKey, error) {
	key, exists := m.keys[keyHash]
	if !exists {
		return nil, nil
	}
	if key.IsRevoked {
		return nil, nil
	}
	if key.ExpiresAt != nil && key.ExpiresAt.Before(time.Now()) {
		return nil, nil
	}
	return key, nil
}

func (m *mockKeyStore) UpdateApiKeyLastUsed(id uuid.UUID) {
	// No-op for testing
}

// addKey creates and stores a mock API key, returning the raw key string.
func (m *mockKeyStore) addKey(keyType string, appID *uuid.UUID, revoked bool, expiresAt *time.Time) string {
	rawKey, keyHash, _, _ := generateTestKey(keyType)
	key := &models.ApiKey{
		ID:        uuid.New(),
		KeyType:   keyType,
		Name:      "test-key",
		KeyHash:   keyHash,
		KeyPrefix: rawKey[:12],
		KeySuffix: rawKey[len(rawKey)-4:],
		AppID:     appID,
		IsRevoked: revoked,
		ExpiresAt: expiresAt,
	}
	m.keys[keyHash] = key
	return rawKey
}

// generateTestKey creates a deterministic test key with the right prefix.
func generateTestKey(keyType string) (rawKey, keyHash, prefix, suffix string) {
	fakeRandom := uuid.New().String()[:24] // 24 chars of randomness
	p := "ak_"
	if keyType == admin.KeyTypeApp {
		p = "apk_"
	}
	rawKey = p + hex.EncodeToString([]byte(fakeRandom)[:24])
	h := sha256.Sum256([]byte(rawKey))
	keyHash = hex.EncodeToString(h[:])
	prefix = rawKey[:12]
	suffix = rawKey[len(rawKey)-4:]
	return
}

// --- Router setup -----------------------------------------------------------

// buildAppRouter creates a Gin router with the full /app/:id middleware chain
// and a test handler that returns 200 with auth_type from context.
func buildAppRouter(store *mockKeyStore) *gin.Engine {
	r := gin.New()

	// Global middleware (same as main.go)
	r.Use(AppIDMiddleware())

	// App API key route group (mirrors main.go)
	appRoutes := r.Group("/app/:id")
	appRoutes.Use(AppApiKeyMiddleware(store))
	appRoutes.Use(AppRouteGuardMiddleware())
	{
		appRoutes.GET("/email-config", func(c *gin.Context) {
			authType, _ := c.Get(web.AuthTypeKey)
			c.JSON(http.StatusOK, gin.H{
				"status":    "ok",
				"auth_type": authType,
				"app_id":    c.Param("id"),
			})
		})
		appRoutes.POST("/email-test", func(c *gin.Context) {
			authType, _ := c.Get(web.AuthTypeKey)
			c.JSON(http.StatusOK, gin.H{
				"status":    "ok",
				"auth_type": authType,
				"app_id":    c.Param("id"),
			})
		})
	}

	// Admin route group (for cross-testing)
	adminRoutes := r.Group("/admin")
	adminRoutes.Use(AdminAuthMiddleware(store))
	{
		adminRoutes.GET("/apps/:id/email-config", func(c *gin.Context) {
			authType, _ := c.Get(web.AuthTypeKey)
			c.JSON(http.StatusOK, gin.H{
				"status":    "ok",
				"auth_type": authType,
			})
		})
	}

	return r
}

// doAppRequest is a helper for making requests to /app routes.
func doAppRequest(r *gin.Engine, method, path, appID, apiKey string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	req := httptest.NewRequest(method, path, nil)
	if appID != "" {
		req.Header.Set("X-App-ID", appID)
	}
	if apiKey != "" {
		req.Header.Set("X-App-API-Key", apiKey)
	}
	r.ServeHTTP(w, req)
	return w
}

// doAdminRequest is a helper for making requests to /admin routes.
func doAdminRequest(r *gin.Engine, method, path, adminKey string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	req := httptest.NewRequest(method, path, nil)
	if adminKey != "" {
		req.Header.Set("X-Admin-API-Key", adminKey)
	}
	r.ServeHTTP(w, req)
	return w
}

// parseResponse extracts a JSON response body.
func parseResponse(w *httptest.ResponseRecorder) map[string]interface{} {
	var body map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &body)
	return body
}

// --- Tests ------------------------------------------------------------------

func TestIntegration_AppKey_ValidAccess(t *testing.T) {
	store := newMockKeyStore()
	appID := uuid.New()
	rawKey := store.addKey(admin.KeyTypeApp, &appID, false, nil)

	r := buildAppRouter(store)
	w := doAppRequest(r, http.MethodGet, "/app/"+appID.String()+"/email-config", appID.String(), rawKey)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d: %s", w.Code, w.Body.String())
	}

	body := parseResponse(w)
	if body["auth_type"] != web.AuthTypeApp {
		t.Fatalf("Expected auth_type=%q, got %v", web.AuthTypeApp, body["auth_type"])
	}
	if body["app_id"] != appID.String() {
		t.Fatalf("Expected app_id=%q, got %v", appID.String(), body["app_id"])
	}
}

func TestIntegration_AppKey_ValidAccess_POST(t *testing.T) {
	store := newMockKeyStore()
	appID := uuid.New()
	rawKey := store.addKey(admin.KeyTypeApp, &appID, false, nil)

	r := buildAppRouter(store)
	w := doAppRequest(r, http.MethodPost, "/app/"+appID.String()+"/email-test", appID.String(), rawKey)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestIntegration_AppKey_MissingAppIDHeader(t *testing.T) {
	store := newMockKeyStore()
	appID := uuid.New()
	rawKey := store.addKey(admin.KeyTypeApp, &appID, false, nil)

	r := buildAppRouter(store)
	w := doAppRequest(r, http.MethodGet, "/app/"+appID.String()+"/email-config", "", rawKey)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("Expected 400 for missing X-App-ID, got %d: %s", w.Code, w.Body.String())
	}
}

func TestIntegration_AppKey_MissingApiKeyHeader(t *testing.T) {
	store := newMockKeyStore()
	appID := uuid.New()

	r := buildAppRouter(store)
	w := doAppRequest(r, http.MethodGet, "/app/"+appID.String()+"/email-config", appID.String(), "")

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("Expected 401 for missing X-App-API-Key, got %d: %s", w.Code, w.Body.String())
	}
}

func TestIntegration_AppKey_InvalidApiKey(t *testing.T) {
	store := newMockKeyStore()
	appID := uuid.New()

	r := buildAppRouter(store)
	w := doAppRequest(r, http.MethodGet, "/app/"+appID.String()+"/email-config", appID.String(), "apk_totally_invalid_key")

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("Expected 401 for invalid key, got %d: %s", w.Code, w.Body.String())
	}
}

func TestIntegration_AppKey_WrongAppID_InURL(t *testing.T) {
	// Key is bound to appA, but URL points to appB.
	// X-App-ID matches the key (appA), so AppApiKeyMiddleware passes,
	// but AppRouteGuardMiddleware should reject because URL :id != X-App-ID.
	store := newMockKeyStore()
	appA := uuid.New()
	appB := uuid.New()
	rawKey := store.addKey(admin.KeyTypeApp, &appA, false, nil)

	r := buildAppRouter(store)
	w := doAppRequest(r, http.MethodGet, "/app/"+appB.String()+"/email-config", appA.String(), rawKey)

	if w.Code != http.StatusForbidden {
		t.Fatalf("Expected 403 for URL/header app ID mismatch, got %d: %s", w.Code, w.Body.String())
	}
	body := parseResponse(w)
	if body["error"] != "X-App-ID header does not match the application in the URL" {
		t.Fatalf("Expected mismatch error, got: %v", body["error"])
	}
}

func TestIntegration_AppKey_WrongAppID_InHeader(t *testing.T) {
	// Key is bound to appA. X-App-ID is appB (doesn't match key).
	// AppApiKeyMiddleware should reject because key's AppID != X-App-ID.
	store := newMockKeyStore()
	appA := uuid.New()
	appB := uuid.New()
	rawKey := store.addKey(admin.KeyTypeApp, &appA, false, nil)

	r := buildAppRouter(store)
	w := doAppRequest(r, http.MethodGet, "/app/"+appB.String()+"/email-config", appB.String(), rawKey)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("Expected 401 for key/app mismatch, got %d: %s", w.Code, w.Body.String())
	}
	body := parseResponse(w)
	if body["error"] != "API key does not match the application" {
		t.Fatalf("Expected app mismatch error, got: %v", body["error"])
	}
}

func TestIntegration_AppKey_RevokedKey(t *testing.T) {
	store := newMockKeyStore()
	appID := uuid.New()
	rawKey := store.addKey(admin.KeyTypeApp, &appID, true, nil) // revoked

	r := buildAppRouter(store)
	w := doAppRequest(r, http.MethodGet, "/app/"+appID.String()+"/email-config", appID.String(), rawKey)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("Expected 401 for revoked key, got %d: %s", w.Code, w.Body.String())
	}
}

func TestIntegration_AppKey_ExpiredKey(t *testing.T) {
	store := newMockKeyStore()
	appID := uuid.New()
	expired := time.Now().Add(-1 * time.Hour) // 1 hour ago
	rawKey := store.addKey(admin.KeyTypeApp, &appID, false, &expired)

	r := buildAppRouter(store)
	w := doAppRequest(r, http.MethodGet, "/app/"+appID.String()+"/email-config", appID.String(), rawKey)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("Expected 401 for expired key, got %d: %s", w.Code, w.Body.String())
	}
}

func TestIntegration_AdminKeyOnAppRoute(t *testing.T) {
	// An admin key should NOT work on /app routes (wrong key type).
	store := newMockKeyStore()
	appID := uuid.New()
	adminKey := store.addKey(admin.KeyTypeAdmin, nil, false, nil)

	r := buildAppRouter(store)
	w := doAppRequest(r, http.MethodGet, "/app/"+appID.String()+"/email-config", appID.String(), adminKey)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("Expected 401 for admin key on app route, got %d: %s", w.Code, w.Body.String())
	}
}

func TestIntegration_AppKeyOnAdminRoute(t *testing.T) {
	// An app key should NOT work on /admin routes (wrong key type).
	store := newMockKeyStore()
	appID := uuid.New()
	appKey := store.addKey(admin.KeyTypeApp, &appID, false, nil)

	r := buildAppRouter(store)
	w := doAdminRequest(r, http.MethodGet, "/admin/apps/"+appID.String()+"/email-config", appKey)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("Expected 401 for app key on admin route, got %d: %s", w.Code, w.Body.String())
	}
}

func TestIntegration_AdminKey_SetsAuthType(t *testing.T) {
	// Verify admin middleware sets auth_type = "admin"
	store := newMockKeyStore()
	appID := uuid.New()
	adminKey := store.addKey(admin.KeyTypeAdmin, nil, false, nil)

	r := buildAppRouter(store)
	w := doAdminRequest(r, http.MethodGet, "/admin/apps/"+appID.String()+"/email-config", adminKey)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d: %s", w.Code, w.Body.String())
	}

	body := parseResponse(w)
	if body["auth_type"] != web.AuthTypeAdmin {
		t.Fatalf("Expected auth_type=%q, got %v", web.AuthTypeAdmin, body["auth_type"])
	}
}

func TestIntegration_AppKey_SetsAuthType(t *testing.T) {
	// Verify app middleware sets auth_type = "app"
	store := newMockKeyStore()
	appID := uuid.New()
	rawKey := store.addKey(admin.KeyTypeApp, &appID, false, nil)

	r := buildAppRouter(store)
	w := doAppRequest(r, http.MethodGet, "/app/"+appID.String()+"/email-config", appID.String(), rawKey)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d: %s", w.Code, w.Body.String())
	}

	body := parseResponse(w)
	if body["auth_type"] != web.AuthTypeApp {
		t.Fatalf("Expected auth_type=%q, got %v", web.AuthTypeApp, body["auth_type"])
	}
}

func TestIntegration_AppKey_NoHeaders(t *testing.T) {
	// Completely empty request — no X-App-ID, no X-App-API-Key
	store := newMockKeyStore()
	r := buildAppRouter(store)

	w := doAppRequest(r, http.MethodGet, "/app/"+uuid.New().String()+"/email-config", "", "")

	if w.Code != http.StatusBadRequest {
		t.Fatalf("Expected 400 for missing all headers, got %d: %s", w.Code, w.Body.String())
	}
}

func TestIntegration_AppKey_InvalidAppIDFormat(t *testing.T) {
	store := newMockKeyStore()
	appID := uuid.New()
	rawKey := store.addKey(admin.KeyTypeApp, &appID, false, nil)

	r := buildAppRouter(store)
	w := doAppRequest(r, http.MethodGet, "/app/not-a-uuid/email-config", "not-a-uuid", rawKey)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("Expected 400 for invalid UUID format, got %d: %s", w.Code, w.Body.String())
	}
}

func TestIntegration_AppKey_AppKeyWithNilAppID(t *testing.T) {
	// Edge case: app-type key that somehow has no AppID bound.
	// AppApiKeyMiddleware should reject because foundKey.AppID == nil.
	store := newMockKeyStore()
	appID := uuid.New()
	rawKey := store.addKey(admin.KeyTypeApp, nil, false, nil) // nil AppID

	r := buildAppRouter(store)
	w := doAppRequest(r, http.MethodGet, "/app/"+appID.String()+"/email-config", appID.String(), rawKey)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("Expected 401 for app key with nil AppID, got %d: %s", w.Code, w.Body.String())
	}
}
