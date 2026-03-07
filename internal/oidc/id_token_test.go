package oidc

import (
	"strings"
	"testing"
	"time"

	"github.com/gjovanovicst/auth_api/pkg/models"
	"github.com/google/uuid"
)

// newTestUser creates a minimal models.User suitable for use in token tests.
func newTestUser() *models.User {
	return &models.User{
		ID:             uuid.MustParse("00000000-0000-0000-0000-000000000042"),
		Email:          "alice@example.com",
		EmailVerified:  true,
		Name:           "Alice Smith",
		FirstName:      "Alice",
		LastName:       "Smith",
		ProfilePicture: "https://example.com/alice.jpg",
		Locale:         "en-US",
	}
}

func newTestParams(key interface { /* *rsa.PrivateKey */
}) MintIDTokenParams {
	rsaKey, _ := GenerateRSAKey()
	if key != nil {
		// type assert only used internally; caller passes nil to use fresh key
	}
	user := newTestUser()
	return MintIDTokenParams{
		Issuer:   "https://auth.example.com/oidc/app-1",
		Audience: "my-client-id",
		User:     user,
		Roles:    []string{"admin", "editor"},
		Scopes:   []string{"openid", "profile", "email", "roles"},
		Nonce:    "test-nonce-xyz",
		TTL:      15 * time.Minute,
		Kid:      "app-key-1",
		Key:      rsaKey,
	}
}

// ─── MintIDToken ────────────────────────────────────────────────────────────

func TestMintIDToken_ReturnsToken(t *testing.T) {
	params := newTestParams(nil)
	token, err := MintIDToken(params)
	if err != nil {
		t.Fatalf("MintIDToken() error = %v", err)
	}
	if token == "" {
		t.Fatal("MintIDToken() returned empty string")
	}
	// A valid JWT has exactly 3 dot-separated segments.
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		t.Errorf("expected 3 JWT segments, got %d", len(parts))
	}
}

func TestMintIDToken_ContainsExpectedClaims(t *testing.T) {
	params := newTestParams(nil)
	tokenStr, err := MintIDToken(params)
	if err != nil {
		t.Fatalf("MintIDToken() error = %v", err)
	}

	claims, err := ParseIDToken(tokenStr, params.Key)
	if err != nil {
		t.Fatalf("ParseIDToken() error = %v", err)
	}

	if claims.Issuer != params.Issuer {
		t.Errorf("iss: got %q, want %q", claims.Issuer, params.Issuer)
	}
	if claims.Subject != params.User.ID.String() {
		t.Errorf("sub: got %q, want %q", claims.Subject, params.User.ID.String())
	}
	if len(claims.Audience) == 0 || claims.Audience[0] != params.Audience {
		t.Errorf("aud: got %v, want [%q]", claims.Audience, params.Audience)
	}
	if claims.Nonce != params.Nonce {
		t.Errorf("nonce: got %q, want %q", claims.Nonce, params.Nonce)
	}
}

func TestMintIDToken_ProfileScopeClaims(t *testing.T) {
	params := newTestParams(nil)
	tokenStr, _ := MintIDToken(params)
	claims, err := ParseIDToken(tokenStr, params.Key)
	if err != nil {
		t.Fatal(err)
	}

	user := params.User
	if claims.Name != user.Name {
		t.Errorf("name: got %q, want %q", claims.Name, user.Name)
	}
	if claims.GivenName != user.FirstName {
		t.Errorf("given_name: got %q, want %q", claims.GivenName, user.FirstName)
	}
	if claims.FamilyName != user.LastName {
		t.Errorf("family_name: got %q, want %q", claims.FamilyName, user.LastName)
	}
	if claims.Picture != user.ProfilePicture {
		t.Errorf("picture: got %q, want %q", claims.Picture, user.ProfilePicture)
	}
	if claims.Locale != user.Locale {
		t.Errorf("locale: got %q, want %q", claims.Locale, user.Locale)
	}
}

func TestMintIDToken_EmailScopeClaims(t *testing.T) {
	params := newTestParams(nil)
	tokenStr, _ := MintIDToken(params)
	claims, err := ParseIDToken(tokenStr, params.Key)
	if err != nil {
		t.Fatal(err)
	}

	if claims.Email != params.User.Email {
		t.Errorf("email: got %q, want %q", claims.Email, params.User.Email)
	}
	if !claims.EmailVerified {
		t.Error("email_verified should be true")
	}
}

func TestMintIDToken_RolesScopeClaims(t *testing.T) {
	params := newTestParams(nil)
	tokenStr, _ := MintIDToken(params)
	claims, err := ParseIDToken(tokenStr, params.Key)
	if err != nil {
		t.Fatal(err)
	}

	if len(claims.Roles) != len(params.Roles) {
		t.Errorf("roles: got %v, want %v", claims.Roles, params.Roles)
	}
}

func TestMintIDToken_OpenIDOnlyScope(t *testing.T) {
	params := newTestParams(nil)
	params.Scopes = []string{"openid"} // no profile, email, roles
	tokenStr, _ := MintIDToken(params)
	claims, err := ParseIDToken(tokenStr, params.Key)
	if err != nil {
		t.Fatal(err)
	}

	if claims.Name != "" {
		t.Errorf("profile scope omitted: name should be empty, got %q", claims.Name)
	}
	if claims.Email != "" {
		t.Errorf("email scope omitted: email should be empty, got %q", claims.Email)
	}
	if len(claims.Roles) != 0 {
		t.Errorf("roles scope omitted: roles should be nil, got %v", claims.Roles)
	}
	// sub must still be set
	if claims.Subject == "" {
		t.Error("sub claim must always be present")
	}
}

