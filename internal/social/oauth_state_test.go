package social

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
)

const (
	testRedirectURI = "http://localhost:5173/auth/callback"
	testAppID       = "00000000-0000-0000-0000-000000000001"
	testVictimID    = "11111111-1111-1111-1111-111111111111"
	testAttackerID  = "22222222-2222-2222-2222-222222222222"
)

// setupOAuthStateTest configures viper and isolates the nonce store.
func setupOAuthStateTest(t *testing.T) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	viper.Reset()
	viper.Set("JWT_SECRET", strings.Repeat("s", 48))
	viper.Set("ALLOWED_REDIRECT_DOMAINS", []string{"localhost:5173"})
	viper.Set("DEFAULT_REDIRECT_URI", testRedirectURI)
	oauthStateNonces = newHybridNonceStore()
	t.Cleanup(func() {
		oauthStateNonces = newHybridNonceStore()
		viper.Reset()
	})
}

// signTestState signs an arbitrary state payload with the real signing key, so
// tests can exercise expiry/clock-skew paths that NewOAuthState never produces.
func signTestState(t *testing.T, state OAuthState) string {
	t.Helper()
	payload, err := json.Marshal(state)
	if err != nil {
		t.Fatalf("marshal state: %v", err)
	}
	encoded := base64.RawURLEncoding.EncodeToString(payload)
	sig, err := signOAuthStatePayload(encoded)
	if err != nil {
		t.Fatalf("sign state: %v", err)
	}
	return OAuthStateVersion + "." + encoded + "." + sig
}

func TestOAuthStateIsSignedAndParsed(t *testing.T) {
	setupOAuthStateTest(t)

	token, issued, err := NewOAuthState(testRedirectURI, testAppID, FlowLink, testVictimID)
	if err != nil {
		t.Fatalf("NewOAuthState: %v", err)
	}
	if !strings.HasPrefix(token, OAuthStateVersion+".") {
		t.Fatalf("state token is not versioned: %q", token)
	}

	parsed, err := ParseOAuthState(token)
	if err != nil {
		t.Fatalf("ParseOAuthState: %v", err)
	}
	if parsed.UserID != testVictimID || parsed.Flow != FlowLink || parsed.AppID != testAppID {
		t.Fatalf("unexpected parsed state: %+v", parsed)
	}
	if parsed.Nonce != issued.Nonce || parsed.Nonce == "" {
		t.Fatalf("nonce mismatch: %q vs %q", parsed.Nonce, issued.Nonce)
	}
}

// TestForgedUserIDRejected is the regression test for the reported account
// takeover: an attacker decodes a legitimate link state, swaps user_id for the
// victim's ID, re-encodes it and replays it with their own authorization code.
func TestForgedUserIDRejected(t *testing.T) {
	setupOAuthStateTest(t)

	token, _, err := NewOAuthState(testRedirectURI, testAppID, FlowLink, testAttackerID)
	if err != nil {
		t.Fatalf("NewOAuthState: %v", err)
	}

	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		t.Fatalf("expected 3 parts, got %d", len(parts))
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	var raw map[string]any
	if err := json.Unmarshal(payload, &raw); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	raw["user_id"] = testVictimID
	raw["app_id"] = testAppID
	tamperedJSON, err := json.Marshal(raw)
	if err != nil {
		t.Fatalf("marshal tampered payload: %v", err)
	}

	// Re-encode with the original signature — the classic forgery attempt.
	forged := parts[0] + "." + base64.RawURLEncoding.EncodeToString(tamperedJSON) + "." + parts[2]
	if _, err := ParseOAuthState(forged); err == nil {
		t.Fatal("forged state with a swapped user_id was accepted")
	}

	// Same payload, signature stripped.
	if _, err := ParseOAuthState(parts[0] + "." + base64.RawURLEncoding.EncodeToString(tamperedJSON)); err == nil {
		t.Fatal("unsigned forged state was accepted")
	}
}

// TestLegacyUnsignedStateRejected ensures pre-fix states (plain base64 JSON)
// cannot be used to bypass the signature check.
func TestLegacyUnsignedStateRejected(t *testing.T) {
	setupOAuthStateTest(t)

	legacy := OAuthState{
		RedirectURI: testRedirectURI,
		AppID:       testAppID,
		Nonce:       "abcdefghijklmnop",
		Timestamp:   time.Now().UTC(),
		UserID:      testVictimID,
		Flow:        FlowLink,
	}
	payload, err := json.Marshal(legacy)
	if err != nil {
		t.Fatalf("marshal legacy state: %v", err)
	}
	encoded := base64.URLEncoding.EncodeToString(payload)

	if _, err := ParseOAuthState(encoded); err == nil {
		t.Fatal("legacy unsigned state was accepted")
	}
}

// TestOAuthStateReplayRejected verifies that a state token can be redeemed once.
func TestOAuthStateReplayRejected(t *testing.T) {
	setupOAuthStateTest(t)

	_, state, err := NewOAuthState(testRedirectURI, testAppID, FlowLink, testVictimID)
	if err != nil {
		t.Fatalf("NewOAuthState: %v", err)
	}
	if err := RegisterOAuthState(state); err != nil {
		t.Fatalf("RegisterOAuthState: %v", err)
	}

	first, err := ConsumeOAuthStateNonce(state.Nonce)
	if err != nil || !first {
		t.Fatalf("first redemption: consumed=%v err=%v", first, err)
	}
	for i := 0; i < 2; i++ {
		if consumed, err := ConsumeOAuthStateNonce(state.Nonce); err != nil || consumed {
			t.Fatalf("replay %d accepted: consumed=%v err=%v", i+2, consumed, err)
		}
	}
}

