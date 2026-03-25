package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/gjovanovicst/auth_api/web"
)

// newTestContext creates a minimal gin.Context backed by a test HTTP request.
func newTestContext() (*gin.Context, *httptest.ResponseRecorder) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)
	return c, w
}

// ---------------------------------------------------------------------------
// parseScopes unit tests
// ---------------------------------------------------------------------------

func TestParseScopes_Empty(t *testing.T) {
	result := parseScopes("")
	if len(result) != 0 {
		t.Fatalf("expected empty slice for empty input, got %v", result)
	}
}

func TestParseScopes_Blank(t *testing.T) {
	result := parseScopes("   ")
	if len(result) != 0 {
		t.Fatalf("expected empty slice for blank input, got %v", result)
	}
}

func TestParseScopes_Single(t *testing.T) {
	result := parseScopes("users:read")
	if len(result) != 1 || result[0] != "users:read" {
		t.Fatalf("unexpected result: %v", result)
	}
}

func TestParseScopes_Multiple(t *testing.T) {
	result := parseScopes("users:read,auth:*,billing:write")
	want := []string{"users:read", "auth:*", "billing:write"}
	if len(result) != len(want) {
		t.Fatalf("expected %v, got %v", want, result)
	}
	for i, v := range want {
		if result[i] != v {
			t.Fatalf("index %d: expected %q, got %q", i, v, result[i])
		}
	}
}

func TestParseScopes_TrimsWhitespace(t *testing.T) {
	result := parseScopes(" users:read , auth:* ")
	want := []string{"users:read", "auth:*"}
	if len(result) != len(want) {
		t.Fatalf("expected %v, got %v", want, result)
	}
	for i, v := range want {
		if result[i] != v {
			t.Fatalf("index %d: expected %q, got %q", i, v, result[i])
		}
	}
}

func TestParseScopes_GlobalWildcard(t *testing.T) {
	result := parseScopes("*")
	if len(result) != 1 || result[0] != "*" {
		t.Fatalf("expected [\"*\"], got %v", result)
	}
}

// ---------------------------------------------------------------------------
// HasScope unit tests
// ---------------------------------------------------------------------------

// TestHasScope_NoContextKey: context has no ApiKeyScopesKey set at all.
// This is the static-env-key path — must be fully permissive.
func TestHasScope_NoContextKey_IsPermissive(t *testing.T) {
	c, _ := newTestContext()
	// Do NOT set ApiKeyScopesKey — simulates static ADMIN_API_KEY auth.
	if !HasScope(c, "admin:delete:users") {
		t.Fatal("expected true when no scopes key is set (static env key path)")
	}
}

// TestHasScope_EmptyScopes: key was issued with no scopes.
// Security fix: must be DENIED, not permitted.
func TestHasScope_EmptyScopes_Denied(t *testing.T) {
	c, _ := newTestContext()
	c.Set(web.ApiKeyScopesKey, []string{})

	scopes := []string{
		"users:read",
		"admin:delete:users",
		"billing:delete",
		"*",
		"anything",
	}
	for _, s := range scopes {
		if HasScope(c, s) {
			t.Errorf("expected false for scope %q with empty granted list (CWE-269 fix)", s)
		}
	}
}

// TestHasScope_BadContextType: context value is not []string — deny defensively.
func TestHasScope_BadContextType_Denied(t *testing.T) {
	c, _ := newTestContext()
	c.Set(web.ApiKeyScopesKey, "users:read") // wrong type: string instead of []string

	if HasScope(c, "users:read") {
		t.Fatal("expected false when context value has wrong type")
	}
}

// TestHasScope_ExactMatch: scope matches exactly.
func TestHasScope_ExactMatch(t *testing.T) {
	c, _ := newTestContext()
	c.Set(web.ApiKeyScopesKey, []string{"users:read", "billing:write"})

	if !HasScope(c, "users:read") {
		t.Error("expected true for exact match 'users:read'")
	}
	if !HasScope(c, "billing:write") {
		t.Error("expected true for exact match 'billing:write'")
	}
}

// TestHasScope_ExactMatch_NotPresent: scope not in granted list.
func TestHasScope_ExactMatch_NotPresent(t *testing.T) {
	c, _ := newTestContext()
	c.Set(web.ApiKeyScopesKey, []string{"users:read"})

	if HasScope(c, "users:write") {
		t.Error("expected false for 'users:write' when only 'users:read' is granted")
	}
	if HasScope(c, "admin:delete:users") {
		t.Error("expected false for 'admin:delete:users' when not granted")
	}
}

// TestHasScope_GlobalWildcard: a single "*" scope grants everything.
func TestHasScope_GlobalWildcard_GrantsAll(t *testing.T) {
	c, _ := newTestContext()
	c.Set(web.ApiKeyScopesKey, []string{"*"})

	scopes := []string{"users:read", "admin:delete:users", "billing:delete", "anything:else"}
	for _, s := range scopes {
		if !HasScope(c, s) {
			t.Errorf("expected true for scope %q when granted=[\"*\"]", s)
		}
	}
}

// TestHasScope_ResourceWildcard: "resource:*" covers any "resource:<action>".
func TestHasScope_ResourceWildcard(t *testing.T) {
	c, _ := newTestContext()
	c.Set(web.ApiKeyScopesKey, []string{"users:*"})

	if !HasScope(c, "users:read") {
		t.Error("expected true for 'users:read' when granted 'users:*'")
	}
	if !HasScope(c, "users:write") {
		t.Error("expected true for 'users:write' when granted 'users:*'")
	}
	if !HasScope(c, "users:delete") {
		t.Error("expected true for 'users:delete' when granted 'users:*'")
	}
}

// TestHasScope_ResourceWildcard_DoesNotCrossResource: "users:*" must not match "billing:read".
func TestHasScope_ResourceWildcard_DoesNotCrossResource(t *testing.T) {
	c, _ := newTestContext()
	c.Set(web.ApiKeyScopesKey, []string{"users:*"})

	if HasScope(c, "billing:read") {
		t.Error("expected false for 'billing:read' when granted 'users:*'")
	}
	if HasScope(c, "admin:delete:users") {
		t.Error("expected false for 'admin:delete:users' when granted 'users:*'")
	}
}

// TestHasScope_ResourceWildcard_NoActionInRequired: "resource:*" wildcard requires
// the required scope to have a colon separator to match.
func TestHasScope_ResourceWildcard_RequiresTwoPartScope(t *testing.T) {
	c, _ := newTestContext()
	c.Set(web.ApiKeyScopesKey, []string{"users:*"})

	// "users" alone (no colon) should not match "users:*" via wildcard,
	// but it also isn't an exact match of "users:*", so expect false.
	if HasScope(c, "users") {
		t.Error("expected false for bare 'users' when granted 'users:*'")
	}
}

// TestHasScope_MultipleGranted_OneMatches: any match in the granted list is enough.
func TestHasScope_MultipleGranted_OneMatches(t *testing.T) {
	c, _ := newTestContext()
	c.Set(web.ApiKeyScopesKey, []string{"billing:read", "users:read", "auth:refresh"})

	if !HasScope(c, "users:read") {
		t.Error("expected true when 'users:read' is among multiple granted scopes")
	}
}

// TestHasScope_MultipleGranted_NoneMatch: no match in the granted list → false.
func TestHasScope_MultipleGranted_NoneMatch(t *testing.T) {
	c, _ := newTestContext()
	c.Set(web.ApiKeyScopesKey, []string{"billing:read", "users:read"})

	if HasScope(c, "admin:delete:users") {
		t.Error("expected false when required scope is absent from granted list")
	}
}