func TestMintIDToken_NoNonce(t *testing.T) {
	params := newTestParams(nil)
	params.Nonce = ""
	tokenStr, err := MintIDToken(params)
	if err != nil {
		t.Fatalf("MintIDToken() error = %v", err)
	}
	claims, err := ParseIDToken(tokenStr, params.Key)
	if err != nil {
		t.Fatal(err)
	}
	if claims.Nonce != "" {
		t.Errorf("nonce should be empty, got %q", claims.Nonce)
	}
}

func TestMintIDToken_ExpirationRespected(t *testing.T) {
	params := newTestParams(nil)
	params.TTL = 5 * time.Minute
	before := time.Now().UTC()
	tokenStr, _ := MintIDToken(params)
	after := time.Now().UTC()

	claims, err := ParseIDToken(tokenStr, params.Key)
	if err != nil {
		t.Fatal(err)
	}

	exp := claims.ExpiresAt.Time
	if exp.Before(before.Add(4*time.Minute)) || exp.After(after.Add(6*time.Minute)) {
		t.Errorf("expiration %v not within expected range [%v, %v]", exp,
			before.Add(4*time.Minute), after.Add(6*time.Minute))
	}
}

func TestMintIDToken_KidInHeader(t *testing.T) {
	params := newTestParams(nil)
	params.Kid = "my-app-uuid"
	tokenStr, err := MintIDToken(params)
	if err != nil {
		t.Fatal(err)
	}
	// Verify the token has 3 segments.
	parts := strings.Split(tokenStr, ".")
	if len(parts) != 3 {
		t.Fatal("invalid JWT format")
	}
	// Indirect check: ParseIDToken must succeed (kid is embedded in the header).
	_, err = ParseIDToken(tokenStr, params.Key)
	if err != nil {
		t.Fatalf("ParseIDToken() after setting kid: %v", err)
	}
}

// ─── ParseIDToken ────────────────────────────────────────────────────────────

func TestParseIDToken_InvalidSignature(t *testing.T) {
	params := newTestParams(nil)

	// Sign with a different key and try to verify with the original — should fail.
	wrongKey, _ := GenerateRSAKey()
	params2 := params
	params2.Key = wrongKey
	tokenWithWrongKey, _ := MintIDToken(params2)

	_, err := ParseIDToken(tokenWithWrongKey, params.Key)
	if err == nil {
		t.Fatal("expected error when verifying token signed by wrong key, got nil")
	}
}

func TestParseIDToken_TamperedToken(t *testing.T) {
	params := newTestParams(nil)
	tokenStr, _ := MintIDToken(params)

	// Tamper with the payload segment.
	parts := strings.Split(tokenStr, ".")
	if len(parts) != 3 {
		t.Fatal("invalid JWT format")
	}
	parts[1] = parts[1] + "X" // corrupt payload
	tampered := strings.Join(parts, ".")

	_, err := ParseIDToken(tampered, params.Key)
	if err == nil {
		t.Fatal("expected error for tampered token, got nil")
	}
}

func TestParseIDToken_ExpiredToken(t *testing.T) {
	params := newTestParams(nil)
	params.TTL = -1 * time.Minute // already expired
	tokenStr, err := MintIDToken(params)
	if err != nil {
		t.Fatal(err)
	}

	_, err = ParseIDToken(tokenStr, params.Key)
	if err == nil {
		t.Fatal("expected error for expired token, got nil")
	}
}

func TestParseIDToken_EmptyString(t *testing.T) {
	key, _ := GenerateRSAKey()
	_, err := ParseIDToken("", key)
	if err == nil {
		t.Fatal("expected error for empty token string, got nil")
	}
}

func TestParseIDToken_NotJWT(t *testing.T) {
	key, _ := GenerateRSAKey()
	_, err := ParseIDToken("not.a.jwt.token.here", key)
	if err == nil {
		t.Fatal("expected error for non-JWT input, got nil")
	}
}

// ─── Round-trip ─────────────────────────────────────────────────────────────

func TestMintAndParse_RoundTrip(t *testing.T) {
	key, _ := GenerateRSAKey()
	user := newTestUser()

	params := MintIDTokenParams{
		Issuer:   "https://issuer.example.com",
		Audience: "rp-client",
		User:     user,
		Roles:    []string{"viewer"},
		Scopes:   []string{"openid", "email", "profile", "roles"},
		Nonce:    "abc123",
		TTL:      30 * time.Minute,
		Kid:      "key-v1",
		Key:      key,
	}

	tokenStr, err := MintIDToken(params)
	if err != nil {
		t.Fatal(err)
	}

	claims, err := ParseIDToken(tokenStr, key)
	if err != nil {
		t.Fatal(err)
	}

	// Spot-check every claim.
	checks := map[string]struct{ got, want string }{
		"iss":     {claims.Issuer, params.Issuer},
		"sub":     {claims.Subject, user.ID.String()},
		"nonce":   {claims.Nonce, params.Nonce},
		"name":    {claims.Name, user.Name},
		"email":   {claims.Email, user.Email},
		"picture": {claims.Picture, user.ProfilePicture},
		"locale":  {claims.Locale, user.Locale},
	}
	for field, tc := range checks {
		if tc.got != tc.want {
			t.Errorf("%s: got %q, want %q", field, tc.got, tc.want)
		}
	}
	if len(claims.Roles) != 1 || claims.Roles[0] != "viewer" {
		t.Errorf("roles: got %v, want [viewer]", claims.Roles)
	}
}