// TestOAuthStateUnknownNonceRejected covers states that were never issued by
// this server.
func TestOAuthStateUnknownNonceRejected(t *testing.T) {
	setupOAuthStateTest(t)

	consumed, err := ConsumeOAuthStateNonce("never-issued-nonce")
	if err != nil {
		t.Fatalf("ConsumeOAuthStateNonce: %v", err)
	}
	if consumed {
		t.Fatal("unknown nonce was accepted")
	}
}

func TestOAuthStateExpiredRejected(t *testing.T) {
	setupOAuthStateTest(t)

	expired := signTestState(t, OAuthState{
		RedirectURI: testRedirectURI,
		AppID:       testAppID,
		Nonce:       "expired-nonce",
		Timestamp:   time.Now().UTC().Add(-OAuthStateTTL - time.Minute),
		Flow:        FlowLogin,
	})
	if _, err := ParseOAuthState(expired); err == nil {
		t.Fatal("expired state was accepted")
	}
}

func TestOAuthStateFutureTimestampRejected(t *testing.T) {
	setupOAuthStateTest(t)

	future := signTestState(t, OAuthState{
		RedirectURI: testRedirectURI,
		AppID:       testAppID,
		Nonce:       "future-nonce",
		Timestamp:   time.Now().UTC().Add(10 * time.Minute),
		Flow:        FlowLogin,
	})
	if _, err := ParseOAuthState(future); err == nil {
		t.Fatal("state dated in the future was accepted")
	}
}

func TestOAuthStateFlowConstraints(t *testing.T) {
	setupOAuthStateTest(t)

	if _, _, err := NewOAuthState(testRedirectURI, testAppID, FlowLogin, testVictimID); err == nil {
		t.Fatal("login flow accepted a user ID")
	}
	if _, _, err := NewOAuthState(testRedirectURI, testAppID, FlowLink, ""); err == nil {
		t.Fatal("link flow accepted an empty user ID")
	}
	if _, _, err := NewOAuthState(testRedirectURI, testAppID, "unknown", ""); err == nil {
		t.Fatal("unknown flow was accepted")
	}
	if _, _, err := NewOAuthState("http://evil.example/callback", testAppID, FlowLogin, ""); err == nil {
		t.Fatal("non allow-listed redirect URI was accepted")
	}
}

// TestOAuthStateCookieBinding covers the browser-binding step: a state minted
// for one browser must not be redeemable from another one.
func TestOAuthStateCookieBinding(t *testing.T) {
	setupOAuthStateTest(t)
	t.Setenv("OAUTH_STATE_COOKIE_BINDING", "true")

	h := &Handler{}

	initWriter := httptest.NewRecorder()
	initCtx, _ := gin.CreateTestContext(initWriter)
	token, err := h.beginOAuthState(initCtx, testRedirectURI, testAppID, FlowLink, testVictimID)
	if err != nil {
		t.Fatalf("beginOAuthState: %v", err)
	}

	state, err := ParseOAuthState(token)
	if err != nil {
		t.Fatalf("ParseOAuthState: %v", err)
	}

	var bindingCookie *http.Cookie
	for _, cookie := range initWriter.Result().Cookies() {
		if cookie.Name == oauthStateCookieName {
			bindingCookie = cookie
		}
	}
	if bindingCookie == nil {
		t.Fatal("binding cookie was not set on the initiation response")
	}
	if !bindingCookie.HttpOnly {
		t.Fatal("binding cookie must be HttpOnly")
	}

	// Attacker's browser: valid token, no binding cookie.
	attackerCtx, _ := gin.CreateTestContext(httptest.NewRecorder())
	attackerCtx.Request = httptest.NewRequest(http.MethodGet, "/auth/google/link/callback", nil)
	if _, err := h.completeOAuthState(attackerCtx, token); err == nil {
		t.Fatal("state was redeemed without the browser-binding cookie")
	}

	// Victim's browser: token + cookie -> accepted, and only once.
	victimReq := httptest.NewRequest(http.MethodGet, "/auth/google/link/callback", nil)
	victimReq.AddCookie(&http.Cookie{Name: oauthStateCookieName, Value: state.Nonce})
	victimCtx, _ := gin.CreateTestContext(httptest.NewRecorder())
	victimCtx.Request = victimReq
	if _, err := h.completeOAuthState(victimCtx, token); err != nil {
		t.Fatalf("legitimate callback rejected: %v", err)
	}

	replayReq := httptest.NewRequest(http.MethodGet, "/auth/google/link/callback", nil)
	replayReq.AddCookie(&http.Cookie{Name: oauthStateCookieName, Value: state.Nonce})
	replayCtx, _ := gin.CreateTestContext(httptest.NewRecorder())
	replayCtx.Request = replayReq
	if _, err := h.completeOAuthState(replayCtx, token); err == nil {
		t.Fatal("replayed state was accepted")
	}
}

// TestOAuthStateCookieBindingCanBeDisabled covers the documented escape hatch
// for non-browser clients: the signature and single-use guarantees still apply.
func TestOAuthStateCookieBindingCanBeDisabled(t *testing.T) {
	setupOAuthStateTest(t)
	t.Setenv("OAUTH_STATE_COOKIE_BINDING", "false")

	h := &Handler{}
	initCtx, _ := gin.CreateTestContext(httptest.NewRecorder())
	token, err := h.beginOAuthState(initCtx, testRedirectURI, testAppID, FlowLink, testVictimID)
	if err != nil {
		t.Fatalf("beginOAuthState: %v", err)
	}

	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	if _, err := h.completeOAuthState(ctx, token); err != nil {
		t.Fatalf("state rejected although binding is disabled: %v", err)
	}
}
